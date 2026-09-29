package whatsmeow_service

import "testing"

// Only one presence scheduler per client, however often alwaysOnline is toggled.
func TestStartPresenceUpdatesIsGuarded(t *testing.T) {
	mycli := &MyClient{}
	mycli.presenceRunning.Store(true) // as if a scheduler were already alive

	if startPresenceUpdates(mycli) {
		t.Fatal("a second scheduler must not start while one is running")
	}
	if !mycli.presenceRunning.Load() {
		t.Fatal("the running flag must be left alone")
	}
}

// A client that is not connected must be left alone (no panic on a nil WAClient).
func TestApplyAlwaysOnlineChangeWithoutAConnectedClient(t *testing.T) {
	applyAlwaysOnlineChange(&MyClient{}, true)
	applyAlwaysOnlineChange(&MyClient{}, false)
}
