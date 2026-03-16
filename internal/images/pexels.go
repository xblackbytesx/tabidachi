package images

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// PexelsProvider fetches images from the Pexels API.
type PexelsProvider struct {
	apiKey string
	client *http.Client
}

// NewPexelsProvider returns nil if apiKey is empty (provider skipped).
// Returns the Provider interface so a nil return is a true nil interface, not
// a typed nil (*PexelsProvider)(nil) wrapped in a non-nil interface value.
func NewPexelsProvider(apiKey string) Provider {
	if apiKey == "" {
		return nil
	}
	return &PexelsProvider{apiKey: apiKey, client: &http.Client{}}
}

func (p *PexelsProvider) Name() string { return "pexels" }

type pexelsResponse struct {
	Photos []struct {
		URL  string `json:"url"`
		Src  struct {
			Landscape string `json:"landscape"`
			Medium    string `json:"medium"`
		} `json:"src"`
		Photographer string `json:"photographer"`
	} `json:"photos"`
}

func (p *PexelsProvider) doSearch(ctx context.Context, query string, perPage int) (*pexelsResponse, error) {
	reqURL := "https://api.pexels.com/v1/search?orientation=landscape&per_page=" +
		fmt.Sprintf("%d", perPage) + "&query=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", p.apiKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pexels: status %d", resp.StatusCode)
	}
	var result pexelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (p *PexelsProvider) Fetch(ctx context.Context, query string) (*ImageResult, error) {
	result, err := p.doSearch(ctx, query, 1)
	if err != nil {
		return nil, err
	}
	if len(result.Photos) == 0 {
		return nil, nil
	}
	ph := result.Photos[0]
	imgURL := ph.Src.Landscape
	if imgURL == "" {
		imgURL = ph.URL
	}
	return &ImageResult{
		URL:    imgURL,
		Credit: "Photo by " + ph.Photographer + " on Pexels",
		Query:  query,
	}, nil
}

func (p *PexelsProvider) Search(ctx context.Context, query string) ([]ImageResult, error) {
	result, err := p.doSearch(ctx, query, 9)
	if err != nil {
		return nil, err
	}
	var out []ImageResult
	for _, ph := range result.Photos {
		imgURL := ph.Src.Medium
		if imgURL == "" {
			imgURL = ph.Src.Landscape
		}
		out = append(out, ImageResult{
			URL:    imgURL,
			Credit: "Photo by " + ph.Photographer + " on Pexels",
			Query:  query,
		})
	}
	return out, nil
}
