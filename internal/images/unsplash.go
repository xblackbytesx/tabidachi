package images

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// UnsplashProvider fetches images from the Unsplash API.
type UnsplashProvider struct {
	accessKey string
	client    *http.Client
}

// NewUnsplashProvider returns nil if accessKey is empty.
// Returns the Provider interface so a nil return is a true nil interface.
func NewUnsplashProvider(accessKey string) Provider {
	if accessKey == "" {
		return nil
	}
	return &UnsplashProvider{accessKey: accessKey, client: &http.Client{}}
}

func (p *UnsplashProvider) Name() string { return "unsplash" }

type unsplashResponse struct {
	Results []struct {
		URLs struct {
			Regular string `json:"regular"`
			Small   string `json:"small"`
			Full    string `json:"full"`
		} `json:"urls"`
		User struct {
			Name string `json:"name"`
		} `json:"user"`
		Links struct {
			HTML string `json:"html"`
		} `json:"links"`
	} `json:"results"`
}

func (p *UnsplashProvider) doSearch(ctx context.Context, query string, perPage int) (*unsplashResponse, error) {
	reqURL := "https://api.unsplash.com/search/photos?orientation=landscape&per_page=" +
		fmt.Sprintf("%d", perPage) + "&query=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Client-ID "+p.accessKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unsplash: status %d", resp.StatusCode)
	}
	var result unsplashResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (p *UnsplashProvider) Fetch(ctx context.Context, query string) (*ImageResult, error) {
	result, err := p.doSearch(ctx, query, 1)
	if err != nil {
		return nil, err
	}
	if len(result.Results) == 0 {
		return nil, nil
	}
	photo := result.Results[0]
	return &ImageResult{
		URL:    photo.URLs.Regular,
		Credit: "Photo by " + photo.User.Name + " on Unsplash",
		Query:  query,
	}, nil
}

func (p *UnsplashProvider) Search(ctx context.Context, query string) ([]ImageResult, error) {
	result, err := p.doSearch(ctx, query, 9)
	if err != nil {
		return nil, err
	}
	var out []ImageResult
	for _, photo := range result.Results {
		out = append(out, ImageResult{
			URL:    photo.URLs.Small,
			Credit: "Photo by " + photo.User.Name + " on Unsplash",
			Query:  query,
		})
	}
	return out, nil
}
