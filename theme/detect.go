package theme

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DetectTheme returns the current theme name and the path to its wallpapers directory.
func DetectTheme() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get home directory: %w", err)
	}

	themeConfPath := filepath.Join(home, ".config/hypr/themes/theme.conf")
	themeName, err := parseGTKTheme(themeConfPath)
	if err != nil {
		return "", "", err
	}

	themeDir, err := FindThemeDir(home, themeName)
	if err != nil {
		return "", "", err
	}

	wallpapersDir := filepath.Join(themeDir, "wallpapers")
	if _, err := os.Stat(wallpapersDir); os.IsNotExist(err) {
		return themeName, "", fmt.Errorf("wallpapers directory does not exist: %s", wallpapersDir)
	}

	return themeName, wallpapersDir, nil
}

func parseGTKTheme(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open theme.conf: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "$GTK_THEME") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading theme.conf: %w", err)
	}

	return "", fmt.Errorf("$GTK_THEME not found in %s", path)
}

func FindThemeDir(home, themeName string) (string, error) {
	hydeThemesDir := filepath.Join(home, ".config/hyde/themes")
	entries, err := os.ReadDir(hydeThemesDir)
	if err != nil {
		return "", fmt.Errorf("failed to read HyDE themes directory: %w", err)
	}

	// Try exact match first
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == themeName {
			return filepath.Join(hydeThemesDir, entry.Name()), nil
		}
	}

	// Try matching with spaces replaced by hyphens or vice versa
	normalizedTheme := strings.ToLower(strings.ReplaceAll(themeName, "-", " "))
	for _, entry := range entries {
		if entry.IsDir() {
			normalizedEntry := strings.ToLower(strings.ReplaceAll(entry.Name(), "-", " "))
			if normalizedEntry == normalizedTheme {
				return filepath.Join(hydeThemesDir, entry.Name()), nil
			}
		}
	}

	return "", fmt.Errorf("could not find HyDE theme directory for: %s", themeName)
}

// GetActiveBorderColor parses the first active border color hex code (6 characters) from theme.conf.
func GetActiveBorderColor(home string) (string, error) {
	themeConfPath := filepath.Join(home, ".config/hypr/themes/theme.conf")
	file, err := os.Open(themeConfPath)
	if err != nil {
		return "", fmt.Errorf("failed to open theme.conf: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "col.active_border") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				// Find first color block, which could be rgba(bb9af7ff), rgb(bb9af7), 0xffbb9af7, or similar
				fields := strings.Fields(val)
				if len(fields) == 0 {
					continue
				}
				colorPart := fields[0] // Grab the first word, e.g., "rgba(bb9af7ff)"

				// Extract digits if inside rgba(...) or rgb(...)
				if strings.Contains(colorPart, "rgba(") {
					idx := strings.Index(colorPart, "rgba(")
					sub := colorPart[idx+5:]
					if endIdx := strings.Index(sub, ")"); endIdx != -1 {
						hexStr := strings.TrimSpace(sub[:endIdx])
						if len(hexStr) >= 6 {
							return hexStr[:6], nil
						}
					}
				} else if strings.Contains(colorPart, "rgb(") {
					idx := strings.Index(colorPart, "rgb(")
					sub := colorPart[idx+4:]
					if endIdx := strings.Index(sub, ")"); endIdx != -1 {
						hexStr := strings.TrimSpace(sub[:endIdx])
						if len(hexStr) >= 6 {
							return hexStr[:6], nil
						}
					}
				} else if strings.HasPrefix(colorPart, "0x") {
					hexStr := colorPart[2:]
					if len(hexStr) >= 8 {
						// 0xaarrggbb -> rrggbb is at the end (last 6 digits)
						return hexStr[len(hexStr)-6:], nil
					} else if len(hexStr) == 6 {
						return hexStr, nil
					}
				} else {
					// Fallback: strip standard non-hex characters and find a 6 or 8 character hex string
					clean := strings.NewReplacer("rgba", "", "rgb", "", "0x", "", "(", "", ")", "").Replace(colorPart)
					if len(clean) >= 6 {
						return clean[:6], nil
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading theme.conf: %w", err)
	}

	return "", fmt.Errorf("col.active_border not found in theme.conf")
}
