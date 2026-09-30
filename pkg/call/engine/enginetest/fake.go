// Package enginetest holds a fake call for tests of the code that sits on top of the
// call engine, so they need neither the call library nor a WhatsApp connection.
package enginetest

import (
	"sync"

	call_engine "github.com/evolution-foundation/evolution-go/pkg/call/engine"
	"go.mau.fi/whatsmeow/types"
)

// Fake behaves like the library's call where it matters: Answer moves it to
// connecting, Reject and Hangup end it locally (firing OnEnd) before they try the
// network, and a sink or source attached to it stays until it is replaced.
type Fake struct {
	id string

	mu       sync.Mutex
	phase    call_engine.Phase
	video    bool
	onEnd    func(string)
	onReady  func()
	sink     call_engine.AudioSink
	src      call_engine.AudioSource
	answered int
	ended    int
}

func NewFake(id string) *Fake { return &Fake{id: id, phase: call_engine.PhaseRinging} }

func (f *Fake) ID() string      { return f.id }
func (f *Fake) IsVideo() bool   { return f.video }
func (f *Fake) Peer() types.JID { return types.NewJID("5511999990000", types.DefaultUserServer) }

func (f *Fake) Phase() call_engine.Phase {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.phase
}

func (f *Fake) SetPhase(p call_engine.Phase) {
	f.mu.Lock()
	f.phase = p
	f.mu.Unlock()
}

func (f *Fake) SetVideo(v bool) { f.mu.Lock(); f.video = v; f.mu.Unlock() }

func (f *Fake) OnReady(fn func())               { f.mu.Lock(); f.onReady = fn; f.mu.Unlock() }
func (f *Fake) OnEnd(fn func(string))           { f.mu.Lock(); f.onEnd = fn; f.mu.Unlock() }
func (f *Fake) Receive(s call_engine.AudioSink) { f.mu.Lock(); f.sink = s; f.mu.Unlock() }
func (f *Fake) Play(s call_engine.AudioSource)  { f.mu.Lock(); f.src = s; f.mu.Unlock() }

func (f *Fake) Answer() error {
	f.mu.Lock()
	f.answered++
	f.phase = call_engine.PhaseConnecting
	f.mu.Unlock()
	return nil
}

func (f *Fake) Reject() error { f.End("rejected"); return nil }
func (f *Fake) Hangup() error { f.End("hangup"); return nil }

// End ends the call the way the peer hanging up would.
func (f *Fake) End(reason string) {
	f.mu.Lock()
	f.phase = call_engine.PhaseEnded
	f.ended++
	fn := f.onEnd
	f.mu.Unlock()
	if fn != nil {
		fn(reason)
	}
}

// Sink is the sink the code under test attached with Receive.
func (f *Fake) Sink() call_engine.AudioSink {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sink
}

// Source is the source the code under test attached with Play.
func (f *Fake) Source() call_engine.AudioSource {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.src
}

func (f *Fake) Answered() int { f.mu.Lock(); defer f.mu.Unlock(); return f.answered }
