package wallpaper

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

type WallhavenResponse struct {
	Data []ImageData `json:"data"`
	Meta MetaData    `json:"meta"`
}

type ThumbsData struct {
	Large string `json:"large"`
	Small string `json:"small"`
}

type ImageData struct {
	ID         string     `json:"id"`
	Path       string     `json:"path"`
	Resolution string     `json:"resolution"`
	Ratio      string     `json:"ratio"`
	Thumbs     ThumbsData `json:"thumbs"`
}

type MetaData struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	Total       int `json:"total"`
}

// SearchWallpapers searches Wallhaven for wallpapers matching the theme, resolution, colors, categories, and purity.
func SearchWallpapers(query string, color string, categories string, purity string, width, height int, count int) ([]ImageData, error) {
	baseURL := "https://wallhaven.cc/api/v1/search"
	res := fmt.Sprintf("%dx%d", width, height)

	params := url.Values{}
	if query != "" {
		params.Add("q", query)
	}
	params.Add("resolutions", res)

	if color != "" {
		params.Add("colors", color)
	}
	if categories != "" {
		params.Add("categories", categories)
	}
	if purity != "" {
		params.Add("purity", purity)
	}

	params.Add("sorting", "random") // Get random results matching our criteria

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search wallhaven: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wallhaven API returned status: %s", resp.Status)
	}

	var wallResp WallhavenResponse
	if err := json.NewDecoder(resp.Body).Decode(&wallResp); err != nil {
		return nil, fmt.Errorf("failed to decode wallhaven response: %w", err)
	}

	if len(wallResp.Data) == 0 {
		return nil, fmt.Errorf("no wallpapers found matching query parameters")
	}

	// Pick N random ones from the results (simple shuffle/slice approach)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(wallResp.Data), func(i, j int) {
		wallResp.Data[i], wallResp.Data[j] = wallResp.Data[j], wallResp.Data[i]
	})

	// Adjust count if we have fewer results than requested
	if count > len(wallResp.Data) {
		count = len(wallResp.Data)
	}

	return wallResp.Data[:count], nil
}
