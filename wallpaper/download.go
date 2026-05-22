package wallpaper

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadImage downloads an image and saves it to the specified directory with a branded name.
func DownloadImage(wall ImageData, destDir string) (string, error) {
	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Resolve file extension
	ext := filepath.Ext(wall.Path)
	if ext == "" || len(ext) > 5 {
		ext = ".jpg" // Default fallback
	}
	// Clean query parameters if any (e.g. ?width=...)
	if idx := strings.Index(ext, "?"); idx != -1 {
		ext = ext[:idx]
	}

	// Filename matching exact unique ID (includes branded source prefix)
	filename := fmt.Sprintf("%s%s", wall.ID, ext)
	destPath := filepath.Join(destDir, filename)

	// Check if this represents a local file
	isLocal := false
	if _, err := os.Stat(wall.Path); err == nil {
		isLocal = true
	}

	if isLocal {
		in, err := os.Open(wall.Path)
		if err != nil {
			return "", fmt.Errorf("failed to open local source file: %w", err)
		}
		defer in.Close()

		out, err := os.Create(destPath)
		if err != nil {
			return "", fmt.Errorf("failed to create local copy target: %w", err)
		}
		defer out.Close()

		if _, err := io.Copy(out, in); err != nil {
			return "", fmt.Errorf("failed to copy local file: %w", err)
		}
		return destPath, nil
	}

	// Download the file with a 30-second client-side timeout
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(wall.Path)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download image, status: %s", resp.Status)
	}

	// Create the local file
	out, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	return destPath, nil
}
