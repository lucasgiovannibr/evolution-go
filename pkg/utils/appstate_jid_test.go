package utils

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

// Without a client (or without a LID mapping) the JID is left as it is; groups and
// LIDs are never touched.
func TestAppStateChatJIDLeavesJIDsAloneWithoutAMapping(t *testing.T) {
	for _, j := range []types.JID{
		types.NewJID("5531999990001", types.DefaultUserServer),
		types.NewJID("120363000000000001", types.GroupServer),
		types.NewJID("273117121392855", types.HiddenUserServer),
	} {
		if got := AppStateChatJID(nil, j); got != j {
			t.Errorf("AppStateChatJID(nil, %v) = %v", j, got)
		}
	}
}
