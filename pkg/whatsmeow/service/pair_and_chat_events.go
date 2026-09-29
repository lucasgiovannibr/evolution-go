package whatsmeow_service

import (
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Events whatsmeow emitted that the project used to drop into the generic
// "Unhandled event" log line:
//
//   - pairing failures (PairError, QRScannedWithoutMultidevice): the user got no
//     feedback at all when pairing did not complete. They follow the QRCODE
//     subscription, like the rest of the pairing flow.
//   - CATRefreshError follows CONNECTION.
//   - chat-state changes made on ANOTHER device (Mute, Pin, Star, MarkChatAsRead,
//     ClearChat, DeleteChat, DeleteForMe, UnarchiveChatsSetting, UserStatusMute):
//     an integration (a CRM, an inbox) never learnt that a chat was pinned, muted or
//     deleted. They follow CHAT_PRESENCE, where Archive already was.
//
// ManualLoginReconnect is only emitted when DisableLoginAutoReconnect is set, which
// this project never does, so it is not handled.

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func jidString(j types.JID) string {
	if j.IsEmpty() {
		return ""
	}
	return j.String()
}

// pairAndChatEventData converts one of those events into the name and payload that
// are published. publish is false for events that must not be published: the
// app-state ones that arrive as part of a full sync. Right after a pairing the
// phone replays its whole state (every pinned, muted, starred chat), and publishing
// that would flood a subscriber with thousands of "changes" that are not changes.
// handled is false for every other event type.
func pairAndChatEventData(rawEvt interface{}) (name string, data map[string]interface{}, publish bool, handled bool) {
	switch evt := rawEvt.(type) {
	case *events.PairError:
		data = map[string]interface{}{
			"id":           jidString(evt.ID),
			"lid":          jidString(evt.LID),
			"businessName": evt.BusinessName,
			"platform":     evt.Platform,
		}
		if evt.Error != nil {
			data["error"] = evt.Error.Error()
		}
		return "PairError", data, true, true

	case *events.QRScannedWithoutMultidevice:
		return "QRScannedWithoutMultidevice", map[string]interface{}{
			"message": "the QR code was scanned by a phone without multi-device enabled; enable it and scan the same code again",
		}, true, true

	case *events.CATRefreshError:
		data = map[string]interface{}{}
		if evt.Error != nil {
			data["error"] = evt.Error.Error()
		}
		return "CATRefreshError", data, true, true

	case *events.Mute:
		return "Mute", map[string]interface{}{
			"jid":              jidString(evt.JID),
			"timestamp":        rfc3339(evt.Timestamp),
			"muted":            evt.Action.GetMuted(),
			"muteEndTimestamp": evt.Action.GetMuteEndTimestamp(),
		}, !evt.FromFullSync, true

	case *events.Pin:
		return "Pin", map[string]interface{}{
			"jid":       jidString(evt.JID),
			"timestamp": rfc3339(evt.Timestamp),
			"pinned":    evt.Action.GetPinned(),
		}, !evt.FromFullSync, true

	case *events.Star:
		return "Star", map[string]interface{}{
			"chatJid":   jidString(evt.ChatJID),
			"senderJid": jidString(evt.SenderJID),
			"isFromMe":  evt.IsFromMe,
			"messageId": evt.MessageID,
			"timestamp": rfc3339(evt.Timestamp),
			"starred":   evt.Action.GetStarred(),
		}, !evt.FromFullSync, true

	case *events.MarkChatAsRead:
		return "MarkChatAsRead", map[string]interface{}{
			"jid":       jidString(evt.JID),
			"timestamp": rfc3339(evt.Timestamp),
			"read":      evt.Action.GetRead(),
		}, !evt.FromFullSync, true

	case *events.ClearChat:
		return "ClearChat", map[string]interface{}{
			"jid":         jidString(evt.JID),
			"timestamp":   rfc3339(evt.Timestamp),
			"deleteMedia": evt.DeleteMedia,
		}, !evt.FromFullSync, true

	case *events.DeleteChat:
		return "DeleteChat", map[string]interface{}{
			"jid":         jidString(evt.JID),
			"timestamp":   rfc3339(evt.Timestamp),
			"deleteMedia": evt.DeleteMedia,
		}, !evt.FromFullSync, true

	case *events.DeleteForMe:
		return "DeleteForMe", map[string]interface{}{
			"chatJid":     jidString(evt.ChatJID),
			"senderJid":   jidString(evt.SenderJID),
			"isFromMe":    evt.IsFromMe,
			"messageId":   evt.MessageID,
			"timestamp":   rfc3339(evt.Timestamp),
			"deleteMedia": evt.Action.GetDeleteMedia(),
		}, !evt.FromFullSync, true

	case *events.UnarchiveChatsSetting:
		return "UnarchiveChatsSetting", map[string]interface{}{
			"timestamp":      rfc3339(evt.Timestamp),
			"unarchiveChats": evt.Action.GetUnarchiveChats(),
		}, !evt.FromFullSync, true

	case *events.UserStatusMute:
		return "UserStatusMute", map[string]interface{}{
			"jid":       jidString(evt.JID),
			"timestamp": rfc3339(evt.Timestamp),
			"muted":     evt.Action.GetMuted(),
		}, !evt.FromFullSync, true
	}

	return "", nil, false, false
}
