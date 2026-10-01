package auth_middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	corsMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	corsHeaders = "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, Cache-Control, X-Requested-With, apikey, ApiKey"
)

// CORS answers cross-origin requests and preflights.
//
// With no origins configured (or "*") every origin is allowed, as before, but without
// "Access-Control-Allow-Credentials": a browser refuses credentialed requests to a
// wildcard origin anyway, and the API authenticates with the apikey header, not cookies.
// With a list (CORS_ORIGINS), only those origins are echoed back, with credentials, and
// "Vary: Origin" so caches keep the answers apart.
func CORS(origins []string) gin.HandlerFunc {
	allowAny := len(origins) == 0
	for _, o := range origins {
		if strings.TrimSpace(o) == "*" {
			allowAny = true
		}
	}

	return func(c *gin.Context) {
		h := c.Writer.Header()
		origin := c.GetHeader("Origin")

		switch {
		case allowAny:
			h.Set("Access-Control-Allow-Origin", "*")
		case origin != "":
			h.Add("Vary", "Origin")
			for _, allowed := range origins {
				if strings.EqualFold(strings.TrimSpace(allowed), origin) {
					h.Set("Access-Control-Allow-Origin", origin)
					h.Set("Access-Control-Allow-Credentials", "true")
					break
				}
			}
		}

		h.Set("Access-Control-Allow-Methods", corsMethods)
		h.Set("Access-Control-Allow-Headers", corsHeaders)
		h.Set("Access-Control-Expose-Headers", "Content-Length")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}
