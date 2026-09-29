package whatsmeow_service

import (
	"sync"
	"time"
)

// One runtime per instance.
//
// The manager's standard flow is POST /instance/connect followed ~2s later by
// GET /instance/qr. The client is only registered after StartClient has done a
// few things, so GetQr saw "no client" and started the instance a SECOND time.
// Two clients then ran for the same instance: the user paired one, the other
// kept rotating QR codes until the max count, forced a logout, and its
// teardown signalled the shared kill channel — which restarted the instance as
// a brand-new, unpaired device right after a successful pairing.
//
// A slot is held for the whole lifetime of a StartClient run; a second start
// while it is held is a no-op.
var runtimeSlots sync.Map // instanceID -> struct{}

// acquireRuntime returns false when a runtime already owns the instance.
func acquireRuntime(instanceID string) bool {
	_, loaded := runtimeSlots.LoadOrStore(instanceID, struct{}{})
	return !loaded
}

func releaseRuntime(instanceID string) {
	runtimeSlots.Delete(instanceID)
}

// runtimeActive reports whether a runtime currently owns the instance.
func runtimeActive(instanceID string) bool {
	_, ok := runtimeSlots.Load(instanceID)
	return ok
}

// waitRuntimeReleased waits until the runtime of instanceID has ended, up to max.
// It returns true when the slot is free.
func waitRuntimeReleased(instanceID string, max time.Duration) bool {
	deadline := time.Now().Add(max)
	for runtimeActive(instanceID) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
	return true
}
