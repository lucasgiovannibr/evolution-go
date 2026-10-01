package auth_middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MediaBodyPaths are the routes that legitimately receive a file (multipart, or base64 in
// the JSON body); every other route gets the small default limit.
var MediaBodyPaths = []string{
	"/send/media",
	"/send/status/media",
	"/user/profilePicture",
	"/group/photo",
}

// LimitBody bounds the size of a request body. Nothing bounded it before, so a single
// 100 MB JSON body made the process allocate ~350 MB just to be rejected.
//
// A body that announces a larger Content-Length is refused at once with 413; one that is
// larger without announcing it (chunked) fails when the handler reads it.
func LimitBody(defaultMax, mediaMax int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := defaultMax
		for _, p := range MediaBodyPaths {
			if c.Request.URL.Path == p {
				limit = mediaMax
				break
			}
		}

		if c.Request.ContentLength > limit {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":    "request body too large",
				"code":     "payload_too_large",
				"maxBytes": limit,
			})
			return
		}
		if c.Request.Body != nil && c.Request.Method != http.MethodGet && !strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}
