// Package call_engine is the only place that touches the WhatsApp call library
// (github.com/purpshell/meowcaller). Everything else talks to the Call interface
// and the Manager, so the library can be pinned, swapped or faked in tests without
// the handlers knowing.
//
// Three properties of the library shape this package:
//
//   - It hooks into whatsmeow through reflection on a private field, so a whatsmeow
//     bump can break it without any compile error. Attach reports that as
//     StateHookFailed instead of pretending calls work, and engine_test.go fails the
//     build when the pinned whatsmeow no longer fits.
//   - It answers every incoming offer with a preaccept by itself. That is a visible
//     side effect on the caller's phone, so an instance only gets an engine when the
//     operator asked for one (Instance.CallsEnabled).
//   - Its media goes over a UDP socket it opens itself and knows nothing about the
//     instance proxy, which would show WhatsApp the server's real address for an
//     account that is configured to hide it. Instances with a proxy get no engine.
//   - It keeps a single OnEnd/OnReady callback per call and has no way to end every
//     call of a client at once. The Manager therefore owns those callbacks (see
//     calls.go) and other code waits on Tracked.Done() instead, and ends the calls of
//     an instance itself when the instance goes away.
package call_engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/purpshell/meowcaller"
	"github.com/purpshell/meowcaller/signaling"
	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// Phase is where a call is in its life.
type Phase string

const (
	PhaseCalling    Phase = "calling"    // outgoing, waiting for the peer
	PhaseRinging    Phase = "ringing"    // incoming, not answered yet
	PhaseConnecting Phase = "connecting" // answered, media not flowing yet
	PhaseActive     Phase = "active"     // media flowing
	PhaseEnded      Phase = "ended"
	PhaseOther      Phase = "other" // idle or waiting room: nothing this project acts on
)

// The audio format of every call: 16 kHz mono, in frames of 60 ms. Taken from the
// library so a change there cannot go unnoticed.
const (
	SampleRate   = meowcaller.SampleRate
	FrameSamples = meowcaller.FrameSamples
)

// AudioSink consumes the peer's audio: 16 kHz mono float32 frames, normally 960
// samples (60 ms) but the decoder may hand over more than one frame at a time. The
// library calls it from its receive goroutine, so it must not block, and closes it
// when the call ends.
type AudioSink interface {
	WriteFrame(frame []float32) error
	Close() error
}

// AudioSource yields the audio to send. The library asks for a frame every 60 ms from
// its send loop, so ReadFrame must not block: nil frame and nil error means "nothing
// yet, send silence", and any error (io.EOF included) ends the source.
type AudioSource interface {
	ReadFrame() ([]float32, error)
	Close() error
}

// VideoSink consumes the peer's video: one H.264 access unit per call, Annex-B (start
// code prefixed NAL units). The library only carries video, it neither decodes nor
// encodes it. Like the audio sink it is called from the library's goroutine and must
// not block; the library closes it when the call ends. A sink that also has
// SetOrientation(int) is told the display rotation, in clockwise quarter turns.
type VideoSink interface {
	WriteVideo(accessUnit []byte) error
	Close() error
}

// VideoState is what the peer told us about its video during the call.
type VideoState struct {
	// Active: the peer's camera is on.
	Active bool `json:"active"`
	// Upgrade: the peer asks to turn an audio call into a video call (answer with
	// the "accept" video action).
	Upgrade bool `json:"upgrade"`
	// Orientation is the peer's device rotation as the peer reports it (0..3). It does not
	// follow the camera in use, so do not rotate the picture by it: every video message of
	// the stream carries the rotation to apply.
	Orientation int `json:"orientation"`
	// State is what the peer actually signalled, see the VideoState* constants. Active
	// and Upgrade alone cannot tell the peer accepting our upgrade from it turning its
	// camera off: both leave them false.
	State string `json:"state"`
}

// What the peer signalled about its video (VideoState.State).
const (
	// VideoStateEnabled: the peer's camera is on. Nobody asks or answers: in current
	// WhatsApp a side simply turns its camera on.
	VideoStateEnabled = "enabled"
	// VideoStateDisabled: the peer muted its camera; the video may come back.
	VideoStateDisabled = "disabled"
	// VideoStateStopped: the peer stopped sending video.
	VideoStateStopped = "stopped"
	// VideoStateUpgradeRequest: the peer asks to turn the call into a video call.
	VideoStateUpgradeRequest = "upgrade_request"
	// VideoStateUpgradeAccepted: the peer accepted the upgrade we asked for.
	VideoStateUpgradeAccepted = "upgrade_accepted"
	// VideoStateUpgradeRejected: the peer refused the upgrade we asked for.
	VideoStateUpgradeRejected = "upgrade_rejected"
	// VideoStateUpgradeCancelled: the peer took back its own upgrade request.
	VideoStateUpgradeCancelled = "upgrade_cancelled"
	// VideoStateUnknown: a state this code does not know; the call carries on.
	VideoStateUnknown = "unknown"
)

// videoStateName names the library's raw video state.
func videoStateName(raw int) string {
	switch raw {
	case signaling.VideoStateEnabled:
		return VideoStateEnabled
	case signaling.VideoStateDisabled:
		return VideoStateDisabled
	case signaling.VideoStateStopped:
		return VideoStateStopped
	case signaling.VideoStateUpgradeRequest, signaling.VideoStateUpgradeRequestV2:
		return VideoStateUpgradeRequest
	case signaling.VideoStateUpgradeAccept:
		return VideoStateUpgradeAccepted
	case signaling.VideoStateUpgradeReject:
		return VideoStateUpgradeRejected
	case signaling.VideoStateUpgradeCancel:
		return VideoStateUpgradeCancelled
	}
	return VideoStateUnknown
}

// Call is what the rest of the code may do with a live call. The library's call
// satisfies it through libCall; tests use fakes.
type Call interface {
	ID() string
	Peer() types.JID
	IsVideo() bool
	Phase() Phase
	Answer() error
	Reject() error
	Hangup() error
	// OnReady and OnEnd replace the previous callback: the library keeps only one.
	// The Manager owns them; everything else waits on Tracked.Done().
	OnReady(fn func())
	OnEnd(fn func(reason string))
	// Receive attaches the sink for the peer's audio (nil detaches), Play the source of
	// the audio sent to the peer. Each replaces the previous one.
	Receive(sink AudioSink)
	Play(src AudioSource)

	// Video. ReceiveVideo attaches the sink for the peer's video (nil detaches).
	// SendVideo sends one Annex-B access unit; duration is the time until the next
	// frame (it advances the RTP clock), zero for the library's default. OnVideoState
	// and OnVideoKeyframeRequest replace the previous callback, like OnEnd: the Manager
	// owns them. A keyframe request means WhatsApp lost part of our video: the next
	// access unit sent must be an IDR.
	ReceiveVideo(sink VideoSink)
	SendVideo(accessUnit []byte, duration time.Duration) error
	OnVideoState(fn func(VideoState))
	OnVideoKeyframeRequest(fn func())
	StartVideo() error
	AcceptVideo() error
	StopVideo() error
	SetVideoEnabled(enabled bool) error
	SetVideoOrientation(orientation int) error
	IsSendingVideo() bool
	IsReceivingVideo() bool
}

// libCall adapts the library's call to Call. The embedded *meowcaller.Call already
// has every method but Phase.
type libCall struct{ *meowcaller.Call }

var _ Call = libCall{}

func (c libCall) Receive(sink AudioSink) {
	if sink == nil {
		c.Call.Receive(nil)
		return
	}
	c.Call.Receive(sink)
}

func (c libCall) Play(src AudioSource) { c.Call.Play(src) }

func (c libCall) ReceiveVideo(sink VideoSink) {
	if sink == nil {
		c.Call.ReceiveVideo(nil)
		return
	}
	c.Call.ReceiveVideo(sink)
}

func (c libCall) SendVideo(accessUnit []byte, duration time.Duration) error {
	return c.Call.SendVideoWithDuration(accessUnit, duration)
}

func (c libCall) OnVideoState(fn func(VideoState)) {
	c.Call.OnVideoState(func(v meowcaller.VideoState) {
		fn(VideoState{Active: v.Active, Upgrade: v.Upgrade, Orientation: v.Orientation, State: videoStateName(v.Raw)})
	})
}

func (c libCall) Phase() Phase {
	switch c.State() {
	case meowcaller.CallPhaseCalling:
		return PhaseCalling
	case meowcaller.CallPhaseRinging:
		return PhaseRinging
	case meowcaller.CallPhaseConnecting:
		return PhaseConnecting
	case meowcaller.CallPhaseActive:
		return PhaseActive
	case meowcaller.CallPhaseEnded:
		return PhaseEnded
	}
	return PhaseOther
}

// State says why an instance does or does not have a working call engine.
type State string

const (
	// StateActive: the engine is attached and its hook into whatsmeow is installed.
	StateActive State = "active"
	// StateHookFailed: the engine was created but could not hook into whatsmeow, so
	// calls will not get media. Usually a whatsmeow update changed its internals.
	StateHookFailed State = "hook_failed"
	// StateBlockedProxy: the instance uses a proxy and call media would bypass it.
	StateBlockedProxy State = "blocked_by_proxy"
)

// Status is the engine state of one instance, reported in the runtime diagnostics.
type Status struct {
	State State  `json:"state"`
	Error string `json:"error,omitempty"`
	// ActiveCalls is how many calls of the instance are tracked right now.
	ActiveCalls int `json:"activeCalls"`
}

// Logger is the per-instance logger (*logger.Logger satisfies it).
type Logger interface {
	LogInfo(format string, args ...interface{})
	LogWarn(format string, args ...interface{})
	LogError(format string, args ...interface{})
	LogDebug(format string, args ...interface{})
}

type runtime struct {
	client *meowcaller.Client
	status Status
	log    Logger
	// dial places a call through the library; nil when there is no library client.
	dial func(ctx context.Context, target string, opts DialOptions) (Call, error)
}

const (
	// DefaultMaxConcurrent is how many calls one instance may have at the same time.
	DefaultMaxConcurrent = 4
	// DefaultRingTimeout is how long a call may stay unanswered before it is dropped.
	// It is longer than WhatsApp's own ring time, so it only catches calls whose end
	// never arrived.
	DefaultRingTimeout = 90 * time.Second
	// DefaultStreamGrace is how long a call without an audio stream is kept.
	DefaultStreamGrace = 10 * time.Second
	// DefaultDialsPerMinute bounds the calls an instance places: mass outgoing calls
	// are what gets an account flagged.
	DefaultDialsPerMinute = 6
)

// Notifier publishes a call lifecycle event of an instance (CallReady, CallEnded).
type Notifier func(instanceID, event string, data map[string]interface{})

// Options tune a Manager; a zero field takes its default.
type Options struct {
	MaxConcurrent int
	RingTimeout   time.Duration
	// StreamGrace is how long a running call waits for its audio stream to come back
	// before it is hung up.
	StreamGrace time.Duration
	// DialsPerMinute is how many calls an instance may place per minute.
	DialsPerMinute int
	// Dial replaces placing calls through the library (tests).
	Dial   DialFunc
	Notify Notifier
}

// Manager keeps one engine per instance and the calls each one has. Build it with
// NewManager; Status, Detach and the call lookups also work on a nil Manager (a
// service built without one has no engines).
type Manager struct {
	opts Options

	mu       sync.RWMutex
	runtimes map[string]*runtime
	calls    map[string]map[string]*Tracked // instance id -> call id -> call

	dialMu sync.Mutex
	dials  map[string][]time.Time // instance id -> when it placed calls, last minute
	now    func() time.Time
}

func NewManager(opts Options) *Manager {
	if opts.MaxConcurrent <= 0 {
		opts.MaxConcurrent = DefaultMaxConcurrent
	}
	if opts.RingTimeout <= 0 {
		opts.RingTimeout = DefaultRingTimeout
	}
	if opts.StreamGrace <= 0 {
		opts.StreamGrace = DefaultStreamGrace
	}
	if opts.DialsPerMinute <= 0 {
		opts.DialsPerMinute = DefaultDialsPerMinute
	}
	return &Manager{
		opts:     opts,
		runtimes: make(map[string]*runtime),
		calls:    make(map[string]map[string]*Tracked),
		dials:    make(map[string][]time.Time),
		now:      time.Now,
	}
}

// Attach gives the instance a call engine. It must run before cli.Connect(): the
// library installs its raw <call> stanza handling at construction and doing that on
// a connected client is a documented race. A previous engine of the same instance
// (a reconnect creates a new whatsmeow client) is replaced.
//
// It never fails the caller: whatever goes wrong is in the returned Status.
func (m *Manager) Attach(instanceID string, cli *whatsmeow.Client, proxied bool, log Logger) Status {
	if proxied {
		st := Status{State: StateBlockedProxy, Error: "call media is sent over UDP directly and would bypass the instance proxy"}
		log.LogWarn("[%s] Calls are not enabled for this instance: %s", instanceID, st.Error)
		m.store(instanceID, &runtime{status: st, log: log})
		return st
	}

	probe := &logBridge{instanceID: instanceID, log: log}
	probe.startProbing()
	rt := &runtime{status: Status{State: StateActive}, log: log}

	func() {
		defer func() {
			if r := recover(); r != nil {
				rt.status = Status{State: StateHookFailed, Error: fmt.Sprintf("call engine panicked while starting: %v", r)}
			}
		}()
		rt.client = meowcaller.NewClient(cli, meowcaller.WithLogger(zerolog.New(probe).Level(zerolog.InfoLevel)))
		rt.client.OnIncomingCall(func(c *meowcaller.Call) { m.onIncoming(instanceID, libCall{c}) })
		client := rt.client
		rt.dial = func(ctx context.Context, target string, opts DialOptions) (Call, error) {
			c, err := client.CallWithOptions(ctx, target, meowcaller.CallOptions{Video: opts.Video})
			if err != nil {
				return nil, err
			}
			return libCall{c}, nil
		}
	}()

	if failure := probe.stopProbing(); failure != "" && rt.status.State == StateActive {
		rt.status = Status{State: StateHookFailed, Error: failure}
	}

	if rt.status.State != StateActive {
		log.LogError("[%s] Call engine is not working: %s", instanceID, rt.status.Error)
	} else {
		log.LogInfo("[%s] Call engine attached", instanceID)
	}
	m.store(instanceID, rt)
	return rt.status
}

// Detach forgets the engine of an instance (its whatsmeow client is gone).
func (m *Manager) Detach(instanceID string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	delete(m.runtimes, instanceID)
	m.mu.Unlock()
	m.endAll(instanceID, "instance_stopped")
}

// Status returns the engine state of an instance; ok is false when the instance has
// no engine (calls are not enabled for it).
func (m *Manager) Status(instanceID string) (st Status, ok bool) {
	if m == nil {
		return Status{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	rt, ok := m.runtimes[instanceID]
	if !ok {
		return Status{}, false
	}
	st = rt.status
	st.ActiveCalls = len(m.calls[instanceID])
	return st, true
}

// store makes rt the engine of the instance. The calls of an engine it replaces
// belong to a client that no longer exists, so they end here.
func (m *Manager) store(instanceID string, rt *runtime) {
	m.mu.Lock()
	_, replaced := m.runtimes[instanceID]
	m.runtimes[instanceID] = rt
	m.mu.Unlock()
	if replaced {
		m.endAll(instanceID, "instance_stopped")
	}
}

// logOf is the instance logger, or one that discards.
func (m *Manager) logOf(instanceID string) Logger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rt, ok := m.runtimes[instanceID]; ok && rt.log != nil {
		return rt.log
	}
	return discardLogger{}
}

type discardLogger struct{}

func (discardLogger) LogInfo(string, ...interface{})  {}
func (discardLogger) LogWarn(string, ...interface{})  {}
func (discardLogger) LogError(string, ...interface{}) {}
func (discardLogger) LogDebug(string, ...interface{}) {}

// logBridge is the zerolog sink handed to the library: it forwards the library's log
// lines to the instance log and, while the engine is being constructed, remembers the
// first error line. That line is the only signal the library gives when its hook into
// whatsmeow could not be installed (it logs "raw call adapter is unavailable" and
// carries on).
type logBridge struct {
	instanceID string
	log        Logger

	mu       sync.Mutex
	probing  bool
	firstErr string
}

func (b *logBridge) startProbing() {
	b.mu.Lock()
	b.probing = true
	b.mu.Unlock()
}

// stopProbing ends the construction window and returns the first error seen in it.
func (b *logBridge) stopProbing() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probing = false
	return b.firstErr
}

func (b *logBridge) Write(p []byte) (int, error) { return len(p), nil }

func (b *logBridge) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	line := strings.TrimSpace(string(p))

	if level >= zerolog.ErrorLevel {
		b.mu.Lock()
		if b.probing && b.firstErr == "" {
			b.firstErr = describeLibraryError(line)
		}
		b.mu.Unlock()
	}

	switch {
	case level >= zerolog.ErrorLevel:
		b.log.LogError("[%s] call library: %s", b.instanceID, line)
	case level == zerolog.WarnLevel:
		b.log.LogWarn("[%s] call library: %s", b.instanceID, line)
	case level == zerolog.InfoLevel:
		b.log.LogInfo("[%s] call library: %s", b.instanceID, line)
	default:
		b.log.LogDebug("[%s] call library: %s", b.instanceID, line)
	}
	return len(p), nil
}

// describeLibraryError turns one zerolog JSON line into "message: error".
func describeLibraryError(line string) string {
	var f struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(line), &f); err != nil || (f.Message == "" && f.Error == "") {
		return line
	}
	switch {
	case f.Message == "":
		return f.Error
	case f.Error == "":
		return f.Message
	}
	return f.Message + ": " + f.Error
}
