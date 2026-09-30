package call_engine

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrStreamBusy: the call already has an audio stream.
var ErrStreamBusy = errors.New("the call already has an audio stream")

// StreamStats counts what one audio stream moved. Frames are the library's unit, not
// the client's: a client may send its audio in chunks of any size.
type StreamStats struct {
	ToClient          atomic.Uint64 // frames of the peer's audio delivered to the client
	FromClient        atomic.Uint64 // frames of the client's audio accepted for the peer
	DroppedToClient   atomic.Uint64 // peer audio dropped because the client read too slowly
	DroppedFromClient atomic.Uint64 // client audio dropped because it sent faster than real time for too long
}

// StreamInfo is the snapshot of StreamStats reported with a call.
type StreamInfo struct {
	Attached          bool   `json:"attached"`
	ToClient          uint64 `json:"toClient"`
	FromClient        uint64 `json:"fromClient"`
	DroppedToClient   uint64 `json:"droppedToClient"`
	DroppedFromClient uint64 `json:"droppedFromClient"`
}

// AttachStream connects the audio of a call to a consumer: sink receives the peer's
// audio, src is what gets sent to the peer. A call has at most one stream at a time.
// The returned detach must be called when the consumer is gone.
//
// A call that is running and loses its stream is not left silent forever: it is hung
// up when no stream comes back within the grace period (Options.StreamGrace), reported
// as "stream_closed". A call that has not been answered yet does not start that clock.
func (m *Manager) AttachStream(instanceID, callID string, sink AudioSink, src AudioSource, stats *StreamStats) (*Tracked, func(), error) {
	t, ok := m.Get(instanceID, callID)
	if !ok {
		return nil, nil, ErrCallNotFound
	}

	t.mu.Lock()
	if t.attached {
		t.mu.Unlock()
		return nil, nil, ErrStreamBusy
	}
	select {
	case <-t.done:
		t.mu.Unlock()
		return nil, nil, ErrCallNotFound
	default:
	}
	t.attached = true
	t.stats = stats
	if t.grace != nil {
		t.grace.Stop()
		t.grace = nil
	}
	t.mu.Unlock()

	t.call.Receive(sink)
	t.call.Play(src)

	var once sync.Once
	detach := func() { once.Do(func() { m.streamDetached(instanceID, t) }) }
	return t, detach, nil
}

func (m *Manager) streamDetached(instanceID string, t *Tracked) {
	t.call.Receive(nil)

	t.mu.Lock()
	t.attached = false
	select {
	case <-t.done:
		t.mu.Unlock()
		return
	default:
	}
	switch t.call.Phase() {
	case PhaseConnecting, PhaseActive:
		if t.grace != nil {
			t.grace.Stop()
		}
		t.grace = time.AfterFunc(m.opts.StreamGrace, func() { m.streamGraceExpired(instanceID, t) })
	}
	t.mu.Unlock()
}

func (m *Manager) streamGraceExpired(instanceID string, t *Tracked) {
	t.mu.Lock()
	stale := t.attached
	t.mu.Unlock()
	select {
	case <-t.done:
		return
	default:
	}
	if stale {
		return // a stream came back after all
	}

	m.logOf(instanceID).LogWarn("[%s] Hanging up call %s: no audio stream for %s", instanceID, t.call.ID(), m.opts.StreamGrace)
	t.setOverride("stream_closed")
	_ = t.call.Hangup()
	m.finish(instanceID, t, "stream_closed")
}
