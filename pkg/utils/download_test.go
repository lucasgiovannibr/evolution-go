package utils

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadBytesReturnsTheBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hello")) }))
	defer srv.Close()

	data, err := DownloadBytes(srv.URL, 1024)
	if err != nil || string(data) != "hello" {
		t.Fatalf("%q %v", data, err)
	}
}

// An error page must not be taken for the file.
func TestDownloadBytesRejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<html>not found</html>"))
	}))
	defer srv.Close()

	data, err := DownloadBytes(srv.URL, 1024)
	if err == nil || data != nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("data=%q err=%v", data, err)
	}
}

func TestDownloadBytesEnforcesTheLimit(t *testing.T) {
	// declared length (Content-Length) over the limit
	declared := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer declared.Close()
	if _, err := DownloadBytes(declared.URL, 10); !errors.Is(err, ErrDownloadTooLarge) {
		t.Fatalf("declared: %v", err)
	}

	// chunked body with no length, over the limit
	chunked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := w.(http.Flusher)
		for i := 0; i < 5; i++ {
			_, _ = w.Write([]byte(strings.Repeat("x", 10)))
			f.Flush()
		}
	}))
	defer chunked.Close()
	if _, err := DownloadBytes(chunked.URL, 20); !errors.Is(err, ErrDownloadTooLarge) {
		t.Fatalf("chunked: %v", err)
	}

	// exactly at the limit is fine
	exact := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 10)))
	}))
	defer exact.Close()
	if data, err := DownloadBytes(exact.URL, 10); err != nil || len(data) != 10 {
		t.Fatalf("exact: %d %v", len(data), err)
	}
}

func TestDownloadBytesOnlyHTTP(t *testing.T) {
	for _, u := range []string{"", "ftp://x/y", "file:///etc/passwd", "gopher://x"} {
		if _, err := DownloadBytes(u, 10); err == nil {
			t.Errorf("%q must be rejected", u)
		}
	}
}
