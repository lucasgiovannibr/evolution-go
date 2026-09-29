package utils

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Size limits for what is fetched from a user-supplied URL.
const (
	// MaxMediaDownload is the largest media file taken from a URL. WhatsApp itself
	// stops accepting media around this size.
	MaxMediaDownload int64 = 100 << 20
	// MaxImageDownload is for images that are re-encoded or set as a picture.
	MaxImageDownload int64 = 20 << 20
	// MaxThumbnailDownload is for link-preview thumbnails.
	MaxThumbnailDownload int64 = 2 << 20
)

// ErrDownloadTooLarge is returned when the body exceeds the limit.
var ErrDownloadTooLarge = errors.New("file is too large")

// DownloadBytes fetches a URL with DownloadClient and returns its body.
//
// Callers used to do http.Get + io.ReadAll, which had three gaps: an error page (404,
// 500) was taken for the file and sent as a document, a huge or endless body was read
// into memory, and nothing said what went wrong. A non-2xx answer is now an error and
// the body is bounded by maxBytes.
func DownloadBytes(rawURL string, maxBytes int64) ([]byte, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("invalid URL: only http and https are supported")
	}

	resp, err := DownloadClient.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("failed to fetch %s: HTTP status %d", rawURL, resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return nil, fmt.Errorf("%w: %d bytes (limit %d)", ErrDownloadTooLarge, resp.ContentLength, maxBytes)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", rawURL, err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: more than %d bytes", ErrDownloadTooLarge, maxBytes)
	}
	return data, nil
}

// compile-time check that the client used above is the shared one
var _ *http.Client = DownloadClient
