package wallpaper

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type NekosResponse struct {
	URL string `json:"url"`
}

// SearchNekosLife queries the nekos.life public API concurrently to fetch desktop anime wallpapers.
func SearchNekosLife(count int) ([]ImageData, error) {
	var wg sync.WaitGroup
	resultChan := make(chan string, count)

	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get("https://nekos.life/api/v2/img/wallpaper")
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return
			}

			var nekosResp NekosResponse
			if err := json.NewDecoder(resp.Body).Decode(&nekosResp); err == nil && nekosResp.URL != "" {
				resultChan <- nekosResp.URL
			}
		}()
	}

	wg.Wait()
	close(resultChan)

	var results []ImageData
	for imgURL := range resultChan {
		// Calculate a unique short ID based on the URL hash to avoid duplicate collision
		h := sha1.New()
		h.Write([]byte(imgURL))
		hashStr := hex.EncodeToString(h.Sum(nil))[:8]

		results = append(results, ImageData{
			ID:         hashStr,
			Path:       imgURL,
			Resolution: "1920x1080", // Vector anime artwork displays perfectly up to 4K
			Ratio:      "16:9",
			Thumbs: ThumbsData{
				Large: imgURL,
				Small: imgURL,
			},
			Source: "Nekos",
		})
	}

	return results, nil
}
