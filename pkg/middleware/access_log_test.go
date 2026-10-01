package auth_middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRedactQuery(t *testing.T) {
	cases := map[string]string{
		"":                                  "",
		"instanceId=abc":                    "instanceId=abc",
		"token=SECRET&instanceId=abc":       "token=REDACTED&instanceId=abc",
		"instanceId=abc&token=SECRET":       "instanceId=abc&token=REDACTED",
		"TOKEN=SECRET":                      "TOKEN=REDACTED",
		"apikey=A&ticket=B&key=C&other=D":   "apikey=REDACTED&ticket=REDACTED&key=REDACTED&other=D",
		"to%6Ben=SECRET":                    "to%6Ben=REDACTED", // percent-encoded name
		"token":                             "token",            // no value, nothing to hide
		"start_date=2026-01-01&limit=5":     "start_date=2026-01-01&limit=5",
		"tokenizer=keep&mytoken=keep&x=a=b": "tokenizer=keep&mytoken=keep&x=a=b",
	}
	for in, want := range cases {
		if got := RedactQuery(in); got != want {
			t.Errorf("RedactQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAccessLogNeverPrintsTheCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var out bytes.Buffer
	prev := gin.DefaultWriter
	gin.DefaultWriter = &out
	defer func() { gin.DefaultWriter = prev }()

	r := gin.New()
	r.Use(AccessLog())
	r.GET("/ws", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/ws?token=TOP-SECRET-KEY&instanceId=abc", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	line := out.String()
	if strings.Contains(line, "TOP-SECRET-KEY") {
		t.Fatalf("the access log leaked the credential: %q", line)
	}
	if !strings.Contains(line, "/ws?token=REDACTED&instanceId=abc") {
		t.Fatalf("expected the redacted path in the access log, got %q", line)
	}
}
