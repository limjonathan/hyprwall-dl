package system

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"hyprwall-dl/wallpaper"
	"golang.org/x/term"
)

var (
	thumbLock     sync.Mutex
	activeThumbID string
)

// RunTUI launches the interactive terminal selector with live previews and refresh support.
func RunTUI(results []wallpaper.ImageData, targetCount int, query, color, categories, purity string, width, height int, wallpapersDir string) []wallpaper.ImageData {
	if len(results) == 0 {
		return nil
	}

	// Filter and prioritize initial wallpapers to show exactly 10 candidates
	results = FilterAndPrioritizeWallpapers(results, wallpapersDir, 10)

	// 1. Enter terminal raw mode using the official golang.org/x/term package
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize raw terminal mode: %v\n", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Clean temporary thumbnail files on start
	CleanTempThumbnails()

	// Clear previous kitty preview icons on start/exit
	clearKittyPreview()

	// Order defers so that:
	// 1. cancel() runs first (stopping background downloads/writes/renders)
	// 2. clearKittyPreview() runs second (wiping graphics from the screen)
	// 3. term.Restore(...) runs last (restoring raw terminal mode)
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}
	defer clearKittyPreview()
	defer cancel()

	cursorIdx := 0
	selected := make(map[int]bool)
	
	// Default: pre-select the first item
	selected[0] = true

	// Use an asynchronous channel-based input reader to safely handle multi-byte
	// escape sequences (like Arrow Keys) without race conditions.
	inputChan := make(chan byte, 100)
	errChan := make(chan error, 1)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			b, err := reader.ReadByte()
			if err != nil {
				errChan <- err
				return
			}
			inputChan <- b
		}
	}()

	for {
		// Draw the complete TUI screen
		drawScreen(ctx, results, cursorIdx, selected, targetCount, wallpapersDir)

		var b byte
		select {
		case err := <-errChan:
			logDebug("Stdin error or EOF: %v", err)
			return nil
		case b = <-inputChan:
		}

		logDebug("Read byte: %d (%q)", b, string(b))

		// Distinguish Arrow Keys from Escape key
		var key byte
		if b == 27 { // Escape character
			logDebug("Escape sequence started...")
			// Wait up to 50ms for subsequent bytes of the escape sequence to arrive
			select {
			case b2 := <-inputChan:
				logDebug("Escape sequence second byte: %d (%q)", b2, string(b2))
				
				// Handle terminal query responses: APC ('_'), DCS ('P'), OSC (']')
				// These are sent by terminal in response to graphics commands and must be swallowed
				if b2 == 95 || b2 == 80 || b2 == 93 {
					logDebug("Terminal response sequence detected. Discarding...")
					discardedCount := 0
					for {
						select {
						case db := <-inputChan:
							discardedCount++
							if db == 7 { // BEL
								goto skipKey
							}
							if db == 27 { // Esc
								select {
								case db2 := <-inputChan:
									discardedCount++
									if db2 == 92 { // '\'
										goto skipKey
									}
								case <-time.After(50 * time.Millisecond):
									goto skipKey
								}
							}
						case <-time.After(50 * time.Millisecond):
							goto skipKey
						}
					}
				skipKey:
					logDebug("Terminal response sequence discarded completely (%d bytes).", discardedCount)
					continue // Skip redrawing and wait for actual keyboard input
				}

				if b2 == 91 || b2 == 79 { // '[' or 'O' (Normal or Application keypad mode)
					// We are inside a CSI/SS3 sequence. Read all parameters and the final character.
					var seq []byte
					timeout := false
					for {
						select {
						case db := <-inputChan:
							seq = append(seq, db)
							if db >= 64 && db <= 126 { // Final byte range for CSI/SS3
								goto parseCSI
							}
						case <-time.After(50 * time.Millisecond):
							timeout = true
							goto parseCSI
						}
					}
				parseCSI:
					if timeout {
						logDebug("Timeout waiting for CSI sequence to complete: %v", seq)
						key = 27 // Treat as Esc
					} else {
						logDebug("Full CSI sequence: %q (bytes: %v)", string(seq), seq)
						finalChar := seq[len(seq)-1]
						if finalChar == 'A' || finalChar == 'k' { // Arrow Up or 'k'
							key = 'k'
						} else if finalChar == 'B' || finalChar == 'j' { // Arrow Down or 'j'
							key = 'j'
						} else if finalChar == 'u' { // Kitty keyboard protocol extension
							// Parse event properties: codepoint, modifiers, event_type
							// E.g. "97;1:3" or "57373"
							s := string(seq[:len(seq)-1])
							s = strings.ReplaceAll(s, ":", ";")
							parts := strings.Split(s, ";")

							codepoint := 0
							eventType := 1 // Default to 1 (press) if not specified

							if len(parts) > 0 {
								fmt.Sscanf(parts[0], "%d", &codepoint)
							}
							if len(parts) > 2 {
								fmt.Sscanf(parts[2], "%d", &eventType)
							} else if len(parts) == 2 {
								// Check if original has colon separating eventType, e.g. codepoint:eventType
								if strings.Contains(string(seq), ":") {
									fmt.Sscanf(parts[1], "%d", &eventType)
								}
							}

							logDebug("Parsed CSI u: codepoint=%d, eventType=%d, parts=%v", codepoint, eventType, parts)

							if eventType == 3 {
								logDebug("Ignoring Kitty release event")
								continue // Skip redrawing and wait for actual keyboard input
							}

							if codepoint >= 0 && codepoint <= 255 {
								key = byte(codepoint)
							} else if codepoint == 57373 { // Kitty Arrow Up keycode
								key = 'k'
							} else if codepoint == 57374 { // Kitty Arrow Down keycode
								key = 'j'
							} else {
								logDebug("Ignoring other Kitty key code: %d", codepoint)
								continue // Ignore other key events to avoid double-processing or corruption
							}
						} else {
							logDebug("Ignoring unknown CSI sequence with final char: %q", string(finalChar))
							continue // Skip redrawing for ignored terminal events
						}
					}
				} else {
					key = 27
				}
			case <-time.After(50 * time.Millisecond):
				logDebug("Timeout waiting for escape sequence second byte (pure Escape)")
				// No extra bytes; user pressed pure Escape key to exit
				key = 27
			}
		} else {
			key = b
		}
		logDebug("Mapped key: %d (%q)", key, string(key))

		switch key {
		case 'k', 'K': // Scroll Up
			if cursorIdx > 0 {
				cursorIdx--
			} else {
				cursorIdx = len(results) - 1 // Wrap around
			}
		case 'j', 'J': // Scroll Down
			if cursorIdx < len(results)-1 {
				cursorIdx++
			} else {
				cursorIdx = 0 // Wrap around
			}
		case ' ': // Toggle Selection (Space)
			selected[cursorIdx] = !selected[cursorIdx]
		case 13, 10: // Confirm Selection (Enter)
			// Ensure we have at least one selection; if none, default to highlighted
			hasSelection := false
			for _, val := range selected {
				if val {
					hasSelection = true
					break
				}
			}
			if !hasSelection {
				selected[cursorIdx] = true
			}
			
			// Build and return the list of selected wallpapers
			var finalSelection []wallpaper.ImageData
			for i, res := range results {
				if selected[i] {
					finalSelection = append(finalSelection, res)
				}
			}
			return finalSelection
		case 'q', 'Q', 27, 3: // Quit / Escape / Ctrl+C
			// Restore terminal and exit with empty download list
			return nil
		case 'c', 'C': // Color Aesthetic Picker
			// Clear Kitty preview first so it doesn't get drawn over the menu
			clearKittyPreview()

			colorOptions := []struct {
				Name  string
				Hex   string
				Color string // ANSI escape code for visual coloring
			}{
				{"Neon Red", "ff0055", "\x1b[38;5;197m"},
				{"Neon Orange", "ff5500", "\x1b[38;5;202m"},
				{"Neon Yellow", "ffcc00", "\x1b[38;5;220m"},
				{"Neon Green", "00ff66", "\x1b[38;5;48m"},
				{"Neon Blue", "00ccff", "\x1b[38;5;45m"},
				{"Neon Purple", "9900ff", "\x1b[38;5;99m"},
				{"Neon Pink", "ff00aa", "\x1b[38;5;201m"},
				{"Cyan", "00ffff", "\x1b[38;5;51m"},
				{"Teal", "008080", "\x1b[38;5;30m"},
				{"White", "ffffff", "\x1b[38;5;231m"},
			}

			colorCursor := 0
			for {
				// Clear screen and draw the color picker menu
				fmt.Print("\x1b[H\x1b[2J")
				fmt.Print("\r\x1b[38;5;242m╔══════════════════════════════════════════════════════════════════════╗\x1b[0m\r\n")
				fmt.Print("\r\x1b[38;5;242m║\x1b[0m                 \x1b[1;38;5;51mSELECT AESTHETICS / COLOR PALETTE\x1b[0m                    \x1b[38;5;242m║\x1b[0m\r\n")
				fmt.Print("\r\x1b[38;5;242m╠══════════════════════════════════════════════════════════════════════╣\x1b[0m\r\n")
				fmt.Print("\r\x1b[38;5;242m║\x1b[0m \x1b[33m[Navigation]\x1b[0m  Up/Down or j/k    \x1b[38;5;242m│\x1b[0m  \x1b[33m[Confirm]\x1b[0m   Press Enter         \x1b[38;5;242m║\x1b[0m\r\n")
				fmt.Print("\r\x1b[38;5;242m║\x1b[0m \x1b[33m[Cancel]\x1b[0m      Press Esc or q                                    \x1b[38;5;242m║\x1b[0m\r\n")
				fmt.Print("\r\x1b[38;5;242m╚══════════════════════════════════════════════════════════════════════╝\x1b[0m\r\n\r\n")

				for idx, opt := range colorOptions {
					ptr := "   "
					if idx == colorCursor {
						ptr = "\x1b[1;38;5;201m▶▶ \x1b[0m"
					}
					// Draw color block
					fmt.Printf("\r%s%s█ %-15s \x1b[0m\x1b[38;5;242m(#%s)\x1b[0m\r\n", ptr, opt.Color, opt.Name, opt.Hex)
				}
				fmt.Print("\r\n\x1b[38;5;242m========================================================================\x1b[0m\r\n")

				// Read input
				var pickByte byte
				select {
				case err := <-errChan:
					logDebug("Stdin error in color picker: %v", err)
					return nil
				case pickByte = <-inputChan:
				}

				// Map Escape sequences in color picker
				var pickKey byte
				if pickByte == 27 {
					select {
					case b2 := <-inputChan:
						if b2 == 91 || b2 == 79 {
							var seq []byte
							timeout := false
							for {
								select {
								case db := <-inputChan:
									seq = append(seq, db)
									if db >= 64 && db <= 126 {
										goto parsePickerCSI
									}
								case <-time.After(50 * time.Millisecond):
									timeout = true
									goto parsePickerCSI
								}
							}
						parsePickerCSI:
							if !timeout {
								finalChar := seq[len(seq)-1]
								if finalChar == 'A' || finalChar == 'k' {
									pickKey = 'k'
								} else if finalChar == 'B' || finalChar == 'j' {
									pickKey = 'j'
								}
							} else {
								pickKey = 27
							}
						} else {
							pickKey = 27
						}
					case <-time.After(50 * time.Millisecond):
						pickKey = 27
					}
				} else {
					pickKey = pickByte
				}

				switch pickKey {
				case 'k', 'K':
					if colorCursor > 0 {
						colorCursor--
					} else {
						colorCursor = len(colorOptions) - 1
					}
				case 'j', 'J':
					if colorCursor < len(colorOptions)-1 {
						colorCursor++
					} else {
						colorCursor = 0
					}
				case 13, 10: // Enter
					// Set overridden color and trigger search/reload instantly!
					chosenColor := colorOptions[colorCursor].Hex
					color = chosenColor // Override color parameter

					// Re-trigger fresh search concurrently, showing the loading screen
					type refreshResult struct {
						results []wallpaper.ImageData
						err     error
					}
					resultChan := make(chan refreshResult, 1)

					go func() {
						newResults, err := wallpaper.SearchAllSources(query, chosenColor, categories, purity, width, height, 15)
						if err == nil {
							newResults = FilterAndPrioritizeWallpapers(newResults, wallpapersDir, 12)
						}
						resultChan <- refreshResult{results: newResults, err: err}
					}()

					// Fluid loading spinner animation frames
					frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
					frameIdx := 0

					for {
						select {
						case res := <-resultChan:
							if res.err == nil && len(res.results) > 0 {
								results = res.results
								cursorIdx = 0
								selected = make(map[int]bool)
								selected[0] = true
							} else {
								errMsg := "unknown error"
								if res.err != nil {
									errMsg = res.err.Error()
								}
								fmt.Printf("\r\n\r           \x1b[1;31mError fetching wallpapers:\x1b[0m %s\r\n", errMsg)
								time.Sleep(1500 * time.Millisecond)
							}
							goto endColorPicker
						case err := <-errChan:
							logDebug("Stdin error during color search: %v", err)
							return nil
						case kb := <-inputChan:
							if kb == 'q' || kb == 'Q' || kb == 27 || kb == 3 {
								return nil
							}
						default:
							fmt.Print("\x1b[H\x1b[2J")
							fmt.Print("\r\x1b[38;5;242m╔══════════════════════════════════════════════════════════════════════╗\x1b[0m\r\n")
							fmt.Print("\r\x1b[38;5;242m║\x1b[0m                 \x1b[1;38;5;51mHYPRWALL-DL INTERACTIVE SELECTOR\x1b[0m                    \x1b[38;5;242m║\x1b[0m\r\n")
							fmt.Print("\r\x1b[38;5;242m╚══════════════════════════════════════════════════════════════════════╝\x1b[0m\r\n\r\n")
							fmt.Printf("\r          \x1b[1;38;5;51m%s\x1b[0m \x1b[38;5;201mQuerying sources for aesthetic palette: \x1b[1;33m%s\x1b[0m\x1b[38;5;201m...\x1b[0m\r\n\r\n", frames[frameIdx], colorOptions[colorCursor].Name)
							fmt.Print("\r\x1b[38;5;242m========================================================================\x1b[0m\r\n")

							frameIdx = (frameIdx + 1) % len(frames)
							time.Sleep(80 * time.Millisecond)
						}
					}
				case 'q', 'Q', 27, 3: // Cancel / Quit back to list
					goto endColorPicker
				}
			}
		endColorPicker:
			// Loop continues, list re-drawn.
			_ = 0
		case 'r', 'R': // Refresh selection with a new set of random wallpapers
			// Clear Kitty preview first so it doesn't get drawn over the loading screen
			clearKittyPreview()

			type refreshResult struct {
				results []wallpaper.ImageData
				err     error
			}
			resultChan := make(chan refreshResult, 1)

			go func() {
				newResults, err := wallpaper.SearchAllSources(query, color, categories, purity, width, height, 15)
				if err == nil {
					newResults = FilterAndPrioritizeWallpapers(newResults, wallpapersDir, 12)
				}
				resultChan <- refreshResult{results: newResults, err: err}
			}()

			// Fluid loading spinner animation frames
			frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			frameIdx := 0

			for {
				select {
				case res := <-resultChan:
					if res.err == nil && len(res.results) > 0 {
						results = res.results
						cursorIdx = 0
						selected = make(map[int]bool)
						// Default pre-select the first one
						selected[0] = true
					} else {
						// Display error temporary state and wait a bit
						errMsg := "unknown error"
						if res.err != nil {
							errMsg = res.err.Error()
						}
						fmt.Printf("\r\n\r           \x1b[1;31mError refreshing wallpapers:\x1b[0m %s\r\n", errMsg)
						time.Sleep(1500 * time.Millisecond)
					}
					goto afterRefresh
				case err := <-errChan:
					logDebug("Stdin error during refresh: %v", err)
					return nil
				case kb := <-inputChan:
					// If the user wants to cancel or quit during the refresh loading screen
					if kb == 'q' || kb == 'Q' || kb == 27 || kb == 3 {
						logDebug("User cancelled or quit during refresh.")
						return nil
					}
					// Swallow other key inputs while loading to avoid buffer build-up
					logDebug("Swallowing input key during refresh: %d", kb)
				default:
					// Clear screen and draw loading spinner
					fmt.Print("\x1b[H\x1b[2J")
					fmt.Print("\r\x1b[38;5;242m╔══════════════════════════════════════════════════════════════════════╗\x1b[0m\r\n")
					fmt.Print("\r\x1b[38;5;242m║\x1b[0m                 \x1b[1;38;5;51mHYPRWALL-DL INTERACTIVE SELECTOR\x1b[0m                    \x1b[38;5;242m║\x1b[0m\r\n")
					fmt.Print("\r\x1b[38;5;242m╚══════════════════════════════════════════════════════════════════════╝\x1b[0m\r\n\r\n")
					fmt.Printf("\r          \x1b[1;38;5;51m%s\x1b[0m \x1b[38;5;201mFetching fresh wallpapers from all online sources...\x1b[0m\r\n\r\n", frames[frameIdx])
					fmt.Print("\r\x1b[38;5;242m========================================================================\x1b[0m\r\n")

					frameIdx = (frameIdx + 1) % len(frames)
					time.Sleep(80 * time.Millisecond)
				}
			}
		afterRefresh:
		}
	}
}

func drawScreen(ctx context.Context, results []wallpaper.ImageData, cursorIdx int, selected map[int]bool, targetCount int, wallpapersDir string) {
	// Standard ANSI codes: \x1b[H resets cursor to home, \x1b[2J clears screen
	fmt.Print("\x1b[H\x1b[2J")
	
	// Sleek Cyberpunk double-line HUD header
	fmt.Print("\r\x1b[38;5;242m╔══════════════════════════════════════════════════════════════════════╗\x1b[0m\r\n")
	fmt.Print("\r\x1b[38;5;242m║\x1b[0m                 \x1b[1;38;5;51mHYPRWALL-DL INTERACTIVE SELECTOR\x1b[0m                    \x1b[38;5;242m║\x1b[0m\r\n")
	fmt.Print("\r\x1b[38;5;242m╠══════════════════════════════════════════════════════════════════════╣\x1b[0m\r\n")
	fmt.Print("\r\x1b[38;5;242m║\x1b[0m \x1b[33m[Navigation]\x1b[0m  Up/Down or j/k    \x1b[38;5;242m│\x1b[0m  \x1b[33m[Toggle]\x1b[0m   Space to select     \x1b[38;5;242m║\x1b[0m\r\n")
	fmt.Print("\r\x1b[38;5;242m║\x1b[0m \x1b[33m[Confirm]\x1b[0m     Press Enter       \x1b[38;5;242m│\x1b[0m  \x1b[33m[Refresh]\x1b[0m  Press r to reload   \x1b[38;5;242m║\x1b[0m\r\n")
	fmt.Print("\r\x1b[38;5;242m║\x1b[0m \x1b[33m[Exit]\x1b[0m        Press q or Esc    \x1b[38;5;242m│\x1b[0m  \x1b[33m[Palette]\x1b[0m  Press c to pick     \x1b[38;5;242m║\x1b[0m\r\n")
	fmt.Print("\r\x1b[38;5;242m╚══════════════════════════════════════════════════════════════════════╝\x1b[0m\r\n\r\n")

	// Render selection list
	for i, res := range results {
		cursor := "   "
		if i == cursorIdx {
			cursor = "\x1b[1;38;5;201m▶▶ \x1b[0m"
		}

		checkbox := "\x1b[38;5;242m[ ]\x1b[0m"
		if selected[i] {
			checkbox = "\x1b[1;32m[✔]\x1b[0m"
		}

		dupStr := ""
		if IsDuplicate(wallpapersDir, res.ID) {
			dupStr = " \x1b[90m(Downloaded)\x1b[0m"
		}

		// Branded colorful source tags
		sourceTag := ""
		switch res.Source {
		case "Local":
			sourceTag = "\x1b[38;5;214m[Local]\x1b[0m "
		case "Reddit":
			sourceTag = "\x1b[38;5;202m[Reddit]\x1b[0m "
		case "Waifu.im":
			sourceTag = "\x1b[38;5;201m[Waifu.im]\x1b[0m "
		case "Picsum":
			sourceTag = "\x1b[38;5;36m[Picsum]\x1b[0m "
		case "NASA APOD":
			sourceTag = "\x1b[38;5;93m[NASA APOD]\x1b[0m "
		case "Bing Daily":
			sourceTag = "\x1b[38;5;220m[Bing Daily]\x1b[0m "
		case "Nekos":
			sourceTag = "\x1b[38;5;125m[Nekos]\x1b[0m "
		default: // Wallhaven
			sourceTag = "\x1b[38;5;51m[Wallhaven]\x1b[0m "
		}

		if i == cursorIdx {
			fmt.Printf("\r%s%s %s\x1b[1;38;5;51mID: %s\x1b[0m \x1b[38;5;242m|\x1b[0m \x1b[1mRes: %s\x1b[0m \x1b[38;5;242m|\x1b[0m Ratio: %s%s\r\n", cursor, checkbox, sourceTag, res.ID, res.Resolution, res.Ratio, dupStr)
		} else {
			fmt.Printf("\r%s%s %sID: %s \x1b[38;5;242m|\x1b[0m Res: %s \x1b[38;5;242m|\x1b[0m Ratio: %s%s\r\n", cursor, checkbox, sourceTag, res.ID, res.Resolution, res.Ratio, dupStr)
		}
	}

	fmt.Print("\r\n\x1b[38;5;242m════════════════════════════════════════════════════════════════════════\x1b[0m\r\n")

	// Render details side panel content underneath
	highlighted := results[cursorIdx]
	fmt.Printf("\r \x1b[1;38;5;51m✨ Highlighted Details:\x1b[0m\r\n")
	fmt.Printf("\r  \x1b[38;5;242m├─\x1b[0m \x1b[33mWallpaper ID:\x1b[0m  \x1b[1m%s\x1b[0m\r\n", highlighted.ID)
	fmt.Printf("\r  \x1b[38;5;242m├─\x1b[0m \x1b[33mSource Site:\x1b[0m   \x1b[1;35m%s\x1b[0m\r\n", highlighted.Source)
	fmt.Printf("\r  \x1b[38;5;242m├─\x1b[0m \x1b[33mResolution:\x1b[0m    %s\r\n", highlighted.Resolution)
	fmt.Printf("\r  \x1b[38;5;242m├─\x1b[0m \x1b[33mAspect Ratio:\x1b[0m  %s\r\n", highlighted.Ratio)
	fmt.Printf("\r  \x1b[38;5;242m└─\x1b[0m \x1b[33mURL Link:\x1b[0m      \x1b[4m%s\x1b[0m\r\n", highlighted.Path)

	// Asynchronously trigger/download thumbnail and display preview using Kitty Graphics protocol
	if highlighted.Thumbs.Small != "" {
		triggerPreview(ctx, highlighted.ID, highlighted.Thumbs.Small, len(results))
	}
}

func triggerPreview(ctx context.Context, imageID string, thumbURL string, numResults int) {
	thumbLock.Lock()
	if activeThumbID == imageID {
		thumbLock.Unlock()
		return
	}
	activeThumbID = imageID
	thumbLock.Unlock()

	go func() {
		if ctx.Err() != nil {
			return
		}

		pngFile := filepathJoin("/tmp", fmt.Sprintf("hyprwall_thumb_%s.png", imageID))
		isLocal := false
		if _, err := os.Stat(thumbURL); err == nil {
			isLocal = true
		}

		if isLocal {
			if err := prepareLocalPreview(thumbURL, pngFile); err != nil {
				logDebug("Failed to prepare local preview: %v", err)
				return
			}
		} else {
			client := &http.Client{Timeout: 10 * time.Second}
			req, err := http.NewRequestWithContext(ctx, "GET", thumbURL, nil)
			if err != nil {
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if ctx.Err() != nil {
				return
			}

			tmpFile := filepathJoin("/tmp", fmt.Sprintf("hyprwall_thumb_%s.jpg", imageID))
			out, err := os.Create(tmpFile)
			if err != nil {
				return
			}
			defer out.Close()

			_, err = io.Copy(out, resp.Body)
			out.Close() // Explicitly close the file to flush completely to disk
			if err != nil {
				return
			}

			if ctx.Err() != nil {
				os.Remove(tmpFile)
				return
			}

			// Convert downloaded JPEG to PNG since the official Kitty Graphics Protocol natively supports PNG (f=100)
			if err := convertJpegToPng(tmpFile, pngFile); err != nil {
				logDebug("Failed to convert JPEG to PNG: %v", err)
				return
			}
			os.Remove(tmpFile)
		}

		if ctx.Err() != nil {
			os.Remove(pngFile)
			return
		}

		// Double-check if the highlighted item has changed while we were downloading and converting
		thumbLock.Lock()
		currentID := activeThumbID
		thumbLock.Unlock()

		if currentID == imageID {
			// Clear previous kitty preview first
			clearKittyPreview()
			
			// Get current terminal size dynamically
			w, h, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				w, h = 80, 24
			}

			var previewCol, previewRow, previewWidth, previewHeight int
			if w >= 82 {
				previewWidth = 30
				previewHeight = 15
				previewCol = w - previewWidth - 1
				if previewCol < 50 {
					previewCol = 50
					previewWidth = w - previewCol - 1
				}
				previewRow = 4
			} else {
				previewWidth = 30
				if w-4 < previewWidth {
					previewWidth = w - 4
				}
				previewHeight = 12
				previewCol = 2
				previewRow = 16 + numResults
				
				// Make sure we don't render out of visual vertical boundaries
				if previewRow+previewHeight > h {
					previewHeight = h - previewRow - 1
				}
				if previewHeight < 5 {
					previewHeight = 5 // Absolute minimum height
				}
			}

			// Render new preview using direct Kitty escape sequence!
			renderKittyPreviewDirect(pngFile, previewCol, previewRow, previewWidth, previewHeight)
		}
	}()
}

func convertJpegToPng(jpegPath string, pngPath string) error {
	f, err := os.Open(jpegPath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, err := jpeg.Decode(f)
	if err != nil {
		return err
	}

	out, err := os.Create(pngPath)
	if err != nil {
		return err
	}
	defer out.Close()

	return png.Encode(out, img)
}

func prepareLocalPreview(srcPath, pngPath string) error {
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == ".png" {
		// Direct copy
		in, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(pngPath)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	}

	// Decode using registered image decoders (jpeg, png, gif, etc.)
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	out, err := os.Create(pngPath)
	if err != nil {
		return err
	}
	defer out.Close()

	return png.Encode(out, img)
}

// wrapEscapeSequence wraps standard terminal escape codes in tmux passthrough if TMUX is running.
func wrapEscapeSequence(seq string) string {
	if os.Getenv("TMUX") != "" || strings.Contains(os.Getenv("TERM"), "tmux") {
		escaped := strings.ReplaceAll(seq, "\x1b", "\x1b\x1b")
		return "\x1bPtmux;" + escaped + "\x1b\\"
	}
	return seq
}

func renderKittyPreviewDirect(filePath string, col, row, width, height int) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		logDebug("Failed to read PNG preview file: %v", err)
		return
	}

	b64Data := base64.StdEncoding.EncodeToString(data)
	chunkSize := 4096
	totalLen := len(b64Data)

	logDebug("Direct render chunked (PNG data): path=%s, col=%d, row=%d, w=%d, h=%d", filePath, col, row, width, height)

	// Save cursor and move
	fmt.Printf("\x1b[s\x1b[%d;%dH", row, col)

	for i := 0; i < totalLen; i += chunkSize {
		end := i + chunkSize
		m := 1
		if end >= totalLen {
			end = totalLen
			m = 0
		}

		chunk := b64Data[i:end]
		var seq string
		if i == 0 {
			// First chunk: a=T (Transmit & Display), f=100 (PNG), c=columns, r=rows, m=more
			seq = fmt.Sprintf("\x1b_Ga=T,f=100,c=%d,r=%d,m=%d;%s\x1b\\", width, height, m, chunk)
		} else {
			// Subsequent chunks: m=more/last, payload
			seq = fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", m, chunk)
		}
		
		fmt.Print(wrapEscapeSequence(seq))
	}

	// Restore cursor
	fmt.Print("\x1b[u")
}

// IsDuplicate checks if a wallpaper with the given ID already exists in the destination folder.
func IsDuplicate(destDir, imageID string) bool {
	if destDir == "" {
		return false
	}
	matches, err := filepath.Glob(filepath.Join(destDir, imageID+".*"))
	return err == nil && len(matches) > 0
}

// Simple path helper to avoid extra imports
func filepathJoin(a, b string) string {
	return a + "/" + b
}

func clearKittyPreview() {
	logDebug("Direct clear kitty preview")
	fmt.Print(wrapEscapeSequence("\x1b_Ga=d\x1b\\"))
}

// CleanTempThumbnails cleans temporary thumbnail files from /tmp.
func CleanTempThumbnails() {
	logDebug("Cleaning temporary thumbnail files from /tmp")
	files, err := filepath.Glob("/tmp/hyprwall_thumb_*")
	if err == nil {
		for _, f := range files {
			os.Remove(f)
		}
	}
}

// FilterAndPrioritizeWallpapers filters out duplicates and prioritizes undownloaded ones.
func FilterAndPrioritizeWallpapers(results []wallpaper.ImageData, wallpapersDir string, targetCount int) []wallpaper.ImageData {
	var undownloaded []wallpaper.ImageData
	var downloaded []wallpaper.ImageData

	for _, wall := range results {
		if IsDuplicate(wallpapersDir, wall.ID) {
			downloaded = append(downloaded, wall)
		} else {
			undownloaded = append(undownloaded, wall)
		}
	}

	var finalResults []wallpaper.ImageData
	finalResults = append(finalResults, undownloaded...)
	finalResults = append(finalResults, downloaded...)

	if len(finalResults) > targetCount {
		return finalResults[:targetCount]
	}
	return finalResults
}

func logDebug(format string, args ...interface{}) {
	// Write debug logs to /tmp/hyprwall_tui.log
	f, err := os.OpenFile("/tmp/hyprwall_tui.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		fmt.Fprintf(f, "[%s] "+format+"\n", append([]interface{}{time.Now().Format("15:04:05.000")}, args...)...)
		f.Close()
	}
}
