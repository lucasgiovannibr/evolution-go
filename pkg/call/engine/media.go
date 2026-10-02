package call_engine

import (
	"math"
	"time"
)

// ReasonMediaStalled is the reason of a call hung up because the peer's audio stopped
// arriving (only when Options.MediaStallHangup is on).
const ReasonMediaStalled = "media_stalled"

// The reasons of a call hung up because it ran past a limit (Options.MaxDuration,
// Options.SilenceTimeout).
const (
	ReasonMaxDuration    = "max_duration"
	ReasonSilenceTimeout = "silence_timeout"
)

// voiceRMS is the level, as the root mean square of a frame (full scale is 1), above
// which a frame counts as somebody making sound. A peer that is silent or muted still
// sends two or three frames a second of comfort noise, which sits far below this (about
// 0.0001), and an ordinary voice far above (0.03 and up); it is set low so that a quiet
// speaker is never taken for silence.
const voiceRMS = 0.003

func frameRMS(frame []float32) float64 {
	var sum float64
	for _, v := range frame {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum / float64(len(frame)))
}

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
		now := s.now()
		s.t.lastAudio.set(now)
		if frameRMS(frame) > voiceRMS {
			s.t.lastVoice.set(now)
		}
	}
	return s.AudioSink.WriteFrame(frame)
}

// activitySource passes the client's audio on and notes when it last carried sound, for
// the silence timeout: a call is idle only when neither side is making any.
type activitySource struct {
	AudioSource
	t   *Tracked
	now func() time.Time
}

func (s activitySource) ReadFrame() ([]float32, error) {
	frame, err := s.AudioSource.ReadFrame()
	if len(frame) > 0 && frameRMS(frame) > voiceRMS {
		s.t.lastVoice.set(s.now())
	}
	return frame, err
}

// watching says whether any of the checks that run on a timer is on.
func (m *Manager) watching() bool {
	return m.opts.MediaStall > 0 || m.opts.MaxDuration > 0 || m.opts.SilenceTimeout > 0
}

// watchCall runs the timed checks of a call every so often until it ends.
func (m *Manager) watchCall(instanceID string, t *Tracked) {
	// often enough to act within a quarter of the shortest limit, never busier than
	// every 10 ms or lazier than every 5 s
	shortest := time.Duration(math.MaxInt64)
	for _, d := range []time.Duration{m.opts.MediaStall, m.opts.MaxDuration, m.opts.SilenceTimeout} {
		if d > 0 && d < shortest {
			shortest = d
		}
	}
	tick := time.NewTicker(min(max(shortest/4, 10*time.Millisecond), 5*time.Second))
	defer tick.Stop()
	for {
		select {
		case <-t.done:
			return
		case <-tick.C:
			if m.opts.MediaStall > 0 {
				m.checkMedia(instanceID, t)
			}
			m.checkLimits(instanceID, t)
		}
	}
}

// checkLimits hangs up a call that has run past Options.MaxDuration, or whose two sides
// have both been silent for Options.SilenceTimeout. Both are off unless asked for: how
// long a conversation lasts is the operator's business, and these exist for the call
// that nobody is attending (an agent that crashed without hanging up, a phone left on
// the table) and so keeps an account's call slot, and a billed model session, forever.
//
// The maximum duration counts from the media being ready and needs no stream. The
// silence timeout, like the media watchdog, only applies to a call with a stream: with
// none, the stream grace already ends it.
func (m *Manager) checkLimits(instanceID string, t *Tracked) {
	select {
	case <-t.done:
		return
	default:
	}
	if t.call.Phase() != PhaseActive {
		return
	}
	now := m.now()

	if m.opts.MaxDuration > 0 {
		if readyAt := t.readyAt.get(); !readyAt.IsZero() && now.Sub(readyAt) >= m.opts.MaxDuration {
			m.hangUpFor(instanceID, t, ReasonMaxDuration, "it has lasted "+m.opts.MaxDuration.String())
			return
		}
	}

	if m.opts.SilenceTimeout > 0 {
		t.mu.Lock()
		attached, attachedAt := t.attached, t.attachedAt
		t.mu.Unlock()
		if !attached {
			return
		}
		if idle := now.Sub(latest(attachedAt, t.readyAt.get(), t.lastVoice.get())); idle >= m.opts.SilenceTimeout {
			m.hangUpFor(instanceID, t, ReasonSilenceTimeout, "nobody has made a sound for "+idle.Round(time.Second).String())
		}
	}
}

// hangUpFor ends a call this package decided to end, and records why.
func (m *Manager) hangUpFor(instanceID string, t *Tracked, reason, why string) {
	m.logOf(instanceID).LogWarn("[%s] Hanging up call %s: %s", instanceID, t.call.ID(), why)
	t.setOverride(reason)
	_ = t.call.Hangup()
	m.finish(instanceID, t, reason)
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
