package system

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
	"hyprwall-dl/wallpaper"
)

var (
	thumbLock     sync.Mutex
	activeThumbID string
)

// RunTUI launches the interactive terminal selector with live previews.
func RunTUI(results []wallpaper.ImageData, targetCount int) []wallpaper.ImageData {
	if len(results) == 0 {
		return nil
	}

	// 1. Enter terminal raw mode to read arrow keys/keypresses instantly
	if err := setTerminalRaw(true); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize raw terminal mode: %v\n", err)
	}
	defer setTerminalRaw(false)

	// Clear previous kitty preview icons on start/exit
	clearKittyPreview()
	defer clearKittyPreview()

	cursorIdx := 0
	selected := make(map[int]bool)
	
	// Default: pre-select the first item
	selected[0] = true

	for {
		// Draw the complete TUI screen
		drawScreen(results, cursorIdx, selected, targetCount)

		// Read key input
		buf := make([]byte, 3)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}

		if n == 0 {
			continue
		}

		// Handle Vim keys, Esc, Enter, Space, Arrows
		var key byte
		if n == 3 && buf[0] == 27 && buf[1] == 91 {
			// Escape sequence (Arrow keys)
			switch buf[2] {
			case 65: // Arrow Up
				key = 'k'
			case 66: // Arrow Down
				key = 'j'
			}
		} else {
			key = buf[0]
		}

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
		}
	}
	return nil
}

func setTerminalRaw(raw bool) error {
	var cmd *exec.Cmd
	if raw {
		cmd = exec.Command("stty", "raw", "-echo")
	} else {
		cmd = exec.Command("stty", "-raw", "echo")
	}
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func drawScreen(results []wallpaper.ImageData, cursorIdx int, selected map[int]bool, targetCount int) {
	// Standard ANSI codes: \x1b[H resets cursor to home, \x1b[2J clears screen
	fmt.Print("\x1b[H\x1b[2J")
	
	fmt.Print("\r========================================================================\r\n")
	fmt.Print("\r                     HYPRWALL-DL INTERACTIVE SELECTOR                   \r\n")
	fmt.Print("\r========================================================================\r\n")
	fmt.Print("\r [Navigation]  Up/Down or j/k to scroll  |  [Toggle] Space to check/uncheck\r\n")
	fmt.Print("\r [Confirm]     Press Enter to download   |  [Exit]   q or Esc to cancel\r\n")
	fmt.Print("\r========================================================================\r\n\r\n")

	// Render selection list
	for i, res := range results {
		cursor := "  "
		if i == cursorIdx {
			cursor = "> "
		}

		checkbox := "[ ]"
		if selected[i] {
			checkbox = "[x]"
		}

		fmt.Printf("\r%s%s ID: %s | Res: %s | Ratio: %s\r\n", cursor, checkbox, res.ID, res.Resolution, res.Ratio)
	}

	fmt.Print("\r\n========================================================================\r\n")

	// Render details side panel content underneath
	highlighted := results[cursorIdx]
	fmt.Printf("\r Highlighted Details:\r\n")
	fmt.Printf("\r - Wallhaven ID: %s\r\n", highlighted.ID)
	fmt.Printf("\r - Resolution:   %s\r\n", highlighted.Resolution)
	fmt.Printf("\r - Aspect Ratio: %s\r\n", highlighted.Ratio)
	fmt.Printf("\r - Full Path URL: %s\r\n", highlighted.Path)

	// Asynchronously trigger/download thumbnail and display preview using Kitty Graphics protocol
	if highlighted.Thumbs.Small != "" {
		triggerPreview(highlighted.ID, highlighted.Thumbs.Small)
	}
}

func triggerPreview(imageID string, thumbURL string) {
	thumbLock.Lock()
	if activeThumbID == imageID {
		thumbLock.Unlock()
		return
	}
	activeThumbID = imageID
	thumbLock.Unlock()

	go func() {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(thumbURL)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		tmpFile := filepathJoin("/tmp", fmt.Sprintf("hyprwall_thumb_%s.jpg", imageID))
		out, err := os.Create(tmpFile)
		if err != nil {
			return
		}
		defer out.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return
		}

		// Double-check if the highlighted item has changed while we were downloading
		thumbLock.Lock()
		currentID := activeThumbID
		thumbLock.Unlock()

		if currentID == imageID {
			// Clear previous kitty preview first
			clearKittyPreview()
			// Render new preview on the right hand side (col 50, row 4)
			exec.Command("kitty", "+kitten", "icat", "--place", "30x15@52x4", tmpFile).Run()
		}
	}()
}

// Simple path helper to avoid extra imports
func filepathJoin(a, b string) string {
	return a + "/" + b
}

func clearKittyPreview() {
	exec.Command("kitty", "+kitten", "icat", "--clear").Run()
}
