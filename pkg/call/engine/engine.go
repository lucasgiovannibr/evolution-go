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
package call_engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/purpshell/meowcaller"
	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// Call is what the rest of the code may do with a live call.
type Call interface {
	ID() string
	Peer() types.JID
	Answer() error
	Reject() error
	Hangup() error
}

// *meowcaller.Call already has exactly these methods.
var _ Call = (*meowcaller.Call)(nil)

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
}

// Manager keeps one engine per instance. Build it with NewManager; Status and Detach
// also work on a nil Manager (a service built without one has no engines).
type Manager struct {
	mu       sync.RWMutex
	runtimes map[string]*runtime
}

func NewManager() *Manager {
	return &Manager{runtimes: make(map[string]*runtime)}
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
		m.store(instanceID, &runtime{status: st})
		return st
	}

	probe := &logBridge{instanceID: instanceID, log: log}
	probe.startProbing()
	rt := &runtime{status: Status{State: StateActive}}

	func() {
		defer func() {
			if r := recover(); r != nil {
				rt.status = Status{State: StateHookFailed, Error: fmt.Sprintf("call engine panicked while starting: %v", r)}
			}
		}()
		rt.client = meowcaller.NewClient(cli, meowcaller.WithLogger(zerolog.New(probe).Level(zerolog.InfoLevel)))
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
	return rt.status, true
}

func (m *Manager) store(instanceID string, rt *runtime) {
	m.mu.Lock()
	m.runtimes[instanceID] = rt
	m.mu.Unlock()
}

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
