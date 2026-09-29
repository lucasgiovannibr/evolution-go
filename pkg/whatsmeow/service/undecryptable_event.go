package whatsmeow_service

import (
	"go.mau.fi/whatsmeow/types/events"
)

// undecryptableEventData is the payload published when a message arrives that this
// device could not decrypt. Until now the only sign of it was a log line, so a
// "the message never arrived" report had nothing to start from.
//
// The id, chat and sender are exactly what POST /message/rerequest needs to ask the
// phone for another copy.
func undecryptableEventData(evt *events.UndecryptableMessage) map[string]interface{} {
	return map[string]interface{}{
		"id":              evt.Info.ID,
		"chat":            jidString(evt.Info.Chat),
		"sender":          jidString(evt.Info.Sender),
		"isGroup":         evt.Info.IsGroup,
		"isFromMe":        evt.Info.IsFromMe,
		"timestamp":       rfc3339(evt.Info.Timestamp),
		"pushName":        evt.Info.PushName,
		"isUnavailable":   evt.IsUnavailable,
		"unavailableType": string(evt.UnavailableType),
		"decryptFailMode": string(evt.DecryptFailMode),
	}
}
