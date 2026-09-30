package call_engine

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// Direction is who started the call.
type Direction string

const (
	Incoming Direction = "incoming"
	Outgoing Direction = "outgoing"
)

// ErrTooManyCalls: the instance already has as many calls as it may have.
var ErrTooManyCalls = errors.New("too many simultaneous calls on this instance")

// Info is a snapshot of one tracked call.
type Info struct {
	CallID    string    `json:"callId"`
	Peer      string    `json:"peer"`
	Direction Direction `json:"direction"`
	Phase     Phase     `json:"phase"`
	Video     bool      `json:"video"`
	StartedAt time.Time `json:"startedAt"`
	// Stream describes the audio stream of the call; absent when none ever attached.
	Stream *StreamInfo `json:"stream,omitempty"`
}

// Tracked is a call the Manager follows from its start until it ends, whoever ends it
// (the peer, this project, the ring timeout or the instance going away).
type Tracked struct {
	call      Call
	direction Direction
	startedAt time.Time

	timer   *time.Timer
	done    chan struct{}
	endOnce sync.Once

	// act serialises answer/hangup so two requests cannot both act on the same phase.
	act sync.Mutex

	mu       sync.Mutex
	override string // reason to report instead of the library's, when this project ended the call
	reason   string
	stats    *StreamStats // of the last audio stream that attached
	attached bool         // an audio stream is attached right now
	grace    *time.Timer  // hangs the call up when its stream does not come back
}

// Call is the underlying call.
func (t *Tracked) Call() Call { return t.call }

// Done is closed when the call has ended.
func (t *Tracked) Done() <-chan struct{} { return t.done }

// Reason is why the call ended, empty while it is running.
func (t *Tracked) Reason() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.reason
}

func (t *Tracked) Info() Info {
	info := Info{
		CallID:    t.call.ID(),
		Peer:      t.call.Peer().String(),
		Direction: t.direction,
		Phase:     t.call.Phase(),
		Video:     t.call.IsVideo(),
		StartedAt: t.startedAt,
	}
	t.mu.Lock()
	if t.stats != nil {
		info.Stream = &StreamInfo{
			Attached:          t.attached,
			ToClient:          t.stats.ToClient.Load(),
			FromClient:        t.stats.FromClient.Load(),
			DroppedToClient:   t.stats.DroppedToClient.Load(),
			DroppedFromClient: t.stats.DroppedFromClient.Load(),
		}
	}
	t.mu.Unlock()
	return info
}

func (t *Tracked) eventData() map[string]interface{} {
	return map[string]interface{}{
		"callId":    t.call.ID(),
		"peer":      t.call.Peer().String(),
		"direction": string(t.direction),
		"video":     t.call.IsVideo(),
	}
}

func (t *Tracked) setOverride(reason string) {
	t.mu.Lock()
	if t.override == "" {
		t.override = reason
	}
	t.mu.Unlock()
}

// Track starts following a call. The Manager takes over the call's OnReady and OnEnd.
// Tracking a call that is already tracked returns the existing entry.
func (m *Manager) Track(instanceID string, c Call, dir Direction) (*Tracked, error) {
	m.mu.Lock()
	per := m.calls[instanceID]
	if existing, ok := per[c.ID()]; ok {
		m.mu.Unlock()
		return existing, nil
	}
	if len(per) >= m.opts.MaxConcurrent {
		m.mu.Unlock()
		return nil, ErrTooManyCalls
	}
	t := &Tracked{call: c, direction: dir, startedAt: time.Now(), done: make(chan struct{})}
	t.timer = time.AfterFunc(m.opts.RingTimeout, func() { m.ringExpired(instanceID, t) })
	if per == nil {
		per = make(map[string]*Tracked)
		m.calls[instanceID] = per
	}
	per[c.ID()] = t
	m.mu.Unlock()

	// Registered after the call is in the map, so an end that arrives right away finds
	// something to remove.
	c.OnReady(func() { m.notify(instanceID, "CallReady", t.eventData()) })
	c.OnEnd(func(reason string) { m.finish(instanceID, t, reason) })
	if c.Phase() == PhaseEnded {
		m.finish(instanceID, t, "ended")
	}
	return t, nil
}

// Get returns a tracked call of the instance.
func (m *Manager) Get(instanceID, callID string) (*Tracked, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.calls[instanceID][callID]
	return t, ok
}

// List returns the tracked calls of the instance, oldest first.
func (m *Manager) List(instanceID string) []Info {
	if m == nil {
		return []Info{}
	}
	m.mu.RLock()
	tracked := make([]*Tracked, 0, len(m.calls[instanceID]))
	for _, t := range m.calls[instanceID] {
		tracked = append(tracked, t)
	}
	m.mu.RUnlock()

	sort.Slice(tracked, func(i, j int) bool { return tracked[i].startedAt.Before(tracked[j].startedAt) })
	out := make([]Info, 0, len(tracked))
	for _, t := range tracked {
		out = append(out, t.Info())
	}
	return out
}

// Reject declines a tracked incoming call. tracked is false when the Manager does not
// know the call (then nothing was sent and the caller can fall back to whatsmeow).
// The call is over locally even when sending the rejection fails, which is why it is
// forgotten either way and err only says the peer may not have been told.
func (m *Manager) Reject(instanceID, callID string) (tracked bool, err error) {
	t, ok := m.Get(instanceID, callID)
	if !ok {
		return false, nil
	}
	err = t.call.Reject()
	m.finish(instanceID, t, "rejected")
	return true, err
}

// onIncoming is called by the library for every incoming offer.
func (m *Manager) onIncoming(instanceID string, c Call) {
	_, err := m.Track(instanceID, c, Incoming)
	if err == nil {
		return
	}
	if errors.Is(err, ErrTooManyCalls) {
		m.logOf(instanceID).LogWarn("[%s] Rejecting call %s: the instance already has %d calls", instanceID, c.ID(), m.opts.MaxConcurrent)
		_ = c.Reject()
		data := map[string]interface{}{
			"callId": c.ID(), "peer": c.Peer().String(), "direction": string(Incoming), "video": c.IsVideo(),
			"reason": "rejected_busy", "durationSeconds": 0,
		}
		m.notify(instanceID, "CallEnded", data)
	}
}

// finish is the one place a call stops being tracked. It runs once per call, whichever
// of the paths gets there first.
func (m *Manager) finish(instanceID string, t *Tracked, libReason string) {
	t.endOnce.Do(func() {
		m.mu.Lock()
		if per := m.calls[instanceID]; per[t.call.ID()] == t {
			delete(per, t.call.ID())
			if len(per) == 0 {
				delete(m.calls, instanceID)
			}
		}
		m.mu.Unlock()

		t.timer.Stop()

		t.mu.Lock()
		reason := libReason
		if t.override != "" {
			reason = t.override
		}
		t.reason = reason
		if t.grace != nil {
			t.grace.Stop()
		}
		t.mu.Unlock()
		close(t.done)

		data := t.eventData()
		data["reason"] = reason
		data["durationSeconds"] = int(time.Since(t.startedAt).Seconds())
		m.notify(instanceID, "CallEnded", data)
	})
}

// ringExpired drops a call nobody answered. Calls already answered are left alone: how
// long a conversation lasts is not this timer's business.
func (m *Manager) ringExpired(instanceID string, t *Tracked) {
	switch t.call.Phase() {
	case PhaseCalling, PhaseRinging:
	default:
		return
	}
	m.logOf(instanceID).LogWarn("[%s] Dropping call %s: still unanswered after %s", instanceID, t.call.ID(), m.opts.RingTimeout)
	t.setOverride("ring_timeout")
	if t.direction == Incoming {
		_ = t.call.Reject()
	} else {
		_ = t.call.Hangup()
	}
	m.finish(instanceID, t, "ring_timeout")
}

// endAll ends every call of the instance: its client is gone, so nobody could end them
// later. The calls are forgotten right away; hanging up runs in the background because
// the library tears the media down first and only then tries to tell the peer, which
// may wait on a connection that no longer exists.
func (m *Manager) endAll(instanceID, reason string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	all := make([]*Tracked, 0, len(m.calls[instanceID]))
	for _, t := range m.calls[instanceID] {
		all = append(all, t)
	}
	m.mu.Unlock()

	for _, t := range all {
		t.setOverride(reason)
		m.finish(instanceID, t, reason)
		go func(c Call) { _ = c.Hangup() }(t.call)
	}
}

func (m *Manager) notify(instanceID, event string, data map[string]interface{}) {
	if m.opts.Notify == nil {
		return
	}
	// A failing subscriber must not take the library's goroutine down with it.
	defer func() {
		if r := recover(); r != nil {
			m.logOf(instanceID).LogError("[%s] Publishing %s panicked: %v", instanceID, event, r)
		}
	}()
	m.opts.Notify(instanceID, event, data)
}
