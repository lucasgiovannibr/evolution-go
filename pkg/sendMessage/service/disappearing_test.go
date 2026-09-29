package send_service

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func TestApplyDisappearingExpirationCreatesContextInfo(t *testing.T) {
	msg := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("oi")}}
	if !applyDisappearingExpiration(msg, 86400) {
		t.Fatal("must apply")
	}
	if got := msg.ExtendedTextMessage.GetContextInfo().GetExpiration(); got != 86400 {
		t.Fatalf("expiration = %d", got)
	}
}

func TestApplyDisappearingExpirationKeepsQuoteAndAKnownValue(t *testing.T) {
	msg := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("Q1")},
	}}
	applyDisappearingExpiration(msg, 604800)
	ci := msg.ExtendedTextMessage.ContextInfo
	if ci.GetStanzaID() != "Q1" || ci.GetExpiration() != 604800 {
		t.Fatalf("quote must survive next to the timer: %+v", ci)
	}

	// what the caller already set wins
	if applyDisappearingExpiration(msg, 86400) || ci.GetExpiration() != 604800 {
		t.Fatal("an existing expiration must not be overwritten")
	}
}

func TestApplyDisappearingExpirationOffOrUnsupported(t *testing.T) {
	msg := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}
	if applyDisappearingExpiration(msg, 0) || msg.ImageMessage.ContextInfo != nil {
		t.Fatal("timer off leaves the message untouched")
	}
	if applyDisappearingExpiration(&waE2E.Message{Conversation: proto.String("x")}, 86400) {
		t.Fatal("a plain conversation has nowhere to carry it")
	}
	if applyDisappearingExpiration(nil, 86400) {
		t.Fatal("nil message")
	}
}

func TestMessageContextInfoCoversTheSendableTypes(t *testing.T) {
	cases := map[string]*waE2E.Message{
		"image":    {ImageMessage: &waE2E.ImageMessage{}},
		"video":    {VideoMessage: &waE2E.VideoMessage{}},
		"ptv":      {PtvMessage: &waE2E.VideoMessage{}},
		"audio":    {AudioMessage: &waE2E.AudioMessage{}},
		"document": {DocumentMessage: &waE2E.DocumentMessage{}},
		"docCap": {DocumentWithCaptionMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{}},
		}},
		"sticker":  {StickerMessage: &waE2E.StickerMessage{}},
		"location": {LocationMessage: &waE2E.LocationMessage{}},
		"contact":  {ContactMessage: &waE2E.ContactMessage{}},
		"poll":     {PollCreationMessage: &waE2E.PollCreationMessage{}},
		"buttons":  {ButtonsMessage: &waE2E.ButtonsMessage{}},
		"list":     {ListMessage: &waE2E.ListMessage{}},
		"interact": {InteractiveMessage: &waE2E.InteractiveMessage{}},
	}
	for name, msg := range cases {
		if !applyDisappearingExpiration(msg, 86400) {
			t.Errorf("%s: timer not applied", name)
			continue
		}
		if got := messageContextInfo(msg, false).GetExpiration(); got != 86400 {
			t.Errorf("%s: expiration = %d", name, got)
		}
	}
}

func TestDisappearingApplies(t *testing.T) {
	if !disappearingApplies(types.NewJID("5511999990001", types.DefaultUserServer)) ||
		!disappearingApplies(types.NewJID("123", types.HiddenUserServer)) ||
		!disappearingApplies(types.NewJID("120363000000000001", types.GroupServer)) {
		t.Fatal("contact, LID and group chats have the setting")
	}
	if disappearingApplies(types.NewJID("123", types.NewsletterServer)) || disappearingApplies(types.StatusBroadcastJID) {
		t.Fatal("newsletters and status do not")
	}
}

func TestAutoDisappearingKillSwitch(t *testing.T) {
	t.Setenv("DISAPPEARING_AUTO_APPLY", "")
	if !autoDisappearingEnabled() {
		t.Fatal("on by default")
	}
	t.Setenv("DISAPPEARING_AUTO_APPLY", " False ")
	if autoDisappearingEnabled() {
		t.Fatal("DISAPPEARING_AUTO_APPLY=false must turn it off")
	}
}
