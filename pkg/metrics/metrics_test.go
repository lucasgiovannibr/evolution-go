package metrics

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	producer_interfaces "github.com/evolution-foundation/evolution-go/pkg/events/interfaces"
)

func scrape(t *testing.T, r *gin.Engine) string {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/metrics answered %d", w.Code)
	}
	return w.Body.String()
}

func TestHTTPMetricsUseTheRoutePatternNotThePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/instance/:instanceId/runtime", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/metrics", Handler())

	for _, id := range []string{"aaa", "bbb", "ccc"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/instance/"+id+"/runtime", nil))
	}
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nowhere/secret-token", nil))

	out := scrape(t, r)
	if !strings.Contains(out, `evolution_http_requests_total{method="GET",route="/instance/:instanceId/runtime",status="200"} 3`) {
		t.Fatalf("three requests to different ids must count under the one route pattern:\n%s", grepLines(out, "evolution_http_requests_total"))
	}
	if strings.Contains(out, "aaa") || strings.Contains(out, "secret-token") {
		t.Fatal("concrete paths (ids, tokens) must never become label values")
	}
	if !strings.Contains(out, `route="unmatched"`) {
		t.Fatal("a request that matches no route is counted under \"unmatched\"")
	}
	if !strings.Contains(out, "evolution_http_request_duration_seconds_bucket") {
		t.Fatal("the latency histogram is missing")
	}
}

func TestWebhookQueueMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	RegisterWebhookQueues(func() *producer_interfaces.WebhookStats {
		return &producer_interfaces.WebhookStats{Pending: 7, PendingBytes: 1234, Sent: 40, Failed: 2, Dropped: 1}
	})
	r := gin.New()
	r.GET("/metrics", Handler())

	out := scrape(t, r)
	for _, want := range []string{
		"evolution_webhook_pending_events 7",
		"evolution_webhook_pending_bytes 1234",
		"evolution_webhook_sent_total 40",
		"evolution_webhook_failed_total 2",
		"evolution_webhook_dropped_total 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, grepLines(out, "evolution_webhook"))
		}
	}
}

func TestDBPoolMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var db *sql.DB
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(25)
	RegisterDBStats("users", db)
	RegisterDBStats("ignored", nil) // a missing database is skipped, not a crash

	r := gin.New()
	r.GET("/metrics", Handler())
	out := scrape(t, r)
	if !strings.Contains(out, `evolution_db_connections_max{db="users"} 25`) {
		t.Fatalf("pool limit missing:\n%s", grepLines(out, "evolution_db"))
	}
}

func TestEventsCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	Events.WithLabelValues("Message").Inc()
	Events.WithLabelValues("Message").Inc()
	r := gin.New()
	r.GET("/metrics", Handler())
	if out := scrape(t, r); !strings.Contains(out, `evolution_whatsapp_events_total{type="Message"} 2`) {
		t.Fatalf("event counter missing:\n%s", grepLines(out, "evolution_whatsapp"))
	}
}

func grepLines(s, sub string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
