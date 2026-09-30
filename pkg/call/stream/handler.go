package call_stream

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	call_engine "github.com/evolution-foundation/evolution-go/pkg/call/engine"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 20 * time.Second

	// A second of audio is 32 kB, 43 kB in base64; this leaves room for a few seconds
	// per message and no more.
	maxMessageBytes = 256 * 1024
)

// Config is what the stream needs from the configuration.
type Config struct {
	// AllowedOrigins are the browser origins (scheme://host[:port]) allowed to open the
	// stream besides the server's own. "*" allows every origin. A client that sends no
	// Origin header (a server, a script) is always allowed: the ticket is what protects
	// the stream, the origin check only keeps other websites from using a logged-in
	// browser's ticket.
	AllowedOrigins []string
}

// message is the JSON envelope of the socket, modelled on Twilio Media Streams so
// existing voice integrations need little adapting.
//
// Server to client:
//
//	{"event":"start", "callId", "sampleRate":16000, "channels":1, "encoding":"audio/pcm-s16le", "frameMs":60, ...}
//	{"event":"media", "track":"inbound", "seq":N, "payload":"<base64 pcm>"}   the peer's audio
//	{"event":"error", "code", "message"}
//	{"event":"stop",  "reason"}                                                the call ended
//
// Client to server:
//
//	{"event":"media", "payload":"<base64 pcm>"}   audio for the peer, any chunk size
//	{"event":"clear"}                             drop the audio queued for the peer
//	{"event":"stop"}                              close the stream (the call is kept for a while)
type message struct {
	Event      string `json:"event"`
	CallID     string `json:"callId,omitempty"`
	Track      string `json:"track,omitempty"`
	Seq        uint64 `json:"seq,omitempty"`
	Payload    string `json:"payload,omitempty"`
	SampleRate int    `json:"sampleRate,omitempty"`
	Channels   int    `json:"channels,omitempty"`
	Encoding   string `json:"encoding,omitempty"`
	FrameMs    int    `json:"frameMs,omitempty"`
	Direction  string `json:"direction,omitempty"`
	Video      *bool  `json:"video,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
}

type handler struct {
	engine   *call_engine.Manager
	tickets  *Tickets
	upgrader websocket.Upgrader
	origins  []string
}

// RegisterRoutes mounts GET /call/stream/:callId on the engine, outside the apikey
// middleware: it is authorised by the ticket instead (see Tickets).
func RegisterRoutes(r *gin.Engine, engine *call_engine.Manager, tickets *Tickets, cfg Config) {
	h := &handler{engine: engine, tickets: tickets, origins: cfg.AllowedOrigins}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     h.checkOrigin,
	}
	r.GET("/call/stream/:callId", h.serve)
}

func (h *handler) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	for _, allowed := range h.origins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

func (h *handler) serve(c *gin.Context) {
	callID := c.Param("callId")

	instanceID, ok := h.tickets.Redeem(c.Query("ticket"), callID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired ticket"})
		return
	}
	if _, ok := h.engine.Get(instanceID, callID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": call_engine.ErrCallNotFound.Error()})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // Upgrade already answered
	}
	(&session{engine: h.engine, conn: conn, instanceID: instanceID, callID: callID}).run()
}

// session is one open socket attached to one call.
type session struct {
	engine     *call_engine.Manager
	conn       *websocket.Conn
	instanceID string
	callID     string

	stats  call_engine.StreamStats
	bridge *bridge
	call   *call_engine.Tracked

	writeMu   sync.Mutex
	quit      chan struct{}
	quitOnce  sync.Once
	errorSent map[string]bool
}

func (s *session) send(m message) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return s.conn.WriteJSON(m)
}

// sendError tells the client about a problem, once per kind: a client that keeps
// sending bad data must not get a message for each chunk.
func (s *session) sendError(code, text string) {
	if s.errorSent[code] {
		return
	}
	s.errorSent[code] = true
	_ = s.send(message{Event: "error", Code: code, Message: text})
}

func (s *session) stop() { s.quitOnce.Do(func() { close(s.quit) }) }

func (s *session) run() {
	defer s.conn.Close()
	s.quit = make(chan struct{})
	s.errorSent = map[string]bool{}
	s.bridge = newBridge(&s.stats)

	call, detach, err := s.engine.AttachStream(s.instanceID, s.callID, s.bridge, s.bridge, &s.stats)
	if err != nil {
		code := "call_not_found"
		if errors.Is(err, call_engine.ErrStreamBusy) {
			code = "stream_busy"
		}
		_ = s.send(message{Event: "error", Code: code, Message: err.Error()})
		s.closeWith(websocket.ClosePolicyViolation, code)
		return
	}
	defer detach()
	s.call = call

	info := call.Info()
	video := info.Video
	if err := s.send(message{
		Event: "start", CallID: s.callID,
		SampleRate: call_engine.SampleRate, Channels: 1, Encoding: "audio/pcm-s16le",
		FrameMs:   call_engine.FrameSamples * 1000 / call_engine.SampleRate,
		Direction: string(info.Direction), Video: &video,
	}); err != nil {
		return
	}

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		s.writeLoop()
	}()
	s.readLoop()
	s.stop()
	<-writerDone
	s.bridge.Close()
}

func (s *session) closeWith(code int, text string) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(writeWait))
}

func (s *session) sendFrame(seq *uint64, frame []float32) error {
	*seq++
	err := s.send(message{
		Event: "media", Track: "inbound", Seq: *seq,
		Payload: base64.StdEncoding.EncodeToString(pcm16(frame)),
	})
	if err == nil {
		s.stats.ToClient.Add(1)
	}
	return err
}

// writeLoop is the only writer of the socket besides sendError: the peer's audio, the
// keepalive pings and the end of the call.
func (s *session) writeLoop() {
	ping := time.NewTicker(pingPeriod)
	defer ping.Stop()

	var seq uint64
	for {
		select {
		case frame := <-s.bridge.toClient:
			if err := s.sendFrame(&seq, frame); err != nil {
				s.conn.Close()
				return
			}

		case <-ping.C:
			s.writeMu.Lock()
			err := s.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
			s.writeMu.Unlock()
			if err != nil {
				s.conn.Close()
				return
			}

		case <-s.call.Done():
			// what the peer said last still reaches the client before the stop
			for drained := false; !drained; {
				select {
				case frame := <-s.bridge.toClient:
					if s.sendFrame(&seq, frame) != nil {
						drained = true
					}
				default:
					drained = true
				}
			}
			_ = s.send(message{Event: "stop", Reason: s.call.Reason()})
			s.closeWith(websocket.CloseNormalClosure, "call ended")
			s.conn.Close() // ends readLoop
			return

		case <-s.quit:
			return
		}
	}
}

func (s *session) readLoop() {
	s.conn.SetReadLimit(maxMessageBytes)
	_ = s.conn.SetReadDeadline(time.Now().Add(pongWait))
	s.conn.SetPongHandler(func(string) error {
		return s.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var m message
		if err := s.conn.ReadJSON(&m); err != nil {
			var syntax *json.SyntaxError
			var typeErr *json.UnmarshalTypeError
			if errors.As(err, &syntax) || errors.As(err, &typeErr) {
				s.sendError("bad_message", "messages must be JSON objects")
				continue
			}
			return
		}
		_ = s.conn.SetReadDeadline(time.Now().Add(pongWait))

		switch m.Event {
		case "media":
			if m.Track != "" && m.Track != "outbound" {
				continue // not audio for the peer (an echo of "inbound", say)
			}
			pcm, err := base64.StdEncoding.DecodeString(m.Payload)
			if err != nil {
				s.sendError("bad_payload", "payload must be base64 of 16-bit little-endian mono PCM at 16 kHz")
				continue
			}
			before := s.stats.DroppedFromClient.Load()
			s.bridge.Push(pcm)
			if s.stats.DroppedFromClient.Load() != before {
				s.sendError("outbound_overflow", "too much audio is queued for the peer; it is being dropped")
			}
		case "clear":
			s.bridge.Clear()
		case "stop":
			return
		}
		// Unknown events are ignored, so the protocol can grow without breaking clients.
	}
}
