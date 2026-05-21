# hyprwall-dl

A Go CLI utility to automatically download and organize wallpapers for your HyDE (Hyprland) themes.

## Features

- **Auto-detection**: Automatically detects your current HyDE theme and monitor resolution.
- **Theme Matching**: Fetches high-quality wallpapers from Wallhaven that match your theme's name.
- **Organization**: Saves wallpapers directly into the correct HyDE theme's `wallpapers` folder.
- **Seamless Integration**: Can optionally apply the downloaded wallpaper using HyDE's `swwwallpaper.sh`.

## Installation

Ensure you have [Go](https://go.dev/) installed.

```bash
git clone <repository-url>
cd hyprwall-dl
go build -o hyprwall-dl
```

## Usage

```bash
# Auto-detect theme and download a matching wallpaper
./hyprwall-dl

# Download 5 wallpapers for the current theme
./hyprwall-dl --count 5

# Specify a theme manually
./hyprwall-dl --theme "Catppuccin Mocha"

# Download and apply immediately
./hyprwall-dl --apply
```

## How it Works

1. Reads `~/.config/hypr/themes/theme.conf` for the current `$GTK_THEME`.
2. Uses `hyprctl monitors -j` to get the primary monitor's resolution.
3. Queries the Wallhaven API for a random image matching the theme and resolution.
4. Saves the image to `~/.config/hyde/themes/<Theme Name>/wallpapers/`.
5. If `--apply` is set, it triggers `~/.local/lib/hyde/swwwallpaper.sh`.
