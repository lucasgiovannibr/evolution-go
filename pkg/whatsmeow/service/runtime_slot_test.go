package whatsmeow_service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOnlyOneRuntimePerInstance(t *testing.T) {
	const id = "slot-test"
	releaseRuntime(id)

	var winners int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if acquireRuntime(id) {
				atomic.AddInt32(&winners, 1)
			}
		}()
	}
	wg.Wait()
	if winners != 1 {
		t.Fatalf("%d runtimes acquired the same instance, want exactly 1", winners)
	}

	if !runtimeActive(id) {
		t.Fatal("slot should be active")
	}
	releaseRuntime(id)
	if runtimeActive(id) || !acquireRuntime(id) {
		t.Fatal("slot should be free again after release")
	}
	releaseRuntime(id)
}

func TestWaitRuntimeReleased(t *testing.T) {
	const id = "slot-wait"
	releaseRuntime(id)

	if !waitRuntimeReleased(id, 50*time.Millisecond) {
		t.Fatal("a free slot must return immediately")
	}

	acquireRuntime(id)
	go func() {
		time.Sleep(150 * time.Millisecond)
		releaseRuntime(id)
	}()
	if !waitRuntimeReleased(id, 2*time.Second) {
		t.Fatal("must observe the release")
	}

	acquireRuntime(id)
	if waitRuntimeReleased(id, 150*time.Millisecond) {
		t.Fatal("must time out while the slot is held")
	}
	releaseRuntime(id)
}
