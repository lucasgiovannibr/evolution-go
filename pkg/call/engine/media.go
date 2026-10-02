package call_engine

import (
	"time"
)

// ReasonMediaStalled is the reason of a call hung up because the peer's audio stopped
// arriving (only when Options.MediaStallHangup is on).
const ReasonMediaStalled = "media_stalled"

// DefaultMediaStall is how long an active call with a stream may go without audio from
// the peer before it is reported as stalled.
const DefaultMediaStall = 15 * time.Second

// activitySink passes the peer's audio on and notes when it last arrived. It is what
// the media watchdog looks at: the library has no public counter of received packets.
type activitySink struct {
	AudioSink
	t   *Tracked
	now func() time.Time
}

func (s activitySink) WriteFrame(frame []float32) error {
	if len(frame) > 0 {
		s.t.lastAudio.set(s.now())
	}
	return s.AudioSink.WriteFrame(frame)
}

// watchMedia checks the call every so often until it ends.
func (m *Manager) watchMedia(instanceID string, t *Tracked) {
	every := min(max(m.opts.MediaStall/4, 10*time.Millisecond), 5*time.Second)
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-t.done:
			return
		case <-tick.C:
			m.checkMedia(instanceID, t)
		}
	}
}

// checkMedia reports a call that is active, has a stream, and has received no audio from
// the peer for Options.MediaStall, and reports it again when the audio comes back.
//
// Known cases that make a call go silent while WhatsApp still shows it as running (see
// the open issues of the call library) are the reason for this: the consumer of the
// stream otherwise just hears nothing. A call without a stream is not watched, there is
// nobody to tell and the stream grace already ends it. A peer that muted itself may
// legitimately send nothing (the codec can stop transmitting during silence), which is
// why hanging up is opt-in and the default is only to say so.
func (m *Manager) checkMedia(instanceID string, t *Tracked) {
	select {
	case <-t.done:
		return
	default:
	}

	t.mu.Lock()
	attached, attachedAt := t.attached, t.attachedAt
	t.mu.Unlock()

	if !attached || t.call.Phase() != PhaseActive {
		t.stalled.Store(false) // nothing to compare until a stream is back and the call is up
		return
	}

	baseline := latest(attachedAt, t.readyAt.get(), t.lastAudio.get())
	idle := m.now().Sub(baseline)

	switch {
	case idle >= m.opts.MediaStall:
		if !t.stalled.CompareAndSwap(false, true) {
			return
		}
		m.metrics.stalls.Inc()
		m.logOf(instanceID).LogWarn("[%s] Call %s has received no audio from the peer for %s", instanceID, t.call.ID(), idle.Round(time.Second))
		data := t.eventData()
		data["idleSeconds"] = int(idle.Seconds())
		data["hangup"] = m.opts.MediaStallHangup
		m.notify(instanceID, "CallMediaStalled", data)

		if m.opts.MediaStallHangup {
			t.setOverride(ReasonMediaStalled)
			_ = t.call.Hangup()
			m.finish(instanceID, t, ReasonMediaStalled)
		}

	case t.stalled.CompareAndSwap(true, false):
		m.notify(instanceID, "CallMediaResumed", t.eventData())
	}
}

// latest is the most recent of the times, ignoring the zero ones.
func latest(times ...time.Time) time.Time {
	var out time.Time
	for _, t := range times {
		if t.After(out) {
			out = t
		}
	}
	return out
}
