# hyprwall-dl

A robust, high-performance Go CLI utility to automatically download, organize, and cache high-quality wallpapers matching your HyDE (Hyprland) themes.

## Features

- **Auto-detection & Accent-Insensitive Theme Resolver**: Automatically detects your active GTK theme and primary monitor resolution. Intelligently normalizes accents and symbols (e.g. mapping `Rose-Pine` to `Rosé Pine` directory) to avoid path lookup crashes.
- **Massive Multi-Source Wallpaper Engine**: Keylessly aggregates and shuffles high-quality wallpapers concurrently from **8 distinct backends**:
  - `[Wallhaven]` — Premium community-sourced artwork database.
  - `[Reddit]` — Curated high-resolution image subreddits (e.g. `r/wallpaper`, `r/EarthPorn`, `r/unixporn`).
  - `[Waifu.im]` — Public anime wallpaper database with purity filters.
  - `[Picsum]` — Ultra-high-resolution landscape and general photography.
  - `[NASA APOD]` — Breathtaking daily deep space/astronomy cosmic collections.
  - `[Bing Daily]` — Nature landscape homepage archive.
  - `[Nekos]` — Desktop vector and digital anime artwork.
  - `[Local]` — Custom local directories parsed concurrently, complete with keyword/color filters.
- **Color-Aesthetic Matching**: Extracts active HyDE window border color hex codes, translates them using Euclidean distance to standard color words, and implements a 3-tiered fallback search (`combined` $\rightarrow$ `color-only` $\rightarrow$ `query-only`) to match theme color palettes.
- **Cyberpunk TUI Selector**: An interactive double-line HUD panel featuring:
  - Color-coded source tags (with amber tags for `[Local]`) identifying matching endpoints.
  - Sandbox-agnostic, Base64-chunked stdout thumbnail live previews compatible inside and outside terminal multiplexers like `tmux`.
  - Non-blocking input loading screens allowing quick cancellation and safe input-swallowing.
  - **Color Palette Selector Overlay**: Suspend drawing at any time to open a gorgeous cyberpunk border overlay, picking from 10 vibrant primary/neon colors to immediately refresh queries.
- **Structured JSON Configuration**: Custom settings dynamically loaded and initialized at `~/.config/hyprwall-dl/config.json`. Customise source state, subreddits, Wallhaven rules, TUI search limits, and local crawling directories.
- **Branded Organization**: Concurrently downloads online images (`<source>-<id>.<ext>`) or copies local candidates (`local-<sha1-hash>.<ext>`) directly into the correct HyDE theme's `wallpapers/` directory.
- **HyDE Native Cache Warming**: Automatically calculates SHA1 hashes of downloaded high-resolution wallpapers and populates the native HyDE thumbnail cache (`~/.cache/hyde/thumbs/`) so thumbnails load instantly inside HyDE shell scripts.

## Installation

Ensure you have [Go](https://go.dev/) installed.

```bash
git clone <repository-url>
cd hyprwall-dl
go build -o hyprwall-dl
```

## Configuration

The utility automatically initializes a configuration file at `~/.config/hyprwall-dl/config.json` on the first launch. You can customize the source engines and declare local search folders:

```json
{
  "sources": {
    "wallhaven": {
      "enabled": true,
      "categories": "111",
      "purity": "100"
    },
    "reddit": {
      "enabled": true,
      "subreddits": [
        "wallpaper",
        "wallpapers",
        "Cyberpunk",
        "EarthPorn",
        "AnimeWallpapers",
        "unixporn"
      ]
    },
    "waifuim": { "enabled": true },
    "picsum": { "enabled": true },
    "nasa": { "enabled": true },
    "bing": { "enabled": true },
    "nekos": { "enabled": true }
  },
  "local_folders": [
    "/home/username/Pictures"
  ],
  "tui": {
    "max_results": 12,
    "preload_thumbnails": true
  },
  "default_purity": "sfw",
  "apply_backend": "auto"
}
```

## Usage

```bash
# Auto-detect theme, query all sources concurrently, and download a matching wallpaper
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

## TUI Interactive Keybinds

When running in interactive selector mode (`--select`), use the following cyberpunk keyboard mapping:

- `j` / `k` or `Arrow Up/Down`: Navigate candidate list.
- `Space`: Toggle selection state for downloading.
- `Enter`: Confirm selection, copy/download, warm cache, and quit.
- `c` / `C`: Open Floating Cyberpunk Aesthetic Picker overlay menu.
- `r` / `R`: Refresh / Re-query all sources concurrently.
- `q` / `Esc`: Quit / Cancel.

## How it Works

1. Dynamically reads GTK theme configurations and parses active window border hex codes.
2. Strips accents/diacritics and matches corresponding HyDE native theme directories accent-insensitively.
3. Queries primary monitor resolution using `hyprctl monitors -j`.
4. Concurrently queries all enabled online APIs alongside configured local folders using 3-tiered fallback algorithms.
5. Shuffles candidates and spawns the terminal-raw Cyberpunk TUI selector.
6. Local previews skip network activity and decode directly from disk, while online previews fetch buffered byte slices.
7. Saves wallpapers as branded filenames in `~/.config/hyde/themes/<Theme Name>/wallpapers/`.
8. Generates SHA1 hashes to pre-warm HyDE's native thumbnail cache.
9. Applies the selection instantly via native active HyDE backends if `--apply` is supplied.
