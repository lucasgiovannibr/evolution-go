package whatsmeow_service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

func testMediaLimiter(global, perInstance int) *mediaLimiter {
	return &mediaLimiter{global: make(chan struct{}, global), perInstance: perInstance, inst: map[string]chan struct{}{}}
}

func TestMediaLimiterHonoursGlobalAndPerInstanceCaps(t *testing.T) {
	l := testMediaLimiter(3, 2)
	var inside, peak, peakA atomic.Int32
	var insideA atomic.Int32
	var wg sync.WaitGroup

	track := func(cur, pk *atomic.Int32) {
		n := cur.Add(1)
		for {
			p := pk.Load()
			if n <= p || pk.CompareAndSwap(p, n) {
				return
			}
		}
	}
	for i := 0; i < 24; i++ {
		id := "a"
		if i%2 == 1 {
			id = "b"
		}
		wg.Add(1)
		if !l.run(id, func() {
			defer wg.Done()
			track(&inside, &peak)
			if id == "a" {
				track(&insideA, &peakA)
			}
			time.Sleep(5 * time.Millisecond)
			if id == "a" {
				insideA.Add(-1)
			}
			inside.Add(-1)
		}) {
			t.Fatal("refused below the pending cap")
		}
	}
	wg.Wait()
	if peak.Load() > 3 {
		t.Fatalf("%d at once, process limit is 3", peak.Load())
	}
	if peakA.Load() > 2 {
		t.Fatalf("instance a had %d at once, its limit is 2", peakA.Load())
	}
	if peak.Load() < 2 {
		t.Fatalf("peak %d: nothing ran concurrently", peak.Load())
	}
}

// A flooded instance waits behind its own limit; the others are not held back.
func TestMediaLimiterFloodedInstanceDoesNotStarveOthers(t *testing.T) {
	l := testMediaLimiter(4, 1)
	release := make(chan struct{})
	for i := 0; i < 10; i++ {
		l.run("flood", func() { <-release })
	}
	done := make(chan struct{})
	l.run("other", func() { close(done) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("another instance was blocked by the flood")
	}
	close(release)
}

func TestMediaLimiterBackPressureWhenTooManyWait(t *testing.T) {
	l := testMediaLimiter(1, 1)
	release := make(chan struct{})
	defer close(release)
	accepted := 0
	for i := 0; i < mediaPendingCap+50; i++ {
		if l.run("a", func() { <-release }) {
			accepted++
		}
	}
	// One is running, mediaPendingCap may be waiting; the rest is refused (handled inline).
	if accepted > mediaPendingCap+1 || accepted < mediaPendingCap {
		t.Fatalf("accepted %d, cap is %d", accepted, mediaPendingCap)
	}
}

func TestMessageHasMedia(t *testing.T) {
	if messageHasMedia(nil) || messageHasMedia(&waE2E.Message{Conversation: strPtr("hi")}) {
		t.Fatal("text is not media")
	}
	if !messageHasMedia(&waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}) ||
		!messageHasMedia(&waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}) {
		t.Fatal("image/sticker are media")
	}
	child := &waE2E.Message{AssociatedChildMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}}}
	if !messageHasMedia(child) {
		t.Fatal("media inside an associated child message counts")
	}
}

func strPtr(s string) *string { return &s }
