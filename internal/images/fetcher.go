package images

import "context"

// ImageResult holds the outcome of a provider search.
type ImageResult struct {
	URL      string // full-res URL — downloaded and stored locally
	ThumbURL string // smaller URL for picker grid display only (optional; falls back to URL)
	Credit   string
	Query    string
}

// DisplayURL returns ThumbURL if set, otherwise URL. Used for picker thumbnails.
func (r ImageResult) DisplayURL() string {
	if r.ThumbURL != "" {
		return r.ThumbURL
	}
	return r.URL
}

// Provider is implemented by each image source.
type Provider interface {
	Name() string
	Fetch(ctx context.Context, query string) (*ImageResult, error)
	// Search returns multiple results (thumbnails) for manual selection.
	Search(ctx context.Context, query string) ([]ImageResult, error)
}

// ChainFetcher tries providers in order, returning the first success.
type ChainFetcher struct {
	providers []Provider
}

// NewChainFetcher creates a ChainFetcher from the given providers.
// Providers are tried in the order given; nil providers are skipped.
func NewChainFetcher(providers ...Provider) *ChainFetcher {
	var active []Provider
	for _, p := range providers {
		if p != nil {
			active = append(active, p)
		}
	}
	return &ChainFetcher{providers: active}
}

// Fetch tries each provider in order and returns the first success.
func (c *ChainFetcher) Fetch(ctx context.Context, query string) (*ImageResult, error) {
	var lastErr error
	for _, p := range c.providers {
		res, err := p.Fetch(ctx, query)
		if err == nil && res != nil {
			return res, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}

// Search returns results from the first provider that succeeds.
func (c *ChainFetcher) Search(ctx context.Context, query string) ([]ImageResult, error) {
	for _, p := range c.providers {
		results, err := p.Search(ctx, query)
		if err == nil && len(results) > 0 {
			return results, nil
		}
	}
	return nil, nil
}
