package main

import (
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"hyprwall-dl/system"
	"hyprwall-dl/theme"
	"hyprwall-dl/wallpaper"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type downloadResult struct {
	path  string
	err   error
	index int
	id    string
}

func main() {
	defer system.CleanTempThumbnails()

	// Initialize user configurations
	if _, err := wallpaper.LoadConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize configuration: %v\n", err)
	}

	// CLI Flags
	themeFlag := flag.String("theme", "", "Specify a theme name instead of auto-detecting")
	applyFlag := flag.Bool("apply", false, "Automatically apply the downloaded wallpaper")
	countFlag := flag.Int("count", 1, "Number of wallpapers to download")
	helpFlag := flag.Bool("help", false, "Show help")

	// Custom option C enhancements
	queryFlag := flag.String("query", "", "Custom query term (overrides default theme-based search)")
	qFlag := flag.String("q", "", "Custom query term (shorthand)")
	colorMatchFlag := flag.Bool("color-match", false, "Extract and match the current HyDE window active border color")
	cFlag := flag.Bool("c", false, "Extract and match active border color (shorthand)")
	notifyFlag := flag.Bool("notify", false, "Trigger a desktop notification with thumbnail upon completion")
	nFlag := flag.Bool("n", false, "Trigger desktop notification (shorthand)")
	selectFlag := flag.Bool("select", false, "Interactive TUI selector to pick specific search results")
	sFlag := flag.Bool("s", false, "Interactive TUI selector (shorthand)")

	// Wallhaven filter flags
	animeFlag := flag.Bool("anime", false, "Include/filter Wallhaven category 'Anime'")
	generalFlag := flag.Bool("general", false, "Include/filter Wallhaven category 'General'")
	peopleFlag := flag.Bool("people", false, "Include/filter Wallhaven category 'People'")
	purityFlag := flag.String("purity", "sfw", "Purity filter (sfw, sketchy, nsfw, sfw,sketchy, all)")

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	// 1. Validation & Pre-checks
	if *countFlag < 1 {
		fmt.Fprintf(os.Stderr, "Error: --count must be at least 1\n")
		os.Exit(1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting user home directory: %v\n", err)
		os.Exit(1)
	}

	// Shorthand consolidations
	searchQueryOverride := *queryFlag
	if *qFlag != "" {
		searchQueryOverride = *qFlag
	}

	colorMatch := *colorMatchFlag || *cFlag
	notifyEnabled := *notifyFlag || *nFlag
	selectMode := *selectFlag || *sFlag

	// 2. Resolve Theme Name and Directory
	var themeName, wallpapersDir string

	if *themeFlag != "" {
		themeName = *themeFlag
		themeDir, err := theme.FindThemeDir(home, themeName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding theme directory: %v\n", err)
			os.Exit(1)
		}
		wallpapersDir = filepath.Join(themeDir, "wallpapers")
	} else {
		themeName, wallpapersDir, err = theme.DetectTheme()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error detecting theme: %v\n", err)
			os.Exit(1)
		}
	}

	// 3. Resolve Monitor Details
	width, height := system.GetResolution()

	// 4. Color Matching Parser
	var colorParam string
	if colorMatch {
		var colorErr error
		colorParam, colorErr = theme.GetActiveBorderColor(home)
		if colorErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to extract theme active border color: %v\n", colorErr)
		} else {
			fmt.Printf("Aesthetics: Matched active border color hex -> #%s\n", colorParam)
		}
	}

	// 5. Build Wallhaven Categories Param (General/Anime/People mask, e.g. 111)
	categories := "111" // Default to search everything
	if *generalFlag || *animeFlag || *peopleFlag {
		g := "0"
		if *generalFlag {
			g = "1"
		}
		a := "0"
		if *animeFlag {
			a = "1"
		}
		p := "0"
		if *peopleFlag {
			p = "1"
		}
		categories = g + a + p
	}

	// 6. Build Purity Param (SFW/Sketchy/NSFW mask, e.g. 100)
	purityVal := "100" // Default SFW
	switch strings.ToLower(*purityFlag) {
	case "sfw":
		purityVal = "100"
	case "sketchy":
		purityVal = "010"
	case "nsfw":
		purityVal = "001"
	case "sfw,sketchy", "sketchy,sfw":
		purityVal = "110"
	case "sketchy,nsfw", "nsfw,sketchy":
		purityVal = "011"
	case "sfw,nsfw", "nsfw,sfw":
		purityVal = "101"
	case "all", "sfw,sketchy,nsfw":
		purityVal = "111"
	}

	// Determine query parameter
	query := themeName
	if searchQueryOverride != "" {
		query = searchQueryOverride
	}

	// Summary output
	fmt.Printf("Theme:         %s\n", themeName)
	fmt.Printf("Destination:   %s\n", wallpapersDir)
	fmt.Printf("Resolution:    %dx%d\n", width, height)
	if searchQueryOverride != "" {
		fmt.Printf("Custom Query:  %s\n", searchQueryOverride)
	}
	fmt.Printf("Purity:        %s\n", *purityFlag)

	// In TUI selection mode, retrieve up to 10 matching candidate wallpapers to choose from
	fetchCount := *countFlag
	if selectMode && fetchCount < 10 {
		fetchCount = 10
	}

	// 7. Search all online sources
	fmt.Printf("Searching all online sources for matching wallpapers...\n")
	results, err := wallpaper.SearchAllSources(query, colorParam, categories, purityVal, width, height, fetchCount)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching wallpapers: %v\n", err)
		os.Exit(1)
	}

	// 8. TUI Selector Mode
	var downloadList []wallpaper.ImageData
	if selectMode {
		downloadList = system.RunTUI(results, *countFlag, query, colorParam, categories, purityVal, width, height, wallpapersDir)
		if len(downloadList) == 0 {
			fmt.Println("No selections made or cancelled. Exiting.")
			system.CleanTempThumbnails()
			os.Exit(0)
		}
	} else {
		// Standard automatic download: skip already downloaded wallpapers
		var downloadListToRun []wallpaper.ImageData
		for _, wall := range results {
			if system.IsDuplicate(wallpapersDir, wall.ID) {
				fmt.Printf("Wallpaper ID %s is already downloaded, skipping.\n", wall.ID)
				continue
			}
			downloadListToRun = append(downloadListToRun, wall)
			if len(downloadListToRun) == *countFlag {
				break
			}
		}

		downloadList = downloadListToRun
		if len(downloadList) == 0 {
			fmt.Println("All wallpapers are already downloaded. Exiting.")
			os.Exit(0)
		}
	}

	// 9. Concurrent Goroutine Downloader
	fmt.Printf("\nDownloading %d wallpaper(s) concurrently...\n", len(downloadList))
	var wg sync.WaitGroup
	resultChan := make(chan downloadResult, len(downloadList))

	for idx, wall := range downloadList {
		wg.Add(1)
		go func(i int, w wallpaper.ImageData) {
			defer wg.Done()
			savedPath, downloadErr := wallpaper.DownloadImage(w, wallpapersDir)
			resultChan <- downloadResult{path: savedPath, err: downloadErr, index: i, id: w.ID}
		}(idx, wall)
	}

	wg.Wait()
	close(resultChan)

	// Process and print download results ordered or grouped
	var lastSavedPath string
	var successCount int
	savedPathsOrdered := make([]string, len(downloadList))

	for res := range resultChan {
		if res.err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading wallpaper index %d: %v\n", res.index+1, res.err)
		} else {
			savedPathsOrdered[res.index] = res.path
			successCount++
			
			// Warm up the HyDE cache natively using the downloaded wallpaper and the ID
			warmUpHydeCache(res.id, res.path)
		}
	}

	// Find the last successfully saved file path
	for i := len(savedPathsOrdered) - 1; i >= 0; i-- {
		if savedPathsOrdered[i] != "" {
			lastSavedPath = savedPathsOrdered[i]
			break
		}
	}

	fmt.Printf("\nSuccessfully downloaded %d of %d requested wallpaper(s).\n", successCount, len(downloadList))

	// 10. Wallpaper Application & Native Notifications
	if lastSavedPath != "" {
		if *applyFlag {
			fmt.Println("Applying the last downloaded wallpaper...")
			if err := system.ApplyWallpaper(lastSavedPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error applying wallpaper: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Wallpaper applied successfully!")
		}

		if notifyEnabled {
			fmt.Println("Sending desktop notification with image thumbnail...")
			if err := system.SendNotification(lastSavedPath, themeName, width, height); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to trigger notification: %v\n", err)
			}
		}
	}
}

// warmUpHydeCache calculates the SHA1 hash of the downloaded wallpaper and copies any TUI thumbnail preview to ~/.cache/hyde/thumbs/
func warmUpHydeCache(imageID, savedPath string) {
	if savedPath == "" {
		return
	}

	// 1. Calculate SHA1 hash of the downloaded high-resolution wallpaper
	f, err := os.Open(savedPath)
	if err != nil {
		return
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return
	}
	f.Close()

	sha1Hash := hex.EncodeToString(h.Sum(nil))

	// 2. Locate temporary preview file (Kitty thumbnail)
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	hydeCacheThumbsDir := filepath.Join(home, ".cache/hyde/thumbs")

	// Ensure the thumbs directory exists
	if _, err := os.Stat(hydeCacheThumbsDir); os.IsNotExist(err) {
		return
	}

	srcFile := ""
	pngFile := filepath.Join("/tmp", fmt.Sprintf("hyprwall_thumb_%s.png", imageID))
	jpgFile := filepath.Join("/tmp", fmt.Sprintf("hyprwall_thumb_%s.jpg", imageID))

	if _, err := os.Stat(pngFile); err == nil {
		srcFile = pngFile
	} else if _, err := os.Stat(jpgFile); err == nil {
		srcFile = jpgFile
	} else {
		return
	}

	// 3. Copy to ~/.cache/hyde/thumbs/<SHA1>.thmb and ~/.cache/hyde/thumbs/<SHA1>.sqre
	copyFile := func(src, dest string) error {
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		return err
	}

	thmbDest := filepath.Join(hydeCacheThumbsDir, sha1Hash+".thmb")
	sqreDest := filepath.Join(hydeCacheThumbsDir, sha1Hash+".sqre")

	_ = copyFile(srcFile, thmbDest)
	_ = copyFile(srcFile, sqreDest)
}
