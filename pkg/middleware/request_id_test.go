package auth_middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func serveWithRequestID(header string) (*httptest.ResponseRecorder, string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	var seen string
	r.GET("/x", func(c *gin.Context) { seen = c.GetString(RequestIDKey) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if header != "" {
		req.Header.Set(RequestIDHeader, header)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, seen
}

func TestRequestIDIsGeneratedAndReturned(t *testing.T) {
	w, seen := serveWithRequestID("")
	got := w.Header().Get(RequestIDHeader)
	if got == "" || got != seen {
		t.Fatalf("header %q, context %q: both must carry the generated id", got, seen)
	}
}

func TestRequestIDKeepsASafeIdFromTheCaller(t *testing.T) {
	w, seen := serveWithRequestID("gateway-123.abc_DEF")
	if w.Header().Get(RequestIDHeader) != "gateway-123.abc_DEF" || seen != "gateway-123.abc_DEF" {
		t.Fatalf("a safe caller id must be kept, got header %q context %q", w.Header().Get(RequestIDHeader), seen)
	}
}

func TestRequestIDReplacesAnUnsafeOrHugeId(t *testing.T) {
	for _, bad := range []string{"with space", "new\nline", "<script>", strings.Repeat("a", 65)} {
		w, _ := serveWithRequestID(bad)
		got := w.Header().Get(RequestIDHeader)
		if got == bad || got == "" {
			t.Errorf("unsafe id %q must be replaced, got %q", bad, got)
		}
	}
}

func TestAccessLogCarriesTheRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var out strings.Builder
	prev := gin.DefaultWriter
	gin.DefaultWriter = &out
	defer func() { gin.DefaultWriter = prev }()

	r := gin.New()
	r.Use(RequestID(), AccessLog())
	r.GET("/x", func(c *gin.Context) {})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestIDHeader, "req-42")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(out.String(), "req=req-42") {
		t.Fatalf("the access log line must carry the request id, got %q", out.String())
	}
}
