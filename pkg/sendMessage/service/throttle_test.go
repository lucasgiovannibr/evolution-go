package send_service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestThrottleLimitsConcurrencyPerInstance(t *testing.T) {
	th := newSendThrottle(throttleConfig{maxConcurrent: 3, maxWait: 5 * time.Second})

	var inside, peak atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := th.acquire(context.Background(), "a")
			if err != nil {
				t.Error(err)
				return
			}
			n := inside.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			inside.Add(-1)
			release()
		}()
	}
	wg.Wait()
	if peak.Load() > 3 {
		t.Fatalf("%d sends were inside at once, limit is 3", peak.Load())
	}
	if peak.Load() < 2 {
		t.Fatalf("peak %d: nothing ran concurrently", peak.Load())
	}
}

// One busy instance must not slow another.
func TestThrottleInstancesAreIndependent(t *testing.T) {
	th := newSendThrottle(throttleConfig{maxConcurrent: 1, maxWait: 100 * time.Millisecond})
	release, err := th.acquire(context.Background(), "busy")
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	start := time.Now()
	r2, err := th.acquire(context.Background(), "other")
	if err != nil {
		t.Fatalf("another instance was refused: %v", err)
	}
	r2()
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("another instance had to wait")
	}
}

func TestThrottleRefusesAfterMaxWait(t *testing.T) {
	th := newSendThrottle(throttleConfig{maxConcurrent: 1, maxWait: 50 * time.Millisecond})
	release, _ := th.acquire(context.Background(), "a")
	defer release()

	_, err := th.acquire(context.Background(), "a")
	thr, ok := AsThrottled(err)
	if !ok {
		t.Fatalf("want ErrSendThrottled, got %v", err)
	}
	if thr.RetryAfterSeconds() < 1 {
		t.Fatalf("Retry-After %d", thr.RetryAfterSeconds())
	}
}

func TestThrottleReleaseLetsTheNextIn(t *testing.T) {
	th := newSendThrottle(throttleConfig{maxConcurrent: 1, maxWait: 2 * time.Second})
	r1, _ := th.acquire(context.Background(), "a")
	got := make(chan error, 1)
	go func() {
		r2, err := th.acquire(context.Background(), "a")
		if err == nil {
			r2()
		}
		got <- err
	}()
	time.Sleep(20 * time.Millisecond)
	r1()
	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiter was not let in after release")
	}
}

func TestThrottleRateSpacesMessagesAndRefusesLongWaits(t *testing.T) {
	now := time.Unix(1000, 0)
	th := newSendThrottle(throttleConfig{ratePerMin: 60, maxWait: 3 * time.Second}) // 1/s, burst 6
	th.now = func() time.Time { return now }
	l := th.limiter("a")

	for i := 0; i < 6; i++ { // the burst goes through at once
		if w := th.reserve(l); w != 0 {
			t.Fatalf("burst message %d must not wait, got %v", i, w)
		}
	}
	if w := th.reserve(l); w < 900*time.Millisecond || w > 1100*time.Millisecond {
		t.Fatalf("7th message waits ~1s, got %v", w)
	}
	if w := th.reserve(l); w < 1900*time.Millisecond || w > 2100*time.Millisecond {
		t.Fatalf("8th queues behind the 7th: ~2s, got %v", w)
	}
	now = now.Add(10 * time.Second) // idle: the bucket refills (to the burst)
	if w := th.reserve(l); w != 0 {
		t.Fatalf("after idling the next one is immediate, got %v", w)
	}
}

func TestThrottleRateRefusesWhenTheWaitExceedsMaxWait(t *testing.T) {
	th := newSendThrottle(throttleConfig{ratePerMin: 6, maxWait: 2 * time.Second}) // 1 per 10 s, burst 1
	if r, err := th.acquire(context.Background(), "a"); err != nil {
		t.Fatal(err)
	} else {
		r()
	}
	_, err := th.acquire(context.Background(), "a")
	thr, ok := AsThrottled(err)
	if !ok {
		t.Fatalf("want a throttle error, got %v", err)
	}
	if thr.RetryAfterSeconds() < 8 || thr.RetryAfterSeconds() > 11 {
		t.Fatalf("Retry-After %d, want about 10", thr.RetryAfterSeconds())
	}
}

func TestThrottleDisabledIsFree(t *testing.T) {
	th := newSendThrottle(throttleConfig{})
	for i := 0; i < 100; i++ {
		r, err := th.acquire(context.Background(), "a")
		if err != nil {
			t.Fatal(err)
		}
		r()
	}
}
