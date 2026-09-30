package call_stream

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	call_engine "github.com/evolution-foundation/evolution-go/pkg/call/engine"
	"github.com/evolution-foundation/evolution-go/pkg/call/engine/enginetest"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type rig struct {
	t       *testing.T
	engine  *call_engine.Manager
	tickets *Tickets
	server  *httptest.Server
}

func newRig(t *testing.T, cfg Config) *rig {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := &rig{t: t, engine: call_engine.NewManager(call_engine.Options{}), tickets: NewTickets()}
	engine := gin.New()
	RegisterRoutes(engine, r.engine, r.tickets, cfg)
	r.server = httptest.NewServer(engine)
	t.Cleanup(r.server.Close)
	return r
}

// track adds a running incoming call to the instance.
func (r *rig) track(instance, id string) *enginetest.Fake {
	f := enginetest.NewFake(id)
	f.SetPhase(call_engine.PhaseActive)
	if _, err := r.engine.Track(instance, f, call_engine.Incoming); err != nil {
		r.t.Fatal(err)
	}
	return f
}

func (r *rig) url(callID, ticket string) string {
	return "ws" + strings.TrimPrefix(r.server.URL, "http") + "/call/stream/" + callID + "?ticket=" + ticket
}

func (r *rig) dial(instance, callID string, header http.Header) (*websocket.Conn, *http.Response, error) {
	token, _, err := r.tickets.Issue(instance, callID)
	if err != nil {
		r.t.Fatal(err)
	}
	return websocket.DefaultDialer.Dial(r.url(callID, token), header)
}

func (r *rig) mustDial(instance, callID string) *websocket.Conn {
	r.t.Helper()
	conn, _, err := r.dial(instance, callID, nil)
	if err != nil {
		r.t.Fatalf("dial: %v", err)
	}
	r.t.Cleanup(func() { conn.Close() })
	return conn
}

func read(t *testing.T, conn *websocket.Conn) message {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var m message
	if err := conn.ReadJSON(&m); err != nil {
		t.Fatalf("read: %v", err)
	}
	return m
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTheStreamCarriesAudioBothWaysAndReportsTheEnd(t *testing.T) {
	r := newRig(t, Config{})
	call := r.track("inst", "C1")
	conn := r.mustDial("inst", "C1")

	start := read(t, conn)
	if start.Event != "start" || start.CallID != "C1" || start.SampleRate != 16000 || start.Channels != 1 ||
		start.Encoding != "audio/pcm-s16le" || start.FrameMs != 60 || start.Direction != "incoming" {
		t.Fatalf("start = %+v", start)
	}

	// the peer's audio reaches the client
	eventually(t, "the sink to be attached", func() bool { return call.Sink() != nil })
	call.Sink().WriteFrame([]float32{0.5, -0.5})
	media := read(t, conn)
	pcm, err := base64.StdEncoding.DecodeString(media.Payload)
	if media.Event != "media" || media.Track != "inbound" || media.Seq != 1 || err != nil || len(pcm) != 4 {
		t.Fatalf("media = %+v (%v)", media, err)
	}

	// the client's audio reaches the call
	eventually(t, "the source to be attached", func() bool { return call.Source() != nil })
	chunk := make([]byte, call_engine.FrameSamples*2)
	if err := conn.WriteJSON(message{Event: "media", Payload: base64.StdEncoding.EncodeToString(chunk)}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the client's frame to be playable", func() bool {
		frame, _ := call.Source().ReadFrame()
		return len(frame) == call_engine.FrameSamples
	})

	// the end of the call is reported, then the socket closes
	call.End("terminate")
	stop := read(t, conn)
	if stop.Event != "stop" || stop.Reason != "terminate" {
		t.Fatalf("stop = %+v", stop)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("the socket stayed open after the call ended")
	}
}

func TestPeerAudioBeforeTheEndIsNotLost(t *testing.T) {
	r := newRig(t, Config{})
	call := r.track("inst", "C1")
	conn := r.mustDial("inst", "C1")
	read(t, conn) // start
	eventually(t, "the sink", func() bool { return call.Sink() != nil })

	call.Sink().WriteFrame([]float32{0.25})
	call.End("terminate")

	got := []string{}
	for {
		m := read(t, conn)
		got = append(got, m.Event)
		if m.Event == "stop" {
			break
		}
	}
	if len(got) != 2 || got[0] != "media" {
		t.Fatalf("events = %v, want the last audio before the stop", got)
	}
}

func TestATicketIsRequiredAndWorksOnce(t *testing.T) {
	r := newRig(t, Config{})
	r.track("inst", "C1")

	_, resp, err := websocket.DefaultDialer.Dial(r.url("C1", ""), nil)
	if err == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no ticket: err=%v status=%v", err, resp)
	}

	token, _, _ := r.tickets.Issue("inst", "C1")
	conn, _, err := websocket.DefaultDialer.Dial(r.url("C1", token), nil)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	_, resp, err = websocket.DefaultDialer.Dial(r.url("C1", token), nil)
	if err == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reused ticket: err=%v status=%v", err, resp)
	}

	other, _, _ := r.tickets.Issue("inst", "C2")
	_, resp, err = websocket.DefaultDialer.Dial(r.url("C1", other), nil)
	if err == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ticket of another call: err=%v status=%v", err, resp)
	}
}

func TestATicketDoesNotOpenACallOfAnotherInstance(t *testing.T) {
	r := newRig(t, Config{})
	r.track("inst", "C1")

	_, resp, err := r.dial("intruder", "C1", nil)

	if err == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("err=%v status=%v, want 404", err, resp)
	}
}

func TestASecondStreamIsRefusedAndTheFirstKeepsWorking(t *testing.T) {
	r := newRig(t, Config{})
	call := r.track("inst", "C1")
	first := r.mustDial("inst", "C1")
	read(t, first) // start
	eventually(t, "the sink", func() bool { return call.Sink() != nil })

	second := r.mustDial("inst", "C1")
	if m := read(t, second); m.Event != "error" || m.Code != "stream_busy" {
		t.Fatalf("second stream got %+v", m)
	}

	call.Sink().WriteFrame([]float32{0.1})
	if m := read(t, first); m.Event != "media" {
		t.Fatalf("the first stream was disturbed: %+v", m)
	}
}

func TestAStreamThatStopsLeavesTheCallAndFreesTheSlot(t *testing.T) {
	r := newRig(t, Config{})
	call := r.track("inst", "C1")
	call.SetPhase(call_engine.PhaseRinging) // not answered: no stream clock
	conn := r.mustDial("inst", "C1")
	read(t, conn)

	if err := conn.WriteJSON(message{Event: "stop"}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the stream to detach", func() bool {
		info := r.engine.List("inst")
		return len(info) == 1 && info[0].Stream != nil && !info[0].Stream.Attached
	})
	if len(r.engine.List("inst")) != 1 {
		t.Fatal("the call ended with its stream")
	}
	r.mustDial("inst", "C1") // a new stream can attach
}

func TestBadMessagesGetOneErrorEach(t *testing.T) {
	r := newRig(t, Config{})
	r.track("inst", "C1")
	conn := r.mustDial("inst", "C1")
	read(t, conn)

	for i := 0; i < 3; i++ {
		conn.WriteJSON(message{Event: "media", Payload: "not base64!!"})
	}
	conn.WriteMessage(websocket.TextMessage, []byte("not json"))
	conn.WriteMessage(websocket.TextMessage, []byte("not json"))

	got := map[string]int{}
	for i := 0; i < 2; i++ {
		got[read(t, conn).Code]++
	}
	if got["bad_payload"] != 1 || got["bad_message"] != 1 {
		t.Fatalf("errors = %v, want one of each kind", got)
	}

	// and the stream still works
	conn.WriteJSON(message{Event: "unknown-thing"})                                                                        // ignored
	conn.WriteJSON(message{Event: "media", Track: "inbound", Payload: base64.StdEncoding.EncodeToString(make([]byte, 4))}) // an echo: ignored
}

func TestOriginPolicy(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		origin  string
		allowed bool
	}{
		{"no origin (a server)", Config{}, "", true},
		{"foreign website", Config{}, "https://evil.example", false},
		{"listed origin", Config{AllowedOrigins: []string{"https://app.example"}}, "https://app.example", true},
		{"listed origin, other case", Config{AllowedOrigins: []string{"https://app.example"}}, "https://APP.example", true},
		{"unlisted origin", Config{AllowedOrigins: []string{"https://app.example"}}, "https://evil.example", false},
		{"wildcard", Config{AllowedOrigins: []string{"*"}}, "https://anything.example", true},
		{"garbage origin", Config{AllowedOrigins: []string{"*"}}, "://", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, c.cfg)
			r.track("inst", "C1")
			header := http.Header{}
			if c.origin != "" {
				header.Set("Origin", c.origin)
			}

			conn, resp, err := r.dial("inst", "C1", header)
			if conn != nil {
				conn.Close()
			}
			if c.allowed && err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !c.allowed && (err == nil || resp.StatusCode != http.StatusForbidden) {
				t.Fatalf("err=%v resp=%v, want 403", err, resp)
			}
		})
	}
}

func TestTheServersOwnOriginIsAllowed(t *testing.T) {
	r := newRig(t, Config{})
	r.track("inst", "C1")
	header := http.Header{}
	header.Set("Origin", r.server.URL) // same host as the request

	conn, _, err := r.dial("inst", "C1", header)
	if err != nil {
		t.Fatalf("the server's own origin was refused: %v", err)
	}
	conn.Close()
}

func TestAStreamThatIsClosedHangsUpARunningCallAfterTheGrace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := call_engine.NewManager(call_engine.Options{StreamGrace: 30 * time.Millisecond})
	tickets := NewTickets()
	g := gin.New()
	RegisterRoutes(g, engine, tickets, Config{})
	server := httptest.NewServer(g)
	defer server.Close()

	call := enginetest.NewFake("C1")
	call.SetPhase(call_engine.PhaseActive)
	tracked, _ := engine.Track("inst", call, call_engine.Incoming)

	token, _, _ := tickets.Issue("inst", "C1")
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/call/stream/C1?ticket="+token, nil)
	if err != nil {
		t.Fatal(err)
	}
	read(t, conn)
	conn.Close() // the consumer goes away

	select {
	case <-tracked.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("the call was never hung up")
	}
	if tracked.Reason() != "stream_closed" {
		t.Fatalf("reason = %q", tracked.Reason())
	}
}
