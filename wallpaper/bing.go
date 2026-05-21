package wallpaper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type BingResponse struct {
	Images []struct {
		StartDate string `json:"startdate"`
		URL       string `json:"url"`
		URLBase   string `json:"urlbase"`
		Title     string `json:"title"`
	} `json:"images"`
}

// SearchBingDaily queries Bing's homepage archive feed keylessly and returns daily landscapes.
func SearchBingDaily(count int) ([]ImageData, error) {
	baseURL := "https://www.bing.com/HPImageArchive.aspx"

	params := url.Values{}
	params.Add("format", "js")
	params.Add("idx", "0")
	params.Add("n", strconv.Itoa(count))
	params.Add("mkt", "en-US")

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to contact Bing API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bing API returned status: %s", resp.Status)
	}

	var bingResp BingResponse
	if err := json.NewDecoder(resp.Body).Decode(&bingResp); err != nil {
		return nil, fmt.Errorf("failed to parse Bing response: %w", err)
	}

	var results []ImageData
	for _, img := range bingResp.Images {
		if img.URL == "" {
			continue
		}

		// Construct high-definition UHD 4K (3840x2160) wallpaper link if possible,
		// otherwise fall back to standard 1920x1080 URL
		imgURL := "https://www.bing.com" + img.URL
		if img.URLBase != "" {
			imgURL = "https://www.bing.com" + img.URLBase + "_UHD.jpg"
		}

		// Small fast thumbnail crop
		thumbURL := "https://www.bing.com" + img.URL

		results = append(results, ImageData{
			ID:         img.StartDate,
			Path:       imgURL,
			Resolution: "3840x2160",
			Ratio:      "16:9",
			Thumbs: ThumbsData{
				Large: thumbURL,
				Small: thumbURL,
			},
			Source: "Bing Daily",
		})
	}

	return results, nil
}
