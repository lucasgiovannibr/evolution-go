package call_handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	call_engine "github.com/evolution-foundation/evolution-go/pkg/call/engine"
	"github.com/evolution-foundation/evolution-go/pkg/call/engine/enginetest"
	call_service "github.com/evolution-foundation/evolution-go/pkg/call/service"
	call_stream "github.com/evolution-foundation/evolution-go/pkg/call/stream"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
)

// fakeWhatsmeow is the WhatsmeowService of the tests: only the call engine is real.
type fakeWhatsmeow struct {
	whatsmeow_service.WhatsmeowService
	engine *call_engine.Manager
}

func (f fakeWhatsmeow) CallEngine() *call_engine.Manager { return f.engine }

type noLog struct{}

func (noLog) LogInfo(string, ...interface{})  {}
func (noLog) LogWarn(string, ...interface{})  {}
func (noLog) LogError(string, ...interface{}) {}
func (noLog) LogDebug(string, ...interface{}) {}

type env struct {
	t       *testing.T
	engine  *call_engine.Manager
	tickets *call_stream.Tickets
	router  *gin.Engine
}

// newEnv builds the call routes the way routes.go does, for the instance "inst" whose
// call engine is active; "bare" is an instance without one.
func newEnv(t *testing.T) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)

	engine := call_engine.NewManager(call_engine.Options{})
	if st := engine.Attach("inst", whatsmeow.NewClient(&store.Device{}, nil), false, noLog{}); st.State != call_engine.StateActive {
		t.Fatalf("engine state = %s (%s)", st.State, st.Error)
	}
	tickets := call_stream.NewTickets()
	svc := call_service.NewCallService(nil, fakeWhatsmeow{engine: engine}, tickets, nil)
	h := NewCallHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("instance", &instance_model.Instance{Id: c.GetHeader("X-Instance")})
	})
	g := r.Group("/call")
	g.GET("/active", h.ActiveCalls)
	g.GET("/:callId", h.GetCall)
	g.POST("/answer", h.AnswerCall)
	g.POST("/hangup", h.HangupCall)
	g.POST("/stream-ticket", h.StreamTicket)

	return &env{t: t, engine: engine, tickets: tickets, router: r}
}

func (e *env) call(method, path, instance string, body interface{}) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("X-Instance", instance)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func (e *env) ringing(id string) *enginetest.Fake {
	f := enginetest.NewFake(id)
	if _, err := e.engine.Track("inst", f, call_engine.Incoming); err != nil {
		e.t.Fatal(err)
	}
	return f
}

func decode(t *testing.T, w *httptest.ResponseRecorder, into interface{}) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), into); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
}

func TestActiveAndAnIdShareTheCallPrefix(t *testing.T) {
	e := newEnv(t)
	e.ringing("C1")

	w := e.call("GET", "/call/active", "inst", nil)
	var active call_service.ActiveCallsResult
	decode(t, w, &active)
	if w.Code != http.StatusOK || !active.Enabled || len(active.Calls) != 1 {
		t.Fatalf("/call/active: %d %s", w.Code, w.Body.String())
	}

	w = e.call("GET", "/call/C1", "inst", nil)
	var info call_engine.Info
	decode(t, w, &info)
	if w.Code != http.StatusOK || info.CallID != "C1" || info.Phase != call_engine.PhaseRinging || info.Direction != call_engine.Incoming {
		t.Fatalf("/call/C1: %d %s", w.Code, w.Body.String())
	}
}

func TestGetCallOfAnotherInstanceOrUnknownIs404(t *testing.T) {
	e := newEnv(t)
	e.ringing("C1")

	if w := e.call("GET", "/call/nope", "inst", nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown call: %d", w.Code)
	}
	// "bare" has no call engine at all
	if w := e.call("GET", "/call/C1", "bare", nil); w.Code != http.StatusConflict {
		t.Fatalf("instance without engine: %d", w.Code)
	}
}

func TestAnswer(t *testing.T) {
	e := newEnv(t)
	call := e.ringing("C1")

	if w := e.call("POST", "/call/answer", "inst", map[string]string{}); w.Code != http.StatusBadRequest {
		t.Fatalf("no callId: %d", w.Code)
	}
	if w := e.call("POST", "/call/answer", "inst", map[string]string{"callId": "nope"}); w.Code != http.StatusNotFound {
		t.Fatalf("unknown call: %d", w.Code)
	}
	if w := e.call("POST", "/call/answer", "bare", map[string]string{"callId": "C1"}); w.Code != http.StatusConflict {
		t.Fatalf("instance without engine: %d", w.Code)
	}

	w := e.call("POST", "/call/answer", "inst", map[string]string{"callId": "C1"})
	var info call_engine.Info
	decode(t, w, &info)
	if w.Code != http.StatusOK || info.Phase != call_engine.PhaseConnecting || call.Answered() != 1 {
		t.Fatalf("answer: %d %s (answered %d)", w.Code, w.Body.String(), call.Answered())
	}

	if w := e.call("POST", "/call/answer", "inst", map[string]string{"callId": "C1"}); w.Code != http.StatusConflict {
		t.Fatalf("second answer: %d", w.Code)
	}
	if call.Answered() != 1 {
		t.Fatalf("answered %d times", call.Answered())
	}
}

func TestHangup(t *testing.T) {
	e := newEnv(t)
	e.ringing("C1")

	if w := e.call("POST", "/call/hangup", "inst", map[string]string{}); w.Code != http.StatusBadRequest {
		t.Fatalf("no callId: %d", w.Code)
	}
	if w := e.call("POST", "/call/hangup", "inst", map[string]string{"callId": "C1"}); w.Code != http.StatusOK {
		t.Fatalf("hangup: %d %s", w.Code, w.Body.String())
	}
	if w := e.call("POST", "/call/hangup", "inst", map[string]string{"callId": "C1"}); w.Code != http.StatusNotFound {
		t.Fatalf("second hangup: %d", w.Code)
	}
	if len(e.engine.List("inst")) != 0 {
		t.Fatal("the call is still tracked")
	}
}

func TestStreamTicket(t *testing.T) {
	e := newEnv(t)
	e.ringing("C1")

	if w := e.call("POST", "/call/stream-ticket", "inst", map[string]string{"callId": "nope"}); w.Code != http.StatusNotFound {
		t.Fatalf("unknown call: %d", w.Code)
	}
	if w := e.call("POST", "/call/stream-ticket", "bare", map[string]string{"callId": "C1"}); w.Code != http.StatusConflict {
		t.Fatalf("instance without engine: %d", w.Code)
	}

	w := e.call("POST", "/call/stream-ticket", "inst", map[string]string{"callId": "C1"})
	var ticket call_service.StreamTicket
	decode(t, w, &ticket)
	if w.Code != http.StatusOK || ticket.Ticket == "" || ticket.ExpiresInSeconds != 30 ||
		ticket.Path != "/call/stream/C1?ticket="+ticket.Ticket {
		t.Fatalf("ticket: %d %s", w.Code, w.Body.String())
	}

	// it opens that call for that instance, once
	if instance, ok := e.tickets.Redeem(ticket.Ticket, "C1"); !ok || instance != "inst" {
		t.Fatalf("Redeem = %q, %v", instance, ok)
	}
	if _, ok := e.tickets.Redeem(ticket.Ticket, "C1"); ok {
		t.Fatal("the ticket worked twice")
	}
}
