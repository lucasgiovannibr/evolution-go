package send_handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	send_service "github.com/evolution-foundation/evolution-go/pkg/sendMessage/service"
	"github.com/gin-gonic/gin"
)

func respond(err error) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	respondSendError(c, err)
	return w
}

func TestRespondSendErrorThrottledIs429WithRetryAfter(t *testing.T) {
	w := respond(fmt.Errorf("wrapped: %w", &send_service.ErrSendThrottled{RetryAfter: 2500 * time.Millisecond}))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "3" {
		t.Fatalf("Retry-After %q, want 3", got)
	}
}

func TestRespondSendErrorOthersStay500(t *testing.T) {
	w := respond(errors.New("boom"))
	if w.Code != http.StatusInternalServerError || w.Header().Get("Retry-After") != "" {
		t.Fatalf("status %d, headers %v", w.Code, w.Header())
	}
}
