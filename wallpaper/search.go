package wallpaper

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ColorRGB struct {
	Name string
	R, G, B int
}

var standardColors = []ColorRGB{
	{"red", 255, 0, 0},
	{"orange", 255, 165, 0},
	{"yellow", 255, 255, 0},
	{"green", 0, 128, 0},
	{"blue", 0, 0, 255},
	{"purple", 128, 0, 128},
	{"pink", 255, 192, 203},
	{"cyan", 0, 255, 255},
	{"teal", 0, 128, 128},
	{"white", 255, 255, 255},
	{"black", 0, 0, 0},
	{"gray", 128, 128, 128},
}

// HexToColorName translates a 6-character hex string into the closest standard English color name.
func HexToColorName(hexStr string) string {
	hexStr = strings.TrimPrefix(hexStr, "#")
	if len(hexStr) != 6 {
		return ""
	}

	rVal, _ := strconv.ParseInt(hexStr[0:2], 16, 64)
	gVal, _ := strconv.ParseInt(hexStr[2:4], 16, 64)
	bVal, _ := strconv.ParseInt(hexStr[4:6], 16, 64)
	r, g, b := int(rVal), int(gVal), int(bVal)

	bestName := "blue"
	minDist := math.MaxFloat64

	for _, c := range standardColors {
		dist := math.Sqrt(math.Pow(float64(r-c.R), 2) + math.Pow(float64(g-c.G), 2) + math.Pow(float64(b-c.B), 2))
		if dist < minDist {
			minDist = dist
			bestName = c.Name
		}
	}

	return bestName
}

// SearchAllSources queries all seven online wallpaper backends concurrently with a 3-tiered fallback strategy.
func SearchAllSources(query string, colorHex string, categories string, purity string, width, height int, count int) ([]ImageData, error) {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = GetDefaultConfig()
	}

	colorName := ""
	if colorHex != "" {
		colorName = HexToColorName(colorHex)
	}

	// We want up to 24 candidates total from search, then we shuffle and return count (usually 10-15)
	fetchPerSource := count
	if fetchPerSource < 5 {
		fetchPerSource = 5
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allCandidates []ImageData

	// Helper to add results safely
	addResults := func(res []ImageData) {
		if len(res) == 0 {
			return
		}
		mu.Lock()
		allCandidates = append(allCandidates, res...)
		mu.Unlock()
	}

	// Source 1: Wallhaven (with 3-tiered color-aesthetic fallback matching)
	if cfg.Sources.Wallhaven.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := SearchWallpapers(query, colorHex, categories, purity, width, height, fetchPerSource)
			if err != nil || len(res) == 0 {
				// Tier 2: Try color alone
				if colorHex != "" {
					res, err = SearchWallpapers("", colorHex, categories, purity, width, height, fetchPerSource)
				}
				// Tier 3: Try query alone
				if (err != nil || len(res) == 0) && query != "" {
					res, _ = SearchWallpapers(query, "", categories, purity, width, height, fetchPerSource)
				}
			}
			addResults(res)
		}()
	}

	// Source 2: Reddit (with 3-tiered color-aesthetic fallback matching)
	if cfg.Sources.Reddit.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Tier 1: Combined search (e.g. "Tokyo Night purple")
			combinedQuery := query
			if colorName != "" {
				combinedQuery = fmt.Sprintf("%s %s", query, colorName)
			}
			res, err := SearchReddit(combinedQuery, purity, fetchPerSource)
			if err != nil || len(res) == 0 {
				// Tier 2: Try color-only search (e.g. "purple wallpaper")
				if colorName != "" {
					res, err = SearchReddit(fmt.Sprintf("%s wallpaper", colorName), purity, fetchPerSource)
				}
				// Tier 3: Try query-only search (e.g. "Tokyo Night")
				if (err != nil || len(res) == 0) && query != "" {
					res, _ = SearchReddit(query, purity, fetchPerSource)
				}
			}
			addResults(res)
		}()
	}

	// Source 3: Waifu.im (Anime API)
	if cfg.Sources.Waifuim.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Try matching tag to query or color
			tagQuery := query
			if tagQuery == "" {
				tagQuery = colorName
			}
			res, _ := SearchWaifuIm(tagQuery, purity, fetchPerSource)
			addResults(res)
		}()
	}

	// Source 4: Picsum Photos (Premium photography)
	if cfg.Sources.Picsum.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, _ := SearchPicsum(fetchPerSource, width, height)
			addResults(res)
		}()
	}

	// Source 5: NASA APOD (Cosmic space wallpapers)
	if cfg.Sources.Nasa.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, _ := SearchNasaApod(fetchPerSource)
			addResults(res)
		}()
	}

	// Source 6: Bing Daily (Nature daily homepage feed)
	if cfg.Sources.Bing.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, _ := SearchBingDaily(fetchPerSource)
			addResults(res)
		}()
	}

	// Source 7: Nekos.life (Desktop anime wallpapers)
	if cfg.Sources.Nekos.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, _ := SearchNekosLife(fetchPerSource)
			addResults(res)
		}()
	}

	// Source 8: Local Folders (Crawler matched by query/aesthetic)
	if len(cfg.LocalFolders) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, _ := SearchLocalFolders(query, cfg.LocalFolders, fetchPerSource)
			if len(res) == 0 && colorName != "" {
				res, _ = SearchLocalFolders(colorName, cfg.LocalFolders, fetchPerSource)
			}
			addResults(res)
		}()
	}

	wg.Wait()

	if len(allCandidates) == 0 {
		return nil, fmt.Errorf("no wallpapers found across any of the online sources")
	}

	// Filter internal duplicates in aggregated list (by checking path/URL or ID)
	seen := make(map[string]bool)
	var uniqueCandidates []ImageData
	for _, cand := range allCandidates {
		// If ID represents a full hash or similar, convert it cleanly
		h := sha1.New()
		h.Write([]byte(cand.Path))
		pathHash := hex.EncodeToString(h.Sum(nil))[:8]

		if seen[pathHash] {
			continue
		}
		seen[pathHash] = true

		// Rewrite ID to incorporate the source for clear downstream checks
		cand.ID = fmt.Sprintf("%s-%s", strings.ToLower(strings.ReplaceAll(cand.Source, " ", "")), cand.ID)
		uniqueCandidates = append(uniqueCandidates, cand)
	}

	// Shuffle elements to blend sources beautifully
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(uniqueCandidates), func(i, j int) {
		uniqueCandidates[i], uniqueCandidates[j] = uniqueCandidates[j], uniqueCandidates[i]
	})

	// Adjust count to fit standard selector viewport
	maxResults := count
	if maxResults < 10 {
		maxResults = 10
	}
	if maxResults > 15 {
		maxResults = 15
	}
	if len(uniqueCandidates) > maxResults {
		uniqueCandidates = uniqueCandidates[:maxResults]
	}

	return uniqueCandidates, nil
}
