package call_engine

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrStreamBusy: the call already has a stream.
var ErrStreamBusy = errors.New("the call already has an audio stream")

// StreamStats counts what one stream moved. Audio frames are the library's unit, not
// the client's: a client may send its audio in chunks of any size. Video counts access
// units.
type StreamStats struct {
	ToClient          atomic.Uint64 // frames of the peer's audio delivered to the client
	FromClient        atomic.Uint64 // frames of the client's audio accepted for the peer
	DroppedToClient   atomic.Uint64 // peer audio dropped because the client read too slowly
	DroppedFromClient atomic.Uint64 // client audio dropped because it sent faster than real time for too long

	VideoToClient          atomic.Uint64 // access units of the peer's video delivered to the client
	VideoFromClient        atomic.Uint64 // access units of the client's video sent to the peer
	VideoDroppedToClient   atomic.Uint64 // peer video dropped (the client read too slowly, or the library refused it)
	VideoDroppedFromClient atomic.Uint64 // client video the library refused (no video media up yet, gated)
	KeyframeRequests       atomic.Uint64 // times WhatsApp asked for a keyframe
}

// StreamInfo is the snapshot of StreamStats reported with a call.
type StreamInfo struct {
	Attached          bool   `json:"attached"`
	ToClient          uint64 `json:"toClient"`
	FromClient        uint64 `json:"fromClient"`
	DroppedToClient   uint64 `json:"droppedToClient"`
	DroppedFromClient uint64 `json:"droppedFromClient"`

	VideoToClient          uint64 `json:"videoToClient"`
	VideoFromClient        uint64 `json:"videoFromClient"`
	VideoDroppedToClient   uint64 `json:"videoDroppedToClient"`
	VideoDroppedFromClient uint64 `json:"videoDroppedFromClient"`
	KeyframeRequests       uint64 `json:"keyframeRequests"`
}

// Endpoints are what a stream connects to a call.
type Endpoints struct {
	// Sink receives the peer's audio, Source is the audio sent to the peer.
	Sink   AudioSink
	Source AudioSource
	// Video receives the peer's video. Nil means the stream does not carry video: the
	// library then discards the peer's video.
	Video VideoSink
	// OnVideoState is told what the peer reports about its video; OnKeyframeRequest,
	// that WhatsApp needs a keyframe from us. Both are called from the library's
	// goroutine and must not block.
	OnVideoState      func(VideoState)
	OnKeyframeRequest func()
}

// AttachStream connects a call to a consumer. A call has at most one stream at a time.
// The returned detach must be called when the consumer is gone.
//
// A call that is running and loses its stream is not left silent forever: it is hung
// up when no stream comes back within the grace period (Options.StreamGrace), reported
// as "stream_closed". A call that has not been answered yet does not start that clock.
func (m *Manager) AttachStream(instanceID, callID string, ep Endpoints, stats *StreamStats) (*Tracked, func(), error) {
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
	t.onVideoState = ep.OnVideoState
	t.onKeyframe = ep.OnKeyframeRequest
	if t.grace != nil {
		t.grace.Stop()
		t.grace = nil
	}
	t.mu.Unlock()

	t.call.Receive(ep.Sink)
	t.call.Play(ep.Source)
	if ep.Video != nil {
		t.call.ReceiveVideo(ep.Video)
	}

	var once sync.Once
	detach := func() { once.Do(func() { m.streamDetached(instanceID, t, ep.Video != nil) }) }
	return t, detach, nil
}

func (m *Manager) streamDetached(instanceID string, t *Tracked, hadVideo bool) {
	t.call.Receive(nil)
	if hadVideo {
		t.call.ReceiveVideo(nil)
	}

	t.mu.Lock()
	t.attached = false
	t.onVideoState = nil
	t.onKeyframe = nil
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

// videoStateChanged is called by the library when the peer reports its video state.
// It is kept, published as an event and handed to the stream that is attached.
func (m *Manager) videoStateChanged(instanceID string, t *Tracked, v VideoState) {
	if v.Active || v.State == VideoStateUpgradeAccepted {
		t.hadVideo.Store(true)
	}
	t.mu.Lock()
	t.peerVideo = &v
	fn := t.onVideoState
	t.mu.Unlock()

	data := t.eventData()
	data["active"] = v.Active
	data["upgrade"] = v.Upgrade
	data["orientation"] = v.Orientation
	data["state"] = v.State
	data["stateCode"] = v.StateCode
	m.notify(instanceID, "CallVideoState", data)

	if fn != nil {
		fn(v)
	}
}

// keyframeRequested is called by the library when WhatsApp asks for a keyframe.
func (t *Tracked) keyframeRequested() {
	t.mu.Lock()
	fn := t.onKeyframe
	stats := t.stats
	t.mu.Unlock()

	if stats != nil {
		stats.KeyframeRequests.Add(1)
	}
	if fn != nil {
		fn()
	}
}
