package auth_middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// RequestIDHeader is read from the caller (so a gateway's id is kept) and always sent
// back.
const RequestIDHeader = "X-Request-ID"

// RequestIDKey is where the id is stored in the gin context.
const RequestIDKey = "requestId"

const maxRequestIDLength = 64

// RequestID gives every request an id, answers it in X-Request-ID and exposes it to the
// handlers and to the access log, so a failing call can be found in the logs. An id
// sent by the caller is kept only if it is short and made of safe characters.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !validRequestID(id) {
			id = newRequestID()
		}
		c.Set(RequestIDKey, id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLength {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
