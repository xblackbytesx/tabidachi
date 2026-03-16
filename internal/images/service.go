package images

import (
	"context"
	"fmt"
)

// Service combines a provider chain with local file storage.
type Service struct {
	fetcher    *ChainFetcher
	uploadsDir string
}

// NewService creates a Service from config values.
// pexelsKey and unsplashKey may be empty; Wikipedia is always available.
func NewService(pexelsKey, unsplashKey, uploadsDir string) *Service {
	return &Service{
		fetcher: NewChainFetcher(
			NewPexelsProvider(pexelsKey),
			NewUnsplashProvider(unsplashKey),
			NewWikipediaProvider(),
		),
		uploadsDir: uploadsDir,
	}
}

// FetchAndStore picks an image for the query, downloads it locally, and
// returns the relative web path and credit string.
func (s *Service) FetchAndStore(ctx context.Context, query string) (localURL, credit string, err error) {
	res, err := s.fetcher.Fetch(ctx, query)
	if err != nil {
		return "", "", fmt.Errorf("fetch image for %q: %w", query, err)
	}
	if res == nil {
		return "", "", fmt.Errorf("no image found for %q", query)
	}
	path, err := Download(ctx, res.URL, s.uploadsDir)
	if err != nil {
		return "", "", fmt.Errorf("download image for %q: %w", query, err)
	}
	return "/" + path, res.Credit, nil
}

// Search returns provider thumbnail URLs (not downloaded) for manual selection.
func (s *Service) Search(ctx context.Context, query string) ([]ImageResult, error) {
	return s.fetcher.Search(ctx, query)
}

// UploadsDir returns the configured uploads directory path.
func (s *Service) UploadsDir() string { return s.uploadsDir }
