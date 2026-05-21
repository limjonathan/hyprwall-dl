# hyprwall-dl

A robust, high-performance Go CLI utility to automatically download, organize, and cache high-quality wallpapers matching your HyDE (Hyprland) themes.

## Features

- **Auto-detection**: Automatically detects your current active HyDE theme and primary monitor resolution.
- **Massive Multi-Source Wallpaper Engine**: Keylessly aggregates and shuffles high-quality wallpapers concurrently from **7 distinct backends**:
  - `[Wallhaven]` — Premium community-sourced artwork database.
  - `[Reddit]` — Curated high-resolution image subreddits (e.g. `r/wallpaper`, `r/EarthPorn`, `r/unixporn`).
  - `[Waifu.im]` — Public anime wallpaper database with purity filters.
  - `[Picsum]` — Ultra-high-resolution landscape and general photography.
  - `[NASA APOD]` — Breathtaking daily deep space/astronomy cosmic collections.
  - `[Bing Daily]` — Nature landscape homepage archive.
  - `[Nekos]` — Desktop vector and digital anime artwork.
- **Color-Aesthetic Matching**: Extracts active HyDE window border color hex codes, translates them using Euclidean distance to standard color words, and implements a 3-tiered fallback search (`combined` $\rightarrow$ `color-only` $\rightarrow$ `query-only`) to match theme color palettes.
- **Cyberpunk TUI Selector**: An interactive double-line HUD panel featuring:
  - Color-coded source tags identifying matching endpoints.
  - Sandbox-agnostic, Base64-chunked stdout thumbnail live previews compatible inside and outside terminal multiplexers like `tmux`.
  - Non-blocking input loading screens allowing quick cancellation and safe input-swallowing.
- **Branded Organization**: Concurrently downloads images and names them with source-branded IDs (`<source>-<id>.<ext>`) directly into the correct HyDE theme's `wallpapers/` directory.
- **HyDE Native Cache Warming**: Automatically calculates SHA1 hashes of downloaded high-resolution wallpapers and populates the native HyDE thumbnail cache (`~/.cache/hyde/thumbs/`) so thumbnails load instantly inside HyDE shell scripts.

## Installation

Ensure you have [Go](https://go.dev/) installed.

```bash
git clone <repository-url>
cd hyprwall-dl
go build -o hyprwall-dl
```

## Usage

```bash
# Auto-detect theme, query all 7 sources concurrently, and download a matching wallpaper
./hyprwall-dl

# Download 5 wallpapers concurrently for the current theme
./hyprwall-dl --count 5

# Launch interactive Cyberpunk TUI selector to pick specific wallpapers from all sources
./hyprwall-dl --select

# Launch TUI selector matching both theme query and window active border colors
./hyprwall-dl --select --color-match

# Download, warm up HyDE cache, and apply immediately using HyDE native wallpaper scripts
./hyprwall-dl --apply --count 1

# Specify a theme manually
./hyprwall-dl --theme "Catppuccin Mocha"
```

## How it Works

1. Reads your active GTK theme and color definitions from your HyDE configuration folder.
2. Extracts active window border hex codes via HyDE system variables.
3. Queries primary monitor resolution using `hyprctl monitors -j`.
4. Concurrently queries all 7 keyless API backends with 3-tiered fallback parameters.
5. Shuffles results and opens the terminal-raw interactive visual selector if `--select` is specified.
6. Saves wallpapers with source-prefixed names in `~/.config/hyde/themes/<Theme Name>/wallpapers/`.
7. Regenerates SHA1 hashes to pre-warm HyDE's native thumbnail cache.
8. If `--apply` is set, dynamically detects active backends (e.g. `swww`, `hyprpaper`, `mpvpaper`) and applies the wallpaper using `wallpaper.sh`.
