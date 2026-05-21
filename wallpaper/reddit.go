package wallpaper

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type RedditListing struct {
	Data struct {
		Children []struct {
			Data struct {
				ID        string `json:"id"`
				Title     string `json:"title"`
				URL       string `json:"url"`
				PostHint  string `json:"post_hint"`
				Over18    bool   `json:"over_18"`
				Preview   struct {
					Images []struct {
						Source struct {
							URL    string `json:"url"`
							Width  int    `json:"width"`
							Height int    `json:"height"`
						} `json:"source"`
						Resolutions []struct {
							URL    string `json:"url"`
							Width  int    `json:"width"`
							Height int    `json:"height"`
						} `json:"resolutions"`
					} `json:"images"`
				} `json:"preview"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

// SearchReddit queries multiple popular wallpaper subreddits keylessly and returns candidates.
func SearchReddit(query string, purity string, count int) ([]ImageData, error) {
	if query == "" {
		return nil, nil
	}

	// Restrict to these high-quality image subreddits
	subreddits := "wallpaper+wallpapers+Cyberpunk+EarthPorn+AnimeWallpapers+unixporn"
	baseURL := fmt.Sprintf("https://www.reddit.com/r/%s/search.json", subreddits)

	params := url.Values{}
	params.Add("q", query)
	params.Add("restrict_sr", "1")
	params.Add("limit", "25")
	params.Add("sort", "relevance")

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Reddit request: %w", err)
	}

	// Custom User-Agent to avoid Reddit HTTP 429 rate limit block
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact Reddit API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Reddit API returned status: %s", resp.Status)
	}

	var listing RedditListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, fmt.Errorf("failed to parse Reddit response: %w", err)
	}

	// Check if NSFW is allowed (third bit of purityVal mask, e.g. "001" or "111")
	allowNSFW := false
	if len(purity) == 3 && purity[2] == '1' {
		allowNSFW = true
	}

	var results []ImageData
	for _, child := range listing.Data.Children {
		data := child.Data
		if data.ID == "" {
			continue
		}

		// Skip NSFW if disallowed
		if data.Over18 && !allowNSFW {
			continue
		}

		// Resolve direct image URL
		imgURL := data.URL
		if !isDirectImage(imgURL) {
			if len(data.Preview.Images) > 0 {
				imgURL = data.Preview.Images[0].Source.URL
			} else {
				// Skip if we can't find a direct image link
				continue
			}
		}

		// Clean XML HTML entity escaping inside Reddit URLs
		imgURL = strings.ReplaceAll(imgURL, "&amp;", "&")

		// Extract resolution and aspect ratio
		width, height := 1920, 1080 // Standard default fallback
		if len(data.Preview.Images) > 0 {
			src := data.Preview.Images[0].Source
			if src.Width > 0 && src.Height > 0 {
				width = src.Width
				height = src.Height
			}
		}

		resolution := fmt.Sprintf("%dx%d", width, height)
		ratio := calculateRatio(width, height)

		// Choose the best thumbnail preview closest to 300px width
		thumbURL := imgURL
		if len(data.Preview.Images) > 0 && len(data.Preview.Images[0].Resolutions) > 0 {
			resolutions := data.Preview.Images[0].Resolutions
			bestIdx := 0
			minDiff := 999999
			for i, res := range resolutions {
				diff := res.Width - 300
				if diff < 0 {
					diff = -diff
				}
				if diff < minDiff {
					minDiff = diff
					bestIdx = i
				}
			}
			thumbURL = strings.ReplaceAll(resolutions[bestIdx].URL, "&amp;", "&")
		}

		results = append(results, ImageData{
			ID:         data.ID,
			Path:       imgURL,
			Resolution: resolution,
			Ratio:      ratio,
			Thumbs: ThumbsData{
				Large: thumbURL,
				Small: thumbURL,
			},
			Source: "Reddit",
		})

		if len(results) >= count {
			break
		}
	}

	return results, nil
}

func isDirectImage(urlStr string) bool {
	lower := strings.ToLower(urlStr)
	return strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".png")
}

func calculateRatio(width, height int) string {
	if height == 0 {
		return "16:9"
	}
	r := float64(width) / float64(height)
	if r > 2.3 {
		return "21:9"
	} else if r > 1.7 {
		return "16:9"
	} else if r > 1.5 {
		return "16:10"
	} else if r > 1.3 {
		return "4:3"
	}
	return "5:4"
}
