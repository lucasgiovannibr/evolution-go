package auth_middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSWildcardHasNoCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(nil))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://anything.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin = %q, want *", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("a wildcard origin must not allow credentials, got %q", got)
	}
}

func TestCORSAllowList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"https://app.example.com"}))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	do := func(origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	allowed := do("https://app.example.com")
	if allowed.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" || allowed.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("a listed origin must be echoed back with credentials, got %v", allowed.Header())
	}
	if !strings.Contains(allowed.Header().Get("Vary"), "Origin") {
		t.Fatal("answers that depend on the Origin must send Vary: Origin")
	}

	if got := do("https://evil.example").Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("an unlisted origin must not be allowed, got %q", got)
	}
	if got := do("").Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("a request without Origin gets no CORS grant, got %q", got)
	}
}

func TestCORSPreflightIsAnswered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(nil))
	called := false
	r.POST("/x", func(c *gin.Context) { called = true })

	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK || called {
		t.Fatalf("a preflight must be answered by the middleware (code %d, handler called %v)", w.Code, called)
	}
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatal("the preflight must list the allowed headers")
	}
}

func limitRouter(def, media int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LimitBody(def, media))
	read := func(c *gin.Context) {
		if _, err := io.Copy(io.Discard, c.Request.Body); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		c.Status(http.StatusOK)
	}
	r.POST("/send/text", read)
	r.POST("/send/media", read)
	return r
}

func TestLimitBodyRefusesAnAnnouncedOversizeBody(t *testing.T) {
	r := limitRouter(10, 100)
	req := httptest.NewRequest(http.MethodPost, "/send/text", strings.NewReader(strings.Repeat("a", 11)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %d, want 413", w.Code)
	}
}

func TestLimitBodyStopsAnUnannouncedOversizeBody(t *testing.T) {
	r := limitRouter(10, 100)
	req := httptest.NewRequest(http.MethodPost, "/send/text", io.NopCloser(strings.NewReader(strings.Repeat("a", 50))))
	req.ContentLength = -1 // chunked: the size is not known up front
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("reading an oversize chunked body must fail, got code %d", w.Code)
	}
}

func TestLimitBodyAllowsMoreOnMediaRoutes(t *testing.T) {
	r := limitRouter(10, 100)

	ok := httptest.NewRecorder()
	r.ServeHTTP(ok, httptest.NewRequest(http.MethodPost, "/send/media", strings.NewReader(strings.Repeat("a", 50))))
	if ok.Code != http.StatusOK {
		t.Fatalf("a media route must accept a body above the default limit, got %d", ok.Code)
	}

	tooBig := httptest.NewRecorder()
	r.ServeHTTP(tooBig, httptest.NewRequest(http.MethodPost, "/send/media", strings.NewReader(strings.Repeat("a", 101))))
	if tooBig.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("a media route still has a limit, got %d", tooBig.Code)
	}
}
