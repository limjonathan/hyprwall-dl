package wallpaper

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

// SearchLocalFolders scans configured local directories for matching wallpaper images.
func SearchLocalFolders(query string, folders []string, count int) ([]ImageData, error) {
	var results []ImageData
	if len(folders) == 0 {
		return nil, nil
	}

	seenPaths := make(map[string]bool)

	// Supported image extensions
	validExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}

	query = strings.ToLower(query)

	for _, folder := range folders {
		// Clean up environmental prefixes (like ~/)
		if strings.HasPrefix(folder, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				folder = filepath.Join(home, folder[1:])
			}
		}

		err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip directory read errors
			}

			if info.IsDir() {
				// Don't recurse excessively; keep it to direct children of specified directories
				if path != folder {
					return filepath.SkipDir
				}
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if !validExts[ext] {
				return nil
			}

			filename := info.Name()
			if query != "" && !strings.Contains(strings.ToLower(filename), query) {
				return nil
			}

			if seenPaths[path] {
				return nil
			}
			seenPaths[path] = true

			// Fast image header decoding to extract resolution & aspect ratio
			width, height := 1920, 1080
			f, err := os.Open(path)
			if err == nil {
				imgCfg, _, err := image.DecodeConfig(f)
				if err == nil && imgCfg.Width > 0 && imgCfg.Height > 0 {
					width = imgCfg.Width
					height = imgCfg.Height
				}
				f.Close()
			}

			h := sha1.New()
			h.Write([]byte(path))
			shortHash := hex.EncodeToString(h.Sum(nil))[:8]

			results = append(results, ImageData{
				ID:         shortHash,
				Path:       path,
				Resolution: fmt.Sprintf("%dx%d", width, height),
				Ratio:      calculateRatio(width, height),
				Thumbs: ThumbsData{
					Large: path,
					Small: path,
				},
				Source: "Local",
			})

			if len(results) >= count {
				return filepath.SkipDir // Stop walking once we satisfy our limits
			}
			return nil
		})

		if err != nil && err != filepath.SkipDir {
			continue
		}

		if len(results) >= count {
			break
		}
	}

	return results, nil
}
