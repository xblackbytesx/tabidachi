package images

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// Download fetches a remote image and saves it to uploadsDir/{uuid}.jpg.
// Returns the relative web path (e.g. "uploads/{uuid}.jpg").
func Download(ctx context.Context, remoteURL, uploadsDir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return "", fmt.Errorf("download image request: %w", err)
	}
	req.Header.Set("User-Agent", "Hakken/1.0 (itinerary manager)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download image: status %d", resp.StatusCode)
	}

	filename := uuid.New().String() + ".jpg"
	fullPath := filepath.Join(uploadsDir, filename)

	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return "", fmt.Errorf("create uploads dir: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("create upload file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write upload file: %w", err)
	}

	return "uploads/" + filename, nil
}
