package whatsmeow_service

import (
	"context"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Disappearing messages (issue #79).
//
// When a chat has a disappearing-messages timer, every message sent to it has to
// carry that duration in ContextInfo.Expiration; without it the recipient sees
// "this message will not disappear / the person may be using an older version of
// WhatsApp". whatsmeow does not add it on send, and it cannot be READ either: its
// chat-settings store only keeps mute/pin/archive. So the project learns the timer
// from what WhatsApp already tells it:
//
//   - ContextInfo.Expiration of messages received (or sent from another device);
//   - the EPHEMERAL_SETTING protocol message, when the timer is changed in a chat
//     (also the only source for "turned off");
//   - group info events and group info lookups (events.GroupInfo.Ephemeral);
//   - the project's own endpoint that sets the timer.
//
// The memory is per process. An entry that is never refreshed is dropped after
// chatTimerTTL, so a timer that was turned off without us seeing it stops being
// applied instead of being applied forever.

const (
	chatTimerTTL            = 7 * 24 * time.Hour
	groupTimerLookupTimeout = 5 * time.Second
)

type chatTimer struct {
	seconds   uint32
	learnedAt time.Time
}

var chatTimers sync.Map // "instanceID|chat" -> chatTimer

func chatTimerKey(instanceID string, chat types.JID) string {
	return instanceID + "|" + chat.ToNonAD().String()
}

// rememberChatTimer records the timer of a chat. seconds == 0 means "off".
func rememberChatTimer(instanceID string, chat types.JID, seconds uint32, now time.Time) {
	if chat.IsEmpty() || chat.Server == types.BroadcastServer || chat.Server == types.NewsletterServer {
		return
	}
	chatTimers.Store(chatTimerKey(instanceID, chat), chatTimer{seconds: seconds, learnedAt: now})
}

// lookupChatTimer returns the remembered timer of a chat. known is false when
// nothing (fresh enough) is remembered.
func lookupChatTimer(instanceID string, chat types.JID, now time.Time) (seconds uint32, known bool) {
	v, ok := chatTimers.Load(chatTimerKey(instanceID, chat))
	if !ok {
		return 0, false
	}
	t := v.(chatTimer)
	if now.Sub(t.learnedAt) > chatTimerTTL {
		chatTimers.Delete(chatTimerKey(instanceID, chat))
		return 0, false
	}
	return t.seconds, true
}

// forgetInstanceChatTimers drops everything remembered for an instance.
func forgetInstanceChatTimers(instanceID string) {
	prefix := instanceID + "|"
	chatTimers.Range(func(k, _ interface{}) bool {
		if key := k.(string); len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			chatTimers.Delete(k)
		}
		return true
	})
}

// ChatDisappearingSeconds returns the disappearing timer known for a chat (in
// seconds). It looks the chat up under its phone-number and its LID form, since the
// events and the sends of the same conversation do not always use the same one.
func (w *whatsmeowService) ChatDisappearingSeconds(instanceID string, chat types.JID) (uint32, bool) {
	now := time.Now()
	if s, ok := lookupChatTimer(instanceID, chat, now); ok {
		return s, true
	}

	client := w.clientPointer.Get(instanceID)
	if client == nil {
		return 0, false
	}

	// A group is the one chat whose timer can be READ: ask once, then it is remembered
	// (this is what makes it survive a restart).
	if chat.Server == types.GroupServer {
		ctx, cancel := context.WithTimeout(context.Background(), groupTimerLookupTimeout)
		defer cancel()
		info, err := client.GetGroupInfo(ctx, chat)
		if err != nil || info == nil {
			return 0, false
		}
		learnChatTimerFromGroup(instanceID, chat, &info.GroupEphemeral, now)
		return lookupChatTimer(instanceID, chat, now)
	}

	if client.Store == nil || client.Store.LIDs == nil {
		return 0, false
	}
	switch chat.Server {
	case types.DefaultUserServer:
		if lid, err := client.Store.LIDs.GetLIDForPN(context.Background(), chat.ToNonAD()); err == nil && !lid.IsEmpty() {
			return lookupChatTimer(instanceID, lid, now)
		}
	case types.HiddenUserServer:
		if pn, err := client.Store.LIDs.GetPNForLID(context.Background(), chat.ToNonAD()); err == nil && !pn.IsEmpty() {
			return lookupChatTimer(instanceID, pn, now)
		}
	}
	return 0, false
}

// RememberChatDisappearing records a timer learnt elsewhere (the endpoint that sets
// it, a group lookup).
func (w *whatsmeowService) RememberChatDisappearing(instanceID string, chat types.JID, seconds uint32) {
	rememberChatTimer(instanceID, chat, seconds, time.Now())
}

// messageExpiration reads ContextInfo.Expiration from the message types that carry a
// ContextInfo. ok is false when the message has none to read.
func messageExpiration(msg *waE2E.Message) (uint32, bool) {
	if msg == nil {
		return 0, false
	}
	var ci *waE2E.ContextInfo
	switch {
	case msg.ExtendedTextMessage != nil:
		ci = msg.ExtendedTextMessage.ContextInfo
	case msg.ImageMessage != nil:
		ci = msg.ImageMessage.ContextInfo
	case msg.VideoMessage != nil:
		ci = msg.VideoMessage.ContextInfo
	case msg.AudioMessage != nil:
		ci = msg.AudioMessage.ContextInfo
	case msg.DocumentMessage != nil:
		ci = msg.DocumentMessage.ContextInfo
	case msg.StickerMessage != nil:
		ci = msg.StickerMessage.ContextInfo
	case msg.LocationMessage != nil:
		ci = msg.LocationMessage.ContextInfo
	case msg.ContactMessage != nil:
		ci = msg.ContactMessage.ContextInfo
	case msg.PollCreationMessage != nil:
		ci = msg.PollCreationMessage.ContextInfo
	case msg.ButtonsMessage != nil:
		ci = msg.ButtonsMessage.ContextInfo
	case msg.ListMessage != nil:
		ci = msg.ListMessage.ContextInfo
	case msg.InteractiveMessage != nil:
		ci = msg.InteractiveMessage.ContextInfo
	}
	if ci == nil || ci.Expiration == nil {
		return 0, false
	}
	return ci.GetExpiration(), true
}

// learnChatTimerFromMessage updates the memory from a received message.
func learnChatTimerFromMessage(instanceID string, evt *events.Message, now time.Time) {
	if evt == nil || evt.Message == nil {
		return
	}

	// The timer was changed in this chat: the authoritative source, and the only one
	// that says "turned off".
	if pm := evt.Message.GetProtocolMessage(); pm != nil && pm.GetType() == waE2E.ProtocolMessage_EPHEMERAL_SETTING {
		rememberChatTimer(instanceID, evt.Info.Chat, pm.GetEphemeralExpiration(), now)
		return
	}

	// A message that carries the duration: positive values only. A message WITHOUT
	// it proves nothing (older clients omit it), so it never resets a known timer.
	if seconds, ok := messageExpiration(evt.Message); ok && seconds > 0 {
		rememberChatTimer(instanceID, evt.Info.Chat, seconds, now)
	}
}

// learnChatTimerFromGroup updates the memory from group info.
func learnChatTimerFromGroup(instanceID string, group types.JID, eph *types.GroupEphemeral, now time.Time) {
	if eph == nil {
		return
	}
	seconds := uint32(0)
	if eph.IsEphemeral {
		seconds = eph.DisappearingTimer
	}
	rememberChatTimer(instanceID, group, seconds, now)
}
