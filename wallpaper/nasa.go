package wallpaper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type NasaApodImage struct {
	Date      string `json:"date"`
	Title     string `json:"title"`
	MediaType string `json:"media_type"`
	URL       string `json:"url"`
	HDURL     string `json:"hdurl"`
}

// SearchNasaApod queries the NASA Astronomy Picture of the Day API using DEMO_KEY.
func SearchNasaApod(count int) ([]ImageData, error) {
	baseURL := "https://api.nasa.gov/planetary/apod"

	params := url.Values{}
	params.Add("api_key", "DEMO_KEY")
	params.Add("count", strconv.Itoa(count*2)) // Request slightly more in case some aren't images (e.g. videos)

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to contact NASA APOD API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NASA APOD API returned status: %s", resp.Status)
	}

	var items []NasaApodImage
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to parse NASA APOD response: %w", err)
	}

	var results []ImageData
	for _, item := range items {
		// Only retrieve actual image types (ignore videos, animations, or interactive pages)
		if item.MediaType != "image" {
			continue
		}

		imgURL := item.HDURL
		if imgURL == "" {
			imgURL = item.URL
		}

		if imgURL == "" {
			continue
		}

		// Cleanup date string to build a sleek identifier
		cleanID := strings.ReplaceAll(item.Date, "-", "")

		// Thumbs
		thumbURL := item.URL
		if thumbURL == "" {
			thumbURL = imgURL
		}

		results = append(results, ImageData{
			ID:         cleanID,
			Path:       imgURL,
			Resolution: "3840x2160", // Cosmic wallpapers are typically 4K or high resolution
			Ratio:      "16:9",
			Thumbs: ThumbsData{
				Large: thumbURL,
				Small: thumbURL,
			},
			Source: "NASA APOD",
		})

		if len(results) >= count {
			break
		}
	}

	return results, nil
}
