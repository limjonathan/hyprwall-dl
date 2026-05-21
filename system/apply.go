package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// detectActiveBackend resolves the current active wallpaper backend by checking environment variables and running daemons.
func detectActiveBackend() string {
	if env := os.Getenv("WALLPAPER_BACKEND"); env != "" {
		return env
	}

	// Check running wallpaper daemons on Wayland/Hyprland
	daemons := []string{"swww-daemon", "hyprpaper", "mpvpaper"}
	for _, daemon := range daemons {
		cmd := exec.Command("pgrep", "-x", daemon)
		if err := cmd.Run(); err == nil {
			if daemon == "swww-daemon" {
				return "swww"
			}
			return daemon
		}
	}

	return "awww"
}

// ApplyWallpaper applies the wallpaper using the HyDE wallpaper.sh script (falling back to legacy versions).
func ApplyWallpaper(imagePath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	backend := detectActiveBackend()

	// 1. Try modern HyDE wallpaper.sh directly
	modernScript := filepath.Join(home, ".local/lib/hyde/wallpaper.sh")
	if _, err := os.Stat(modernScript); err == nil {
		cmd := exec.Command(modernScript, imagePath, "--backend", backend, "--global")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run wallpaper.sh: %w", err)
		}
		return nil
	}

	// 2. Try modern wallpaper.sh on the global PATH
	if path, err := exec.LookPath("wallpaper.sh"); err == nil {
		cmd := exec.Command(path, imagePath, "--backend", backend, "--global")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run wallpaper.sh in PATH: %w", err)
		}
		return nil
	}

	// 3. Fallback to deprecated swwwallpaper.sh wrapper
	legacyScript := filepath.Join(home, ".local/lib/hyde/swwwallpaper.sh")
	if _, err := os.Stat(legacyScript); err == nil {
		cmd := exec.Command(legacyScript, imagePath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run swwwallpaper.sh: %w", err)
		}
		return nil
	}

	// 4. Fallback to swww directly
	if _, err := exec.LookPath("swww"); err == nil {
		cmd := exec.Command("swww", "img", imagePath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run swww img: %w", err)
		}
		return nil
	}

	return fmt.Errorf("could not find wallpaper.sh, swwwallpaper.sh, or swww to apply wallpaper")
}

// SendNotification triggers a native desktop notification with an image thumbnail.
func SendNotification(imagePath string, themeName string, width, height int) error {
	if _, err := exec.LookPath("notify-send"); err != nil {
		// If notify-send isn't installed, silently fail or return nil
		return nil
	}

	resStr := strconv.Itoa(width) + "x" + strconv.Itoa(height)
	body := fmt.Sprintf("Theme: %s\nResolution: %s", themeName, resStr)

	// notify-send -i <imagePath> "Wallpaper Applied" "<body text>"
	cmd := exec.Command("notify-send", "-i", imagePath, "Wallpaper Applied", body)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to trigger notification: %w", err)
	}

	return nil
}
