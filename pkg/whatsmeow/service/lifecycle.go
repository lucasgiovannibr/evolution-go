package whatsmeow_service

import (
	"context"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Process lifecycle: how instances are brought up, how automatic reconnections are paced
// and how everything is released when the process is told to stop.

var (
	// shuttingDown is set once Shutdown starts: from then on nothing reconnects or restarts
	// an instance, and disconnections no longer change the stored connection state (the
	// instances must come back on the next start, CONNECT_ON_STARTUP restores the ones
	// marked connected).
	shuttingDown atomic.Bool
	shutdownCh   = make(chan struct{})
	shutdownOnce sync.Once
)

func envDuration(name string, def time.Duration, unit time.Duration) time.Duration {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name))); err == nil && v >= 0 {
		return time.Duration(v) * unit
	}
	return def
}

// reconnectBackoff paces the reconnections the process starts by itself (the websocket
// closed, keepalives timed out). They used to be immediate and unlimited: an account that
// WhatsApp keeps dropping (banned, replaced, bad proxy) was reconnected in a tight loop,
// which is exactly the pattern that makes it worse. A reconnection asked for through the
// API is not paced.
type reconnectBackoff struct {
	mu     sync.Mutex
	states map[string]*reconnectState

	base   time.Duration // wait before the 2nd consecutive attempt; doubles each time
	max    time.Duration
	stable time.Duration // a connection that lasted this long resets the count
	rnd    func() float64
}

type reconnectState struct {
	fails       int
	connectedAt time.Time
	pending     bool
}

func newReconnectBackoff() *reconnectBackoff {
	return &reconnectBackoff{
		states: map[string]*reconnectState{},
		base:   envDuration("RECONNECT_BACKOFF_BASE_SEC", 5*time.Second, time.Second),
		max:    envDuration("RECONNECT_BACKOFF_MAX_SEC", 5*time.Minute, time.Second),
		stable: 60 * time.Second,
		rnd:    rand.Float64,
	}
}

func (b *reconnectBackoff) state(id string) *reconnectState {
	st := b.states[id]
	if st == nil {
		st = &reconnectState{}
		b.states[id] = st
	}
	return st
}

// connected records that the instance is connected now.
func (b *reconnectBackoff) connected(id string, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state(id).connectedAt = now
}

// schedule decides how long to wait before the next automatic reconnection of the
// instance. It reports false when one is already scheduled (a Disconnected and a keepalive
// timeout for the same drop must not start two).
func (b *reconnectBackoff) schedule(id string, now time.Time) (time.Duration, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.state(id)
	if st.pending {
		return 0, false
	}
	// It stayed up long enough: this is a new problem, not a continuation.
	if !st.connectedAt.IsZero() && now.Sub(st.connectedAt) >= b.stable {
		st.fails = 0
	}
	st.connectedAt = time.Time{} // that connection is gone

	var d time.Duration
	if st.fails > 0 {
		d = b.base
		for i := 1; i < st.fails && d < b.max; i++ {
			d *= 2
		}
		if d > b.max {
			d = b.max
		}
		// +-20% so instances that dropped together do not come back together.
		d = time.Duration(float64(d) * (0.8 + 0.4*b.rnd()))
	}
	st.fails++
	st.pending = true
	return d, true
}

// done marks the scheduled reconnection as handled.
func (b *reconnectBackoff) done(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state(id).pending = false
}

// forget drops what is known about an instance (deleted).
func (b *reconnectBackoff) forget(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.states, id)
}

var autoReconnect = newReconnectBackoff()

// scheduleAutoReconnect reconnects the instance after the backoff, unless the process is
// shutting down or this client was replaced in the meantime.
func (mycli *MyClient) scheduleAutoReconnect(reason string) {
	if shuttingDown.Load() {
		return
	}
	id := mycli.userID
	delay, ok := autoReconnect.schedule(id, time.Now())
	if !ok {
		return
	}
	log := mycli.loggerWrapper.GetLogger(id)

	go func() {
		defer autoReconnect.done(id)
		if delay > 0 {
			log.LogWarn("[%s] %s: reconnecting in %s (backoff)", id, reason, delay.Round(time.Second))
			t := time.NewTimer(delay)
			defer t.Stop()
			select {
			case <-t.C:
			case <-shutdownCh:
				return
			}
		} else {
			log.LogInfo("[%s] %s: reconnecting", id, reason)
		}
		if shuttingDown.Load() {
			return
		}
		if !mycli.isCurrentRuntime() {
			log.LogInfo("[%s] the client was replaced while waiting, not reconnecting", id)
			return
		}
		if err := mycli.service.ReconnectClient(id); err != nil {
			log.LogError("[%s] Failed to restart instance: %v", id, err)
		}
	}()
}

// startupStagger is the pause between two instances brought up at boot
// (STARTUP_STAGGER_MS, default 300; 0 starts them all at once, as before), plus up to 50%
// of jitter. Every instance does a handshake with WhatsApp and reads its device from the
// database: hundreds at once is a thundering herd against both.
func startupStagger() time.Duration {
	return envDuration("STARTUP_STAGGER_MS", 300*time.Millisecond, time.Millisecond)
}

func staggerWithJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return d + time.Duration(float64(d)*0.5*rand.Float64())
}

// sleepOrShutdown waits d and reports false if the process started shutting down first.
func sleepOrShutdown(d time.Duration) bool {
	if d <= 0 {
		return !shuttingDown.Load()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return !shuttingDown.Load()
	case <-shutdownCh:
		return false
	}
}

// Shutdown disconnects every client and stops the process from starting or restarting any.
// The stored state is left alone: instances marked connected are brought back on the next
// start. It returns when the clients are disconnected or ctx expires.
func (w whatsmeowService) Shutdown(ctx context.Context) {
	shutdownOnce.Do(func() {
		shuttingDown.Store(true)
		close(shutdownCh)
	})

	// Let go of the instances at the end: another replica may take them over.
	if w.owner != nil {
		defer w.owner.Close()
	}

	clients := w.clientPointer.Snapshot()
	if len(clients) == 0 {
		return
	}
	w.loggerWrapper.GetLogger("system").LogInfo("[SHUTDOWN] disconnecting %d WhatsApp client(s)", len(clients))

	var wg sync.WaitGroup
	for id, c := range clients {
		if c == nil {
			continue
		}
		wg.Add(1)
		go func(id string, c interface{ Disconnect() }) {
			defer wg.Done()
			defer recoverAndLog(w.loggerWrapper, id, "Shutdown")
			c.Disconnect()
		}(id, c)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		w.loggerWrapper.GetLogger("system").LogWarn("[SHUTDOWN] some clients did not disconnect in time")
	}
}
