package send_service

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func countingFetch(calls *atomic.Int32, jid string, found bool, err error) func() (string, bool, error) {
	return func() (string, bool, error) {
		calls.Add(1)
		return jid, found, err
	}
}

func TestRegisteredNumberIsAskedOnce(t *testing.T) {
	c := newUserExistsCache(time.Hour)
	var calls atomic.Int32

	for i := 0; i < 5; i++ {
		jid, found, err := c.lookup("inst", "5511999990001", countingFetch(&calls, "5511999990001@s.whatsapp.net", true, nil))
		if err != nil || !found || jid != "5511999990001@s.whatsapp.net" {
			t.Fatalf("lookup %d = %q %v %v", i, jid, found, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("WhatsApp was asked %d times for the same number, want 1", calls.Load())
	}
}

func TestNotRegisteredIsRememberedBrieflyOnly(t *testing.T) {
	c := newUserExistsCache(time.Hour)
	now := time.Now()
	c.now = func() time.Time { return now }
	var calls atomic.Int32

	c.lookup("inst", "n", countingFetch(&calls, "", false, nil))
	c.lookup("inst", "n", countingFetch(&calls, "", false, nil))
	if calls.Load() != 1 {
		t.Fatalf("a 'no' must be remembered for a while, calls = %d", calls.Load())
	}

	now = now.Add(userExistsNegativeTTL + time.Second)
	c.lookup("inst", "n", countingFetch(&calls, "n@s.whatsapp.net", true, nil))
	if calls.Load() != 2 {
		t.Fatalf("after the short negative TTL the number is asked again, calls = %d", calls.Load())
	}
	// ... and the new, positive answer is kept for the long TTL.
	now = now.Add(time.Hour - time.Minute)
	if _, found, _ := c.lookup("inst", "n", countingFetch(&calls, "", false, nil)); !found || calls.Load() != 2 {
		t.Fatalf("the positive answer must be served from the cache (found=%v calls=%d)", found, calls.Load())
	}
}

func TestPositiveAnswerExpires(t *testing.T) {
	c := newUserExistsCache(time.Hour)
	now := time.Now()
	c.now = func() time.Time { return now }
	var calls atomic.Int32

	c.lookup("inst", "n", countingFetch(&calls, "j", true, nil))
	now = now.Add(time.Hour + time.Second)
	c.lookup("inst", "n", countingFetch(&calls, "j", true, nil))
	if calls.Load() != 2 {
		t.Fatalf("an expired answer must be fetched again, calls = %d", calls.Load())
	}
}

func TestErrorsAreNeverRemembered(t *testing.T) {
	c := newUserExistsCache(time.Hour)
	var calls atomic.Int32
	boom := errors.New("network down")

	if _, _, err := c.lookup("inst", "n", countingFetch(&calls, "", false, boom)); !errors.Is(err, boom) {
		t.Fatalf("the error must be returned, got %v", err)
	}
	c.lookup("inst", "n", countingFetch(&calls, "j", true, nil))
	if calls.Load() != 2 {
		t.Fatalf("after a failure the next call must ask again, calls = %d", calls.Load())
	}
}

func TestInstancesDoNotShareAnswers(t *testing.T) {
	c := newUserExistsCache(time.Hour)
	var calls atomic.Int32
	c.lookup("a", "n", countingFetch(&calls, "j", true, nil))
	c.lookup("b", "n", countingFetch(&calls, "j", true, nil))
	if calls.Load() != 2 {
		t.Fatalf("each instance asks for itself, calls = %d", calls.Load())
	}
}

func TestConcurrentChecksOfTheSameNumberShareOneQuery(t *testing.T) {
	c := newUserExistsCache(time.Hour)
	var calls atomic.Int32
	release := make(chan struct{})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.lookup("inst", "n", func() (string, bool, error) {
				calls.Add(1)
				<-release
				return "j", true, nil
			})
		}()
	}
	time.Sleep(50 * time.Millisecond) // let them all arrive while the first is in flight
	close(release)
	wg.Wait()

	if calls.Load() != 1 {
		t.Fatalf("20 concurrent sends to one number made %d queries, want 1", calls.Load())
	}
}

func TestForgetAndDisabledCache(t *testing.T) {
	c := newUserExistsCache(time.Hour)
	var calls atomic.Int32
	c.lookup("inst", "n", countingFetch(&calls, "j", true, nil))
	c.forget("inst", "n")
	c.lookup("inst", "n", countingFetch(&calls, "j", true, nil))
	if calls.Load() != 2 {
		t.Fatalf("a forgotten number is asked again, calls = %d", calls.Load())
	}

	var off *userExistsCache = newUserExistsCache(0)
	if off != nil {
		t.Fatal("a zero TTL disables the cache (nil)")
	}
	calls.Store(0)
	off.lookup("inst", "n", countingFetch(&calls, "j", true, nil))
	off.lookup("inst", "n", countingFetch(&calls, "j", true, nil))
	off.forget("inst", "n") // must not panic
	if calls.Load() != 2 {
		t.Fatalf("a disabled cache always asks, calls = %d", calls.Load())
	}
}

func TestCacheNeverGrowsWithoutLimit(t *testing.T) {
	c := newUserExistsCache(time.Hour)
	for i := 0; i < userExistsMaxEntries+10; i++ {
		c.store("inst|"+time.Duration(i).String(), "j", true)
	}
	c.mu.Lock()
	n := len(c.entries)
	c.mu.Unlock()
	if n > userExistsMaxEntries {
		t.Fatalf("the cache holds %d entries, the limit is %d", n, userExistsMaxEntries)
	}
}
