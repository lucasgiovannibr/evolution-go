package call_engine

import (
	"testing"

	"github.com/purpshell/meowcaller/signaling"
)

// The peer accepting our upgrade and the peer switching its camera off both leave
// Active and Upgrade false; only State tells them apart.
func TestEveryStateThePeerCanSignalHasItsOwnName(t *testing.T) {
	for raw, want := range map[int]string{
		signaling.VideoStateEnabled:          VideoStateEnabled,
		signaling.VideoStateDisabled:         VideoStateDisabled,
		signaling.VideoStateStopped:          VideoStateStopped,
		signaling.VideoStateUpgradeRequest:   VideoStateUpgradeRequest,
		signaling.VideoStateUpgradeRequestV2: VideoStateUpgradeRequest,
		signaling.VideoStateUpgradeAccept:    VideoStateUpgradeAccepted,
		signaling.VideoStateUpgradeReject:    VideoStateUpgradeRejected,
		signaling.VideoStateUpgradeCancel:    VideoStateUpgradeCancelled,
		99:                                   VideoStateUnknown,
	} {
		if got := videoStateName(raw); got != want {
			t.Errorf("videoStateName(%d) = %q, want %q", raw, got, want)
		}
	}
	if videoStateName(signaling.VideoStateUpgradeAccept) == videoStateName(signaling.VideoStateDisabled) {
		t.Error("an accepted upgrade must not look like a camera turned off")
	}
}

// Numbers WhatsApp has been seen to use but nobody has identified stay unknown, and the
// event carries the number (StateCode) so a live call can tell them apart.
func TestStatesNobodyHasIdentifiedAreUnknown(t *testing.T) {
	for _, code := range []int{2, 7, 9, 10, 12} {
		if got := videoStateName(code); got != VideoStateUnknown {
			t.Fatalf("videoStateName(%d) = %q; if it now has a name, give it a case and a test", code, got)
		}
	}
}
