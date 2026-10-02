package call_engine

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrEngineUnavailable: the instance has no working call engine.
	ErrEngineUnavailable = errors.New("the instance has no working call engine")
	// ErrDialRateLimited: the instance placed too many calls in the last minute.
	ErrDialRateLimited = errors.New("too many calls placed in the last minute")
	// ErrDialFailed: the library could not place the call (unknown number, peer
	// unreachable, offer not sent). The wrapped error says why.
	ErrDialFailed = errors.New("could not place the call")
)

// DialOptions say what kind of call to place.
type DialOptions struct {
	// Video places a video call: both directions carry video from the start.
	Video bool
}

// DialFunc places a call to target. The library's is the default; Options.Dial
// replaces it in tests.
type DialFunc func(ctx context.Context, instanceID, target string, opts DialOptions) (Call, error)

// Dial places an outgoing call and follows it like any other. The Tracked
// returned is in the calling phase: it rings on the peer's phone, and its audio can be
// streamed right away.
//
// Placing calls is the most likely way to get an account flagged, so it is bounded on
// its own: an instance may place Options.DialsPerMinute calls a minute, whether or not
// they succeed (a failed attempt still talks to WhatsApp), and never more than
// Options.MaxConcurrent at a time.
func (m *Manager) Dial(ctx context.Context, instanceID, target string, opts DialOptions) (*Tracked, error) {
	m.mu.RLock()
	rt := m.runtimes[instanceID]
	open := len(m.calls[instanceID])
	m.mu.RUnlock()

	if rt == nil || rt.status.State != StateActive || (rt.dial == nil && m.opts.Dial == nil) {
		m.metrics.dials.WithLabelValues("unavailable").Inc()
		return nil, ErrEngineUnavailable
	}
	if open >= m.opts.MaxConcurrent {
		m.metrics.dials.WithLabelValues("busy").Inc()
		return nil, ErrTooManyCalls
	}
	if !m.allowDial(instanceID) {
		m.metrics.dials.WithLabelValues("rate_limited").Inc()
		return nil, ErrDialRateLimited
	}

	dial := rt.dial
	if m.opts.Dial != nil {
		dial = func(ctx context.Context, target string, opts DialOptions) (Call, error) {
			return m.opts.Dial(ctx, instanceID, target, opts)
		}
	}
	c, err := dial(ctx, target, opts)
	if err != nil {
		m.metrics.dials.WithLabelValues("failed").Inc()
		return nil, fmt.Errorf("%w: %v", ErrDialFailed, err)
	}

	t, err := m.Track(instanceID, c, Outgoing)
	if err != nil {
		// Another call took the last slot while this one was being placed.
		_ = c.Hangup()
		m.metrics.dials.WithLabelValues("busy").Inc()
		return nil, err
	}
	m.metrics.dials.WithLabelValues("ok").Inc()
	return t, nil
}

// allowDial records a dial attempt and says whether it is within the instance's rate.
func (m *Manager) allowDial(instanceID string) bool {
	m.dialMu.Lock()
	defer m.dialMu.Unlock()

	now := m.now()
	recent := m.dials[instanceID][:0]
	for _, at := range m.dials[instanceID] {
		if now.Sub(at) < time.Minute {
			recent = append(recent, at)
		}
	}
	if len(recent) >= m.opts.DialsPerMinute {
		m.dials[instanceID] = recent
		return false
	}
	m.dials[instanceID] = append(recent, now)
	return true
}
