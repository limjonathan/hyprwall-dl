package wallpaper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type WaifuResponse struct {
	Images []struct {
		ImageID int    `json:"image_id"`
		URL     string `json:"url"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
	} `json:"images"`
}

// SearchWaifuIm queries the waifu.im public API keylessly and returns anime wallpapers.
func SearchWaifuIm(query string, purity string, count int) ([]ImageData, error) {
	baseURL := "https://api.waifu.im/search"

	params := url.Values{}
	params.Add("limit", strconv.Itoa(count*2)) // Fetch slightly more to filter

	// Map purity flag SFW/NSFW
	allowNSFW := false
	if len(purity) == 3 && purity[2] == '1' {
		allowNSFW = true
	}

	if allowNSFW {
		// Omit is_nsfw to allow both SFW and NSFW contents
	} else {
		params.Add("is_nsfw", "false")
	}

	// Waifu.im doesn't support free-text keyword search, but does support tag mapping
	// If query matches standard themes or tags, we can request it
	hasTag := false
	supportedTags := []string{"waifu", "maid", "uniform", "oppai", "marin-kitagawa", "raiden-shogun", "mori-calliope"}
	for _, tag := range supportedTags {
		if tag == query {
			params.Add("included_tags", tag)
			hasTag = true
			break
		}
	}

	if !hasTag {
		// Default to generic high-quality versatile tag
		params.Add("included_tags", "versatile")
	}

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to contact Waifu.im API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Waifu.im API returned status: %s", resp.Status)
	}

	var waifuResp WaifuResponse
	if err := json.NewDecoder(resp.Body).Decode(&waifuResp); err != nil {
		return nil, fmt.Errorf("failed to parse Waifu.im response: %w", err)
	}

	var results []ImageData
	for _, img := range waifuResp.Images {
		idStr := strconv.Itoa(img.ImageID)
		w, h := img.Width, img.Height
		if w <= 0 || h <= 0 {
			w, h = 1920, 1080
		}

		resolution := fmt.Sprintf("%dx%d", w, h)
		ratio := calculateRatio(w, h)

		results = append(results, ImageData{
			ID:         idStr,
			Path:       img.URL,
			Resolution: resolution,
			Ratio:      ratio,
			Thumbs: ThumbsData{
				Large: img.URL, // High speed CDN supports direct thumbs
				Small: img.URL,
			},
			Source: "Waifu.im",
		})

		if len(results) >= count {
			break
		}
	}

	return results, nil
}
