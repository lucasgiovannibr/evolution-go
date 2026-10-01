package auth_middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestManagerSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/manager", ManagerSecurityHeaders(), func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/api", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/manager", nil))
	csp := w.Header().Get("Content-Security-Policy")
	for _, want := range []string{"script-src 'self';", "object-src 'none'", "frame-ancestors 'none'", "base-uri 'none'"} {
		if !strings.Contains(csp, want) {
			t.Fatalf("CSP lacks %q: %s", want, csp)
		}
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe") || strings.Contains(csp, "unsafe-eval") {
		t.Fatalf("scripts must not be allowed inline or eval: %s", csp)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("headers: %v", w.Header())
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api", nil))
	if w.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("the API must not carry the manager CSP")
	}
}
