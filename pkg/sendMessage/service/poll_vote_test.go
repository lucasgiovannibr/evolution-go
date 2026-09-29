package send_service

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestBuildPollInfo(t *testing.T) {
	user := types.NewJID("5511999999999", types.DefaultUserServer)
	group := types.NewJID("120363000000000000", types.GroupServer)
	own := types.NewADJID("5511000000000", 0, 7)

	t.Run("1:1 poll from the contact defaults the author to the chat", func(t *testing.T) {
		info, err := buildPollInfo(user, "P1", false, "", own)
		if err != nil {
			t.Fatal(err)
		}
		if info.Sender != user || info.IsFromMe || info.IsGroup || info.ID != "P1" {
			t.Fatalf("unexpected info: %#v", info)
		}
	})

	t.Run("poll sent by us uses our own JID without device", func(t *testing.T) {
		info, err := buildPollInfo(user, "P2", true, "", own)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsFromMe || info.Sender.Device != 0 || info.Sender.User != "5511000000000" {
			t.Fatalf("unexpected sender: %#v", info.Sender)
		}
	})

	t.Run("group poll needs the author", func(t *testing.T) {
		if _, err := buildPollInfo(group, "P3", false, "", own); err == nil {
			t.Fatal("expected an error without participant in a group")
		}
		info, err := buildPollInfo(group, "P3", false, "5511888888888@s.whatsapp.net", own)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsGroup || info.Sender.User != "5511888888888" {
			t.Fatalf("unexpected info: %#v", info)
		}
	})

	t.Run("validation", func(t *testing.T) {
		if _, err := buildPollInfo(user, "", false, "", own); err == nil {
			t.Fatal("empty poll id must fail")
		}
		if _, err := buildPollInfo(user, "P", true, "", types.EmptyJID); err == nil {
			t.Fatal("fromMe without a logged-in instance must fail")
		}
	})
}
