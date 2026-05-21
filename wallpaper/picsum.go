package wallpaper

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

type PicsumImage struct {
	ID          string `json:"id"`
	Author      string `json:"author"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	URL         string `json:"url"`
	DownloadURL string `json:"download_url"`
}

// SearchPicsum fetches high-resolution, perfectly cropped landscape photography from Picsum Photos.
func SearchPicsum(count int, width, height int) ([]ImageData, error) {
	// Query the list of high-quality photography
	fullURL := "https://picsum.photos/v2/list?limit=50"

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to contact Picsum API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Picsum API returned status: %s", resp.Status)
	}

	var images []PicsumImage
	if err := json.NewDecoder(resp.Body).Decode(&images); err != nil {
		return nil, fmt.Errorf("failed to parse Picsum response: %w", err)
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("Picsum API returned an empty list")
	}

	// Shuffle elements to get unique wallpapers on each refresh
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(images), func(i, j int) {
		images[i], images[j] = images[j], images[i]
	})

	if count > len(images) {
		count = len(images)
	}

	w := width
	h := height
	if w <= 0 || h <= 0 {
		w, h = 1920, 1080
	}

	var results []ImageData
	for i := 0; i < count; i++ {
		img := images[i]

		// Build a mathematically perfect custom-sized crop for the user's primary monitor resolution!
		destURL := fmt.Sprintf("https://picsum.photos/id/%s/%d/%d", img.ID, w, h)
		// Small fast thumbnail crop
		thumbURL := fmt.Sprintf("https://picsum.photos/id/%s/300/180", img.ID)

		resolution := fmt.Sprintf("%dx%d", w, h)
		ratio := calculateRatio(w, h)

		results = append(results, ImageData{
			ID:         img.ID,
			Path:       destURL,
			Resolution: resolution,
			Ratio:      ratio,
			Thumbs: ThumbsData{
				Large: thumbURL,
				Small: thumbURL,
			},
			Source: "Picsum",
		})
	}

	return results, nil
}
