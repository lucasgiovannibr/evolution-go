package user_handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	user_service "github.com/evolution-foundation/evolution-go/pkg/user/service"
	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
)

func TestWriteUserWAError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "rate overlimit",
			err:        fmt.Errorf("usync: %w", whatsmeow.ErrIQRateOverLimit),
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "iq timeout",
			err:        fmt.Errorf("avatar: %w", whatsmeow.ErrIQTimedOut),
			wantStatus: http.StatusGatewayTimeout,
		},
		{
			name:       "context deadline",
			err:        fmt.Errorf("avatar: %w", context.DeadlineExceeded),
			wantStatus: http.StatusGatewayTimeout,
		},
		{
			name:       "context canceled",
			err:        fmt.Errorf("avatar: %w", context.Canceled),
			wantStatus: http.StatusGatewayTimeout,
		},
		{
			name:       "invalid number",
			err:        fmt.Errorf("check: %w", &user_service.InvalidNumberError{Err: errors.New("invalid number format")}),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "other error",
			err:        errors.New("no profile picture found"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			writeUserWAError(c, tt.err)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			if body["error"] == "" {
				t.Fatal("expected non-empty error field")
			}
		})
	}
}
