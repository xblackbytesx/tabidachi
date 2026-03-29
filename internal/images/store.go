package images

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// maxImageBytes is the maximum size allowed for a downloaded image (20 MB).
const maxImageBytes = 20 * 1024 * 1024

// httpClient is a shared HTTP client with a sensible timeout for all image operations.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// Download fetches a remote image and saves it to uploadsDir/{uuid}.jpg.
// Returns the relative web path (e.g. "uploads/{uuid}.jpg").
func Download(ctx context.Context, remoteURL, uploadsDir string) (relPath string, err error) {
	if !strings.HasPrefix(remoteURL, "https://") && !strings.HasPrefix(remoteURL, "http://") {
		return "", fmt.Errorf("download image: unsupported URL scheme")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return "", fmt.Errorf("download image request: %w", err)
	}
	req.Header.Set("User-Agent", "Tabidachi/1.0 (itinerary manager)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download image: status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxImageBytes {
		return "", fmt.Errorf("download image: response too large (%d bytes)", resp.ContentLength)
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
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(fullPath)
		}
	}()

	// Limit reads to maxImageBytes+1 so we can detect oversized responses even
	// when Content-Length is absent or lies.
	lr := io.LimitReader(resp.Body, maxImageBytes+1)
	written, err := io.Copy(f, lr)
	if err != nil {
		return "", fmt.Errorf("write upload file: %w", err)
	}
	if written > maxImageBytes {
		return "", fmt.Errorf("download image: content exceeds %d MB limit", maxImageBytes/1024/1024)
	}

	// Validate that the downloaded content is actually an image.
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek upload file: %w", err)
	}
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	ct := http.DetectContentType(buf[:n])
	switch ct {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		// accepted
	default:
		return "", fmt.Errorf("download image: unsupported content type %q", ct)
	}

	return "uploads/" + filename, nil
}
