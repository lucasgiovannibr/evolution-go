package webhook_producer

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
)

func newTestProducer(t *testing.T, timeout time.Duration) *webhookProducer {
	t.Helper()
	cfg := &config.Config{LogDirectory: t.TempDir()}
	return &webhookProducer{
		loggerWrapper: logger_wrapper.NewLoggerManager(cfg),
		httpClient:    &http.Client{Timeout: timeout},
	}
}

// A receiver that never answers must not hold the delivery forever.
func TestSendWebhookGivesUpOnAHangingReceiver(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)

	p := newTestProducer(t, 200*time.Millisecond)
	start := time.Now()
	err, _, _ := p.sendWebhook(srv.URL, []byte(`{}`), "u")
	if err == nil {
		t.Fatal("a hanging receiver must produce an error")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("took %v, the timeout was not applied", elapsed)
	}
}

// Only a bounded part of the answer is read.
func TestSendWebhookLimitsTheResponseRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 10*maxLoggedResponse)))
	}))
	defer srv.Close()

	p := newTestProducer(t, 5*time.Second)
	err, body, status := p.sendWebhook(srv.URL, []byte(`{}`), "u")
	if err != nil || status != 200 {
		t.Fatalf("err=%v status=%d", err, status)
	}
	if len(body) != maxLoggedResponse {
		t.Fatalf("read %d bytes, want %d", len(body), maxLoggedResponse)
	}
}

// A failing receiver is retried, and there is no wait after the last attempt.
func TestSendWebhookWithRetryDoesNotSleepAfterTheLastAttempt(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newTestProducer(t, 5*time.Second)
	start := time.Now()
	p.sendWebhookWithRetry(srv.URL, []byte(`{}`), 3, 150*time.Millisecond, "u")
	elapsed := time.Since(start)

	if hits.Load() != 3 {
		t.Fatalf("hits = %d, want 3", hits.Load())
	}
	// two waits between three attempts (300ms), not three
	if elapsed < 250*time.Millisecond || elapsed > 420*time.Millisecond {
		t.Fatalf("elapsed %v, want about 300ms", elapsed)
	}
}
