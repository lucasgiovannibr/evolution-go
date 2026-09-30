package call_engine

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
)

type recordingLogger struct {
	mu    sync.Mutex
	lines []string
}

func (r *recordingLogger) add(level, format string, args ...interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, level+" "+strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func (r *recordingLogger) LogInfo(f string, a ...interface{})  { r.add("INFO", f, a...) }
func (r *recordingLogger) LogWarn(f string, a ...interface{})  { r.add("WARN", f, a...) }
func (r *recordingLogger) LogError(f string, a ...interface{}) { r.add("ERROR", f, a...) }
func (r *recordingLogger) LogDebug(f string, a ...interface{}) { r.add("DEBUG", f, a...) }

func (r *recordingLogger) has(level string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.lines {
		if strings.HasPrefix(l, level+" ") {
			return true
		}
	}
	return false
}

// This is the guard for the reflection the call library does on whatsmeow's private
// node handler map: when a whatsmeow bump changes that layout the library only logs an
// error and carries on, so nothing else would notice that calls stopped working. If
// this test fails after a dependency update, calls need a new library revision (or a
// whatsmeow revision the library still fits) before the update can ship.
func TestAttachInstallsTheHookOnThePinnedWhatsmeow(t *testing.T) {
	m := NewManager()
	log := &recordingLogger{}
	cli := whatsmeow.NewClient(&store.Device{}, nil)

	st := m.Attach("inst-1", cli, false, log)

	if st.State != StateActive {
		t.Fatalf("state = %q (%s), want %q: the call library no longer fits the pinned whatsmeow", st.State, st.Error, StateActive)
	}
	if log.has("ERROR") {
		t.Fatalf("attaching logged errors: %v", log.lines)
	}
	got, ok := m.Status("inst-1")
	if !ok || got != st {
		t.Fatalf("Status = %+v, %v; want %+v, true", got, ok, st)
	}
}

func TestAttachRefusesAProxiedInstance(t *testing.T) {
	m := NewManager()
	log := &recordingLogger{}

	st := m.Attach("inst-1", nil, true, log)

	if st.State != StateBlockedProxy {
		t.Fatalf("state = %q, want %q", st.State, StateBlockedProxy)
	}
	if st.Error == "" {
		t.Fatal("a blocked instance must say why")
	}
	if got, ok := m.Status("inst-1"); !ok || got.State != StateBlockedProxy {
		t.Fatalf("Status = %+v, %v", got, ok)
	}
}

func TestStatusOfAnInstanceWithoutAnEngine(t *testing.T) {
	m := NewManager()
	if _, ok := m.Status("nobody"); ok {
		t.Fatal("an instance that never attached must have no status")
	}
}

func TestDetachForgetsTheEngine(t *testing.T) {
	m := NewManager()
	m.Attach("inst-1", nil, true, &recordingLogger{})
	m.Detach("inst-1")
	if _, ok := m.Status("inst-1"); ok {
		t.Fatal("status still there after Detach")
	}
	m.Detach("inst-1") // detaching twice is harmless
}

func TestAttachReplacesThePreviousEngine(t *testing.T) {
	m := NewManager()
	log := &recordingLogger{}
	m.Attach("inst-1", nil, true, log)

	st := m.Attach("inst-1", whatsmeow.NewClient(&store.Device{}, nil), false, log)

	if got, _ := m.Status("inst-1"); got != st || got.State != StateActive {
		t.Fatalf("Status = %+v, want the new engine %+v", got, st)
	}
}

func TestLogBridgeKeepsTheFirstErrorOfTheConstructionWindow(t *testing.T) {
	log := &recordingLogger{}
	b := &logBridge{instanceID: "inst-1", log: log}
	b.startProbing()

	b.WriteLevel(zerolog.InfoLevel, []byte(`{"level":"info","message":"fine"}`))
	b.WriteLevel(zerolog.ErrorLevel, []byte(`{"level":"error","error":"layout changed","message":"raw call adapter is unavailable"}`))
	b.WriteLevel(zerolog.ErrorLevel, []byte(`{"level":"error","message":"second"}`))

	got := b.stopProbing()
	if got != "raw call adapter is unavailable: layout changed" {
		t.Fatalf("first error = %q", got)
	}
}

func TestLogBridgeIgnoresErrorsAfterTheConstructionWindow(t *testing.T) {
	log := &recordingLogger{}
	b := &logBridge{instanceID: "inst-1", log: log}
	b.startProbing()
	if got := b.stopProbing(); got != "" {
		t.Fatalf("clean construction reported %q", got)
	}

	b.WriteLevel(zerolog.ErrorLevel, []byte(`{"level":"error","message":"call failed"}`))

	if got := b.stopProbing(); got != "" {
		t.Fatalf("an error during a call must not turn the engine into hook_failed, got %q", got)
	}
	if !log.has("ERROR") {
		t.Fatal("the error was not forwarded to the instance log")
	}
}

func TestDescribeLibraryError(t *testing.T) {
	cases := map[string]string{
		`{"message":"m","error":"e"}`: "m: e",
		`{"message":"m"}`:             "m",
		`{"error":"e"}`:               "e",
		`not json`:                    "not json",
		`{}`:                          "{}",
	}
	for in, want := range cases {
		if got := describeLibraryError(in); got != want {
			t.Errorf("describeLibraryError(%s) = %q, want %q", in, got, want)
		}
	}
}
