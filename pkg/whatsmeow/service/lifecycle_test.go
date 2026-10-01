package whatsmeow_service

import (
	"testing"
	"time"
)

func testBackoff() *reconnectBackoff {
	return &reconnectBackoff{
		states: map[string]*reconnectState{},
		base:   5 * time.Second,
		max:    40 * time.Second,
		stable: 60 * time.Second,
		rnd:    func() float64 { return 0.5 }, // jitter factor 1.0
	}
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	b := testBackoff()
	now := time.Now()
	want := []time.Duration{0, 5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 40 * time.Second}
	for i, w := range want {
		d, ok := b.schedule("i", now)
		if !ok {
			t.Fatalf("attempt %d refused", i)
		}
		if d != w {
			t.Fatalf("attempt %d: delay %v, want %v", i, d, w)
		}
		b.done("i")
	}
}

// A drop caused by an instance that was fine for a while is a new problem, not the
// continuation of a loop.
func TestBackoffResetsAfterAStableConnection(t *testing.T) {
	b := testBackoff()
	now := time.Now()
	for i := 0; i < 4; i++ {
		b.schedule("i", now)
		b.done("i")
	}
	b.connected("i", now)
	if d, _ := b.schedule("i", now.Add(2*time.Minute)); d != 0 {
		t.Fatalf("after a stable connection the next reconnect is immediate, got %v", d)
	}
}

// A connection that dropped again within seconds (flapping) does not reset the count: the
// wait keeps growing even though it connected in between.
func TestBackoffKeepsGrowingWhenConnectionsDoNotLast(t *testing.T) {
	b := testBackoff()
	now := time.Now()
	if d, _ := b.schedule("i", now); d != 0 {
		t.Fatalf("first reconnect is immediate, got %v", d)
	}
	b.done("i")
	b.connected("i", now)
	if d, _ := b.schedule("i", now.Add(3*time.Second)); d != 5*time.Second {
		t.Fatalf("dropped 3s after connecting: want 5s, got %v", d)
	}
	b.done("i")
	b.connected("i", now.Add(9*time.Second))
	if d, _ := b.schedule("i", now.Add(12*time.Second)); d != 10*time.Second {
		t.Fatalf("flapping again: want 10s, got %v", d)
	}
}

// A Disconnected and a keepalive timeout for the same drop must start one reconnection.
func TestBackoffDeduplicatesPendingReconnects(t *testing.T) {
	b := testBackoff()
	if _, ok := b.schedule("i", time.Now()); !ok {
		t.Fatal("first must be accepted")
	}
	if _, ok := b.schedule("i", time.Now()); ok {
		t.Fatal("second while the first is pending must be refused")
	}
	if _, ok := b.schedule("other", time.Now()); !ok {
		t.Fatal("instances are independent")
	}
	b.done("i")
	if _, ok := b.schedule("i", time.Now()); !ok {
		t.Fatal("after done a new one is accepted")
	}
}

func TestBackoffJitterStaysWithinTwentyPercent(t *testing.T) {
	for _, r := range []float64{0, 0.999} {
		b := testBackoff()
		b.rnd = func() float64 { return r }
		now := time.Now()
		b.schedule("i", now)
		b.done("i")
		d, _ := b.schedule("i", now)
		if d < 4*time.Second || d > 6*time.Second {
			t.Fatalf("rnd=%v: %v is outside 5s +-20%%", r, d)
		}
	}
}

func TestStaggerWithJitter(t *testing.T) {
	if staggerWithJitter(0) != 0 {
		t.Fatal("0 means no pacing")
	}
	for i := 0; i < 100; i++ {
		d := staggerWithJitter(100 * time.Millisecond)
		if d < 100*time.Millisecond || d > 150*time.Millisecond {
			t.Fatalf("%v outside [100ms,150ms]", d)
		}
	}
}
