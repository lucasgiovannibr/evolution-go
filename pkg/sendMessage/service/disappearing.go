package send_service

import (
	"os"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// autoDisappearingEnabled is the kill switch of the automatic disappearing-messages
// timer on send: DISAPPEARING_AUTO_APPLY=false turns it off.
func autoDisappearingEnabled() bool {
	return strings.ToLower(strings.TrimSpace(os.Getenv("DISAPPEARING_AUTO_APPLY"))) != "false"
}

// disappearingApplies says whether a chat can have a disappearing timer at all.
func disappearingApplies(chat types.JID) bool {
	switch chat.Server {
	case types.DefaultUserServer, types.HiddenUserServer, types.GroupServer:
		return true
	}
	return false
}

// messageContextInfo returns the ContextInfo of whichever content the message holds,
// creating it when create is true. nil when the message has no content that carries
// one.
func messageContextInfo(msg *waE2E.Message, create bool) *waE2E.ContextInfo {
	if msg == nil {
		return nil
	}

	var slot **waE2E.ContextInfo
	switch {
	case msg.ExtendedTextMessage != nil:
		slot = &msg.ExtendedTextMessage.ContextInfo
	case msg.ImageMessage != nil:
		slot = &msg.ImageMessage.ContextInfo
	case msg.VideoMessage != nil:
		slot = &msg.VideoMessage.ContextInfo
	case msg.PtvMessage != nil:
		slot = &msg.PtvMessage.ContextInfo
	case msg.AudioMessage != nil:
		slot = &msg.AudioMessage.ContextInfo
	case msg.DocumentMessage != nil:
		slot = &msg.DocumentMessage.ContextInfo
	case msg.DocumentWithCaptionMessage != nil &&
		msg.DocumentWithCaptionMessage.Message != nil &&
		msg.DocumentWithCaptionMessage.Message.DocumentMessage != nil:
		slot = &msg.DocumentWithCaptionMessage.Message.DocumentMessage.ContextInfo
	case msg.StickerMessage != nil:
		slot = &msg.StickerMessage.ContextInfo
	case msg.LocationMessage != nil:
		slot = &msg.LocationMessage.ContextInfo
	case msg.ContactMessage != nil:
		slot = &msg.ContactMessage.ContextInfo
	case msg.PollCreationMessage != nil:
		slot = &msg.PollCreationMessage.ContextInfo
	case msg.ButtonsMessage != nil:
		slot = &msg.ButtonsMessage.ContextInfo
	case msg.ListMessage != nil:
		slot = &msg.ListMessage.ContextInfo
	case msg.InteractiveMessage != nil:
		slot = &msg.InteractiveMessage.ContextInfo
	default:
		return nil
	}

	if *slot == nil && create {
		*slot = &waE2E.ContextInfo{}
	}
	return *slot
}

// applyDisappearingExpiration stamps the chat's disappearing timer on the message, so
// the recipient does not see "this message will not disappear". A value the caller
// already set wins; seconds == 0 (timer off) leaves the message alone.
func applyDisappearingExpiration(msg *waE2E.Message, seconds uint32) bool {
	if seconds == 0 {
		return false
	}
	ci := messageContextInfo(msg, true)
	if ci == nil || ci.Expiration != nil {
		return false
	}
	ci.Expiration = &seconds
	return true
}
