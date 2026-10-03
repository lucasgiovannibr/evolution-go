package auth_middleware

import "github.com/gin-gonic/gin"

// managerCSP is the Content-Security-Policy of the manager panel (a single-page app that
// talks to this API, or to another WhatyGo the operator types in).
//
//   - script-src 'self': no inline scripts and no eval; the theme bootstrap is a file of its own;
//   - style-src allows inline styles (the UI sets style attributes), nothing else is inline;
//   - connect-src is open to http(s)/ws(s): the panel is pointed at an API URL chosen at login;
//   - img-src allows https and data/blob (profile pictures come from WhatsApp's CDN);
//   - the page cannot be framed, cannot be given a <base>, and posts forms only to itself.
const managerCSP = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob: https:; " +
	"font-src 'self' data:; " +
	"connect-src 'self' http: https: ws: wss:; " +
	"object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// ManagerSecurityHeaders sets the browser-side protections for the pages of the manager.
// It is not applied to the API (JSON) or to Swagger UI, which needs inline scripts.
func ManagerSecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Content-Security-Policy", managerCSP)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		c.Next()
	}
}
