package images

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// WikipediaProvider fetches page thumbnail images from Wikipedia.
// No API key required — used as a zero-config fallback.
type WikipediaProvider struct {
	client *http.Client
}

func NewWikipediaProvider() *WikipediaProvider {
	return &WikipediaProvider{client: &http.Client{}}
}

func (p *WikipediaProvider) Name() string { return "wikipedia" }

type wikiSummary struct {
	Thumbnail struct {
		Source string `json:"source"`
	} `json:"thumbnail"`
	Title string `json:"title"`
}

// fetchSummary fetches the Wikipedia page summary for an exact title.
func (p *WikipediaProvider) fetchSummary(ctx context.Context, title string) (*wikiSummary, error) {
	encoded := strings.ReplaceAll(title, " ", "_")
	reqURL := "https://en.wikipedia.org/api/rest_v1/page/summary/" + url.PathEscape(encoded)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Hakken/1.0 (itinerary manager)")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wikipedia: status %d for %q", resp.StatusCode, title)
	}
	var summary wikiSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

// search uses Wikipedia's opensearch API to find the best-matching article title.
// Returns up to n titles for the query.
func (p *WikipediaProvider) search(ctx context.Context, query string, n int) ([]string, error) {
	reqURL := "https://en.wikipedia.org/w/api.php?action=opensearch&format=json&limit=" +
		fmt.Sprintf("%d", n) + "&search=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Hakken/1.0 (itinerary manager)")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wikipedia opensearch: status %d", resp.StatusCode)
	}
	// opensearch returns [query, [titles], [descriptions], [urls]]
	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if len(raw) < 2 {
		return nil, nil
	}
	var titles []string
	if err := json.Unmarshal(raw[1], &titles); err != nil {
		return nil, err
	}
	return titles, nil
}

// Fetch finds the best Wikipedia article for the query and returns its thumbnail.
// First tries an exact title lookup; if that fails or has no image, falls back
// to opensearch to find the closest article.
func (p *WikipediaProvider) Fetch(ctx context.Context, query string) (*ImageResult, error) {
	// Try exact title first (fast path for city names like "Tokyo")
	if summary, err := p.fetchSummary(ctx, query); err == nil && summary.Thumbnail.Source != "" {
		return &ImageResult{
			URL:    summary.Thumbnail.Source,
			Credit: "Image from Wikipedia — " + summary.Title,
			Query:  query,
		}, nil
	}

	// Fall back to opensearch to find the closest article
	titles, err := p.search(ctx, query, 5)
	if err != nil || len(titles) == 0 {
		return nil, fmt.Errorf("wikipedia: no results for %q", query)
	}
	for _, title := range titles {
		summary, err := p.fetchSummary(ctx, title)
		if err != nil || summary.Thumbnail.Source == "" {
			continue
		}
		return &ImageResult{
			URL:    summary.Thumbnail.Source,
			Credit: "Image from Wikipedia — " + summary.Title,
			Query:  query,
		}, nil
	}
	return nil, fmt.Errorf("wikipedia: no thumbnail found for %q", query)
}

// Search returns up to 4 results: each opensearch hit that has a thumbnail image.
func (p *WikipediaProvider) Search(ctx context.Context, query string) ([]ImageResult, error) {
	titles, err := p.search(ctx, query, 9)
	if err != nil {
		return nil, err
	}

	var results []ImageResult
	for _, title := range titles {
		if len(results) >= 4 {
			break
		}
		summary, err := p.fetchSummary(ctx, title)
		if err != nil || summary.Thumbnail.Source == "" {
			continue
		}
		results = append(results, ImageResult{
			URL:    summary.Thumbnail.Source,
			Credit: "Image from Wikipedia — " + summary.Title,
			Query:  query,
		})
	}
	return results, nil
}
