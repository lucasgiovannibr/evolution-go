package auth_middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	instance_service "github.com/evolution-foundation/evolution-go/pkg/instance/service"
	"github.com/gin-gonic/gin"
)

// fakeInstances only implements the lookup the middleware needs.
type fakeInstances struct {
	instance_service.InstanceService
	byToken map[string]*instance_model.Instance
}

func (f fakeInstances) GetInstanceByToken(token string) (*instance_model.Instance, error) {
	if i, ok := f.byToken[token]; ok {
		return i, nil
	}
	return nil, errors.New("not found")
}

func run(t *testing.T, apikey string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	m := NewMiddleware(&config.Config{GlobalApiKey: "global"}, fakeInstances{byToken: map[string]*instance_model.Instance{
		"tokA": {Id: "A"},
		"tokB": {Id: "B"},
	}})
	r := gin.New()
	r.GET("/instance/:instanceId/advanced-settings", m.AuthInstanceScoped, func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/instance/A/advanced-settings", nil)
	if apikey != "" {
		req.Header.Set("apikey", apikey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestAuthInstanceScoped(t *testing.T) {
	cases := []struct {
		name   string
		apikey string
		want   int
	}{
		{"no key", "", http.StatusUnauthorized},
		{"unknown key", "nope", http.StatusUnauthorized},
		{"global key", "global", http.StatusOK},
		{"own instance token", "tokA", http.StatusOK},
		{"other instance token", "tokB", http.StatusForbidden},
	}
	for _, c := range cases {
		if got := run(t, c.apikey); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
