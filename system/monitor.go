package system

import (
	"encoding/json"
	"os/exec"
)

type Monitor struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Focused bool   `json:"focused"`
}

// GetResolution returns the width and height of the focused monitor.
// Fallback to 1920x1080 if hyprctl fails or no monitor is found.
func GetResolution() (int, int) {
	defaultWidth, defaultHeight := 1920, 1080

	cmd := exec.Command("hyprctl", "monitors", "-j")
	output, err := cmd.Output()
	if err != nil {
		return defaultWidth, defaultHeight
	}

	var monitors []Monitor
	if err := json.Unmarshal(output, &monitors); err != nil {
		return defaultWidth, defaultHeight
	}

	if len(monitors) == 0 {
		return defaultWidth, defaultHeight
	}

	// Find the focused monitor or just the first one
	for _, m := range monitors {
		if m.Focused {
			return m.Width, m.Height
		}
	}

	return monitors[0].Width, monitors[0].Height
}
