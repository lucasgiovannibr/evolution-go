package group_service

import (
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

func clientWithIdentity(pn, lid string) *whatsmeow.Client {
	dev := &store.Device{}
	id := types.JID{User: pn, Device: 82, Server: types.DefaultUserServer}
	dev.ID = &id
	dev.LID = types.NewJID(lid, types.HiddenUserServer)
	return &whatsmeow.Client{Store: dev}
}

func TestIsGroupOwner(t *testing.T) {
	c := clientWithIdentity("553197157574", "273117121392855")

	cases := map[string]struct {
		group *types.GroupInfo
		want  bool
	}{
		"owner by LID":      {&types.GroupInfo{OwnerJID: types.NewJID("273117121392855", types.HiddenUserServer)}, true},
		"owner by phone":    {&types.GroupInfo{OwnerJID: types.NewJID("553197157574", types.DefaultUserServer)}, true},
		"LID owner, PN set": {&types.GroupInfo{OwnerJID: types.NewJID("999", types.HiddenUserServer), OwnerPN: types.NewJID("553197157574", types.DefaultUserServer)}, true},
		"someone else":      {&types.GroupInfo{OwnerJID: types.NewJID("999", types.HiddenUserServer), OwnerPN: types.NewJID("5511999990001", types.DefaultUserServer)}, false},
		"no owner":          {&types.GroupInfo{}, false},
		"nil group":         {nil, false},
	}
	for name, tc := range cases {
		if got := isGroupOwner(c, tc.group); got != tc.want {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}

	// a user id that only looks alike on the wrong server is not the owner
	if isGroupOwner(c, &types.GroupInfo{OwnerJID: types.NewJID("553197157574", types.HiddenUserServer)}) {
		t.Error("a LID with the same digits as the phone number is a different user")
	}
	if isGroupOwner(nil, &types.GroupInfo{OwnerJID: types.NewJID("553197157574", types.DefaultUserServer)}) {
		t.Error("no client, no ownership")
	}
}
