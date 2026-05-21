package wallpaper

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DownloadImage downloads an image from the URL and saves it to the specified directory.
func DownloadImage(imageURL, destDir string) (string, error) {
	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Extract filename from URL or generate a unique one
	filename := filepath.Base(imageURL)
	if filename == "" || filename == "." || filename == "/" {
		filename = fmt.Sprintf("wallpaper_%d.jpg", time.Now().Unix())
	}

	destPath := filepath.Join(destDir, filename)

	// Download the file with a 30-second client-side timeout
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(imageURL)
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
