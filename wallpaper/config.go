package wallpaper

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Sources       SourcesConfig `json:"sources"`
	LocalFolders  []string      `json:"local_folders"`
	Tui           TuiConfig     `json:"tui"`
	DefaultPurity string        `json:"default_purity"`
	ApplyBackend  string        `json:"apply_backend"`
}

type SourcesConfig struct {
	Wallhaven WallhavenConfig `json:"wallhaven"`
	Reddit    RedditConfig    `json:"reddit"`
	Waifuim   EnabledConfig   `json:"waifuim"`
	Picsum    EnabledConfig   `json:"picsum"`
	Nasa      EnabledConfig   `json:"nasa"`
	Bing      EnabledConfig   `json:"bing"`
	Nekos     EnabledConfig   `json:"nekos"`
}

type EnabledConfig struct {
	Enabled bool `json:"enabled"`
}

type WallhavenConfig struct {
	Enabled    bool   `json:"enabled"`
	Categories string `json:"categories"`
	Purity     string `json:"purity"`
}

type RedditConfig struct {
	Enabled    bool     `json:"enabled"`
	Subreddits []string `json:"subreddits"`
}

type TuiConfig struct {
	MaxResults        int  `json:"max_results"`
	PreloadThumbnails bool `json:"preload_thumbnails"`
}

var ActiveConfig Config

// LoadConfig resolves, initializes, and loads configuration definitions in a non-circular package path.
func LoadConfig() (Config, error) {
	var cfg Config
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}

	configDir := filepath.Join(home, ".config", "hyprwall-dl")
	configPath := filepath.Join(configDir, "config.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Initialize with default configurations
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return cfg, err
		}
		cfg = GetDefaultConfig()
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return cfg, err
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return cfg, err
		}
		ActiveConfig = cfg
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	ActiveConfig = cfg
	return cfg, nil
}

// GetDefaultConfig returns the standard fallback configurations.
func GetDefaultConfig() Config {
	home, _ := os.UserHomeDir()
	defaultPicDir := filepath.Join(home, "Pictures")
	if _, err := os.Stat(defaultPicDir); err != nil {
		defaultPicDir = home
	}
	return Config{
		Sources: SourcesConfig{
			Wallhaven: WallhavenConfig{
				Enabled:    true,
				Categories: "111",
				Purity:     "100",
			},
			Reddit: RedditConfig{
				Enabled: true,
				Subreddits: []string{
					"wallpaper",
					"wallpapers",
					"Cyberpunk",
					"EarthPorn",
					"AnimeWallpapers",
					"unixporn",
				},
			},
			Waifuim: EnabledConfig{Enabled: true},
			Picsum:  EnabledConfig{Enabled: true},
			Nasa:    EnabledConfig{Enabled: true},
			Bing:    EnabledConfig{Enabled: true},
			Nekos:   EnabledConfig{Enabled: true},
		},
		LocalFolders: []string{
			defaultPicDir,
		},
		Tui: TuiConfig{
			MaxResults:        12,
			PreloadThumbnails: true,
		},
		DefaultPurity: "sfw",
		ApplyBackend:  "auto",
	}
}
