package call_engine

import "time"

// What became of a call, as a person would put it. See Classify.
const (
	OutcomeAnswered   = "answered"   // the media came up: there was a conversation
	OutcomeMissed     = "missed"     // an incoming call nobody answered
	OutcomeRejected   = "rejected"   // declined (by this side, or by the peer on an outgoing call)
	OutcomeCancelled  = "cancelled"  // an outgoing call this side hung up before it was answered
	OutcomeUnanswered = "unanswered" // an outgoing call that was not answered
	OutcomeBusy       = "busy"       // an incoming call refused because the instance had too many
	OutcomeFailed     = "failed"     // ended by an error or a server code
)

// Outcomes lists every value of Record.Outcome.
var Outcomes = []string{OutcomeAnswered, OutcomeMissed, OutcomeRejected, OutcomeCancelled, OutcomeUnanswered, OutcomeBusy, OutcomeFailed}

// Record is what the engine knows about a call once it is over: no audio, no content,
// only who, when and how it ended.
type Record struct {
	InstanceID string
	CallID     string
	// Peer is the other side as WhatsApp reports it, often a @lid.
	Peer      string
	Direction Direction
	// Video: the call had video at some point.
	Video   bool
	Outcome string
	// Reason is the technical reason the call ended (see Tracked.Reason).
	Reason    string
	StartedAt time.Time
	// AnsweredAt is when the media came up; zero when the call was never answered.
	AnsweredAt time.Time
	EndedAt    time.Time
}

// Talk is how long the conversation lasted: from the media being ready to the end.
func (r Record) Talk() time.Duration {
	if r.AnsweredAt.IsZero() || r.EndedAt.Before(r.AnsweredAt) {
		return 0
	}
	return r.EndedAt.Sub(r.AnsweredAt)
}

// Ring is how long the call rang: from its start to its being answered, or to its end.
func (r Record) Ring() time.Duration {
	until := r.EndedAt
	if !r.AnsweredAt.IsZero() {
		until = r.AnsweredAt
	}
	if until.Before(r.StartedAt) {
		return 0
	}
	return until.Sub(r.StartedAt)
}

// Classify says what became of a call from its direction, whether the media ever came
// up, and the reason it ended.
//
// WhatsApp sends no reason when the other side hangs up (see ReasonPeerHangup), which
// before the call is answered means "missed" on an incoming call and "not answered" on an
// outgoing one. What the library reports when an outgoing call is declined by the peer
// has not been observed on a device: it is assumed to read "rejected", and anything
// unknown is reported as failed rather than guessed.
func Classify(dir Direction, answered bool, reason string) string {
	if answered {
		return OutcomeAnswered
	}
	switch reason {
	case "rejected_busy":
		return OutcomeBusy
	case "rejected":
		return OutcomeRejected
	case "hangup":
		return OutcomeCancelled
	case ReasonPeerHangup, "ring_timeout":
		if dir == Incoming {
			return OutcomeMissed
		}
		return OutcomeUnanswered
	}
	return OutcomeFailed
}

// Finished is told about every call as it ends (see Manager.SetOnFinished).
type Finished func(rec Record)

// SetOnFinished sets what is told about every call as it ends, replacing the previous
// one; nil stops it. It runs on whatever goroutine ended the call, possibly the
// library's, so it must hand the record over and return.
func (m *Manager) SetOnFinished(fn Finished) {
	if m == nil {
		return
	}
	if fn == nil {
		m.onFinished.Store(nil)
		return
	}
	m.onFinished.Store(&fn)
}

// record builds the Record of a call that has just ended.
func (m *Manager) record(instanceID string, t *Tracked, reason string) Record {
	answeredAt := t.readyAt.get()
	return Record{
		InstanceID: instanceID,
		CallID:     t.call.ID(),
		Peer:       t.call.Peer().String(),
		Direction:  t.direction,
		Video:      t.hadVideo.Load(),
		Outcome:    Classify(t.direction, !answeredAt.IsZero(), reason),
		Reason:     reason,
		StartedAt:  t.startedAt,
		AnsweredAt: answeredAt,
		EndedAt:    m.now(),
	}
}

// finished hands the record of a call to whoever asked for it. A failing consumer must
// not take the library's goroutine down with it.
func (m *Manager) finished(instanceID string, t *Tracked, reason string) {
	fn := m.onFinished.Load()
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			m.logOf(instanceID).LogError("[%s] Recording call %s panicked: %v", instanceID, t.call.ID(), r)
		}
	}()
	(*fn)(m.record(instanceID, t, reason))
}

// finishedUntracked records a call that was turned away before it was tracked (the
// instance was over its limit): it has a start and an end at the same moment, and was
// never answered.
func (m *Manager) finishedUntracked(instanceID string, c Call, reason string) {
	fn := m.onFinished.Load()
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			m.logOf(instanceID).LogError("[%s] Recording call %s panicked: %v", instanceID, c.ID(), r)
		}
	}()
	now := m.now()
	(*fn)(Record{
		InstanceID: instanceID,
		CallID:     c.ID(),
		Peer:       c.Peer().String(),
		Direction:  Incoming,
		Video:      c.IsVideo(),
		Outcome:    Classify(Incoming, false, reason),
		Reason:     reason,
		StartedAt:  now,
		EndedAt:    now,
	})
}
