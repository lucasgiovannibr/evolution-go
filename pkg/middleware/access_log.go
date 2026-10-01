package auth_middleware

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// sensitiveQueryKeys are the query parameters that carry a credential. The default gin
// logger prints the whole query string, so a request like GET /ws?token=<GLOBAL_API_KEY>
// used to write the master key to the access log.
var sensitiveQueryKeys = map[string]bool{
	"token":  true,
	"apikey": true,
	"key":    true,
	"ticket": true,
}

// RedactQuery returns rawQuery with the value of every credential parameter replaced by
// "REDACTED". The other parameters, and their order, are kept as they were sent.
func RedactQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	parts := strings.Split(rawQuery, "&")
	for i, part := range parts {
		name, _, hasValue := strings.Cut(part, "=")
		if !hasValue {
			continue
		}
		if decoded, err := url.QueryUnescape(name); err == nil {
			name = decoded
		}
		if sensitiveQueryKeys[strings.ToLower(name)] {
			key, _, _ := strings.Cut(part, "=")
			parts[i] = key + "=REDACTED"
		}
	}
	return strings.Join(parts, "&")
}

// AccessLog is gin's default request logger with credentials removed from the query
// string. The line format is the default one (without colors), so existing log
// parsers keep working.
func AccessLog() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(p gin.LogFormatterParams) string {
			path := p.Request.URL.Path
			if q := RedactQuery(p.Request.URL.RawQuery); q != "" {
				path += "?" + q
			}
			return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %#v\n%s",
				p.TimeStamp.Format("2006/01/02 - 15:04:05"),
				p.StatusCode,
				p.Latency,
				p.ClientIP,
				p.Method,
				path,
				p.ErrorMessage,
			)
		},
	})
}
