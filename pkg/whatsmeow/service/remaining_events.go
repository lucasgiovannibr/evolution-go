package whatsmeow_service

import (
	"encoding/json"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// The last events whatsmeow emitted that the project dropped into the generic
// "Unhandled event" log line. Where each one goes:
//
//	CONTACT     Blocklist, PrivacySettings, BusinessName
//	CALL        CallPreAccept, CallReject, CallTransport, UnknownCallEvent
//	MESSAGE     MediaRetry
//	NEWSLETTER  NewsletterLiveUpdate, NewsletterMuteChange
//	CONNECTION  OfflineSyncPreview
//
// RotateADVSecret and ManualLoginReconnect are handled but never published (see
// remainingEventData).
//
// PushNameSetting was already handled (it refreshes the "Connected" state), so it is not
// listed here.
//
// Not handled on purpose, because whatsmeow never delivers them on their own: BlocklistChange
// (it is inside Blocklist.Changes), MediaRetryError (inside MediaRetry.Error),
// NewsletterMessageMeta (a field of the newsletter Message event) and MexNotificationData
// (a field of the mex events). FBMessage only exists for Messenger/Instagram sessions
// (MessengerConfig), which this project never sets.

// toMap turns a JSON-serializable value into a generic map/slice for the payload.
func toMap(v interface{}) interface{} {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

func callMetaData(meta types.BasicCallMeta) map[string]interface{} {
	return map[string]interface{}{
		"from":           jidString(meta.From),
		"timestamp":      rfc3339(meta.Timestamp),
		"callCreator":    jidString(meta.CallCreator),
		"callCreatorAlt": jidString(meta.CallCreatorAlt),
		"callId":         meta.CallID,
		"groupJid":       jidString(meta.GroupJID),
	}
}

// remainingEventData converts one of those events into the name and payload that are
// published. publish is false for events that are handled but must not go out; handled
// is false for every other event type.
func remainingEventData(rawEvt interface{}) (name string, data map[string]interface{}, publish bool, handled bool) {
	switch evt := rawEvt.(type) {
	case *events.Blocklist:
		changes := make([]map[string]interface{}, 0, len(evt.Changes))
		for _, c := range evt.Changes {
			changes = append(changes, map[string]interface{}{"jid": jidString(c.JID), "action": string(c.Action)})
		}
		// action "modify" (and no changes) means the whole list must be fetched again.
		return "Blocklist", map[string]interface{}{
			"action":    string(evt.Action),
			"dhash":     evt.DHash,
			"prevDhash": evt.PrevDHash,
			"changes":   changes,
		}, true, true

	case *events.PrivacySettings:
		return "PrivacySettings", map[string]interface{}{
			"settings": toMap(evt.NewSettings),
			"changed": map[string]interface{}{
				"groupAdd":     evt.GroupAddChanged,
				"lastSeen":     evt.LastSeenChanged,
				"status":       evt.StatusChanged,
				"profile":      evt.ProfileChanged,
				"readReceipts": evt.ReadReceiptsChanged,
				"online":       evt.OnlineChanged,
				"callAdd":      evt.CallAddChanged,
				"messages":     evt.MessagesChanged,
				"defense":      evt.DefenseChanged,
				"stickers":     evt.StickersChanged,
			},
		}, true, true

	case *events.BusinessName:
		data = map[string]interface{}{
			"jid":             jidString(evt.JID),
			"oldBusinessName": evt.OldBusinessName,
			"newBusinessName": evt.NewBusinessName,
		}
		if evt.Message != nil {
			data["messageId"] = evt.Message.ID
			data["chat"] = jidString(evt.Message.Chat)
		}
		return "BusinessName", data, true, true

	case *events.CallPreAccept:
		data = callMetaData(evt.BasicCallMeta)
		data["remotePlatform"] = evt.RemotePlatform
		data["remoteVersion"] = evt.RemoteVersion
		return "CallPreAccept", data, true, true

	case *events.CallTransport:
		data = callMetaData(evt.BasicCallMeta)
		data["remotePlatform"] = evt.RemotePlatform
		data["remoteVersion"] = evt.RemoteVersion
		return "CallTransport", data, true, true

	case *events.CallReject:
		return "CallReject", callMetaData(evt.BasicCallMeta), true, true

	case *events.UnknownCallEvent:
		data = map[string]interface{}{}
		if evt.Node != nil {
			attrs := make(map[string]string, len(evt.Node.Attrs))
			for k, v := range evt.Node.Attrs {
				attrs[k] = toString(v)
			}
			data["tag"] = evt.Node.Tag
			data["attrs"] = attrs
		}
		return "UnknownCallEvent", data, true, true

	case *events.MediaRetry:
		// The ciphertext is only useful to whoever holds the media key, and the project
		// does not retry downloads on its own: publish which message needs a new upload.
		data = map[string]interface{}{
			"messageId":     evt.MessageID,
			"chat":          jidString(evt.ChatID),
			"sender":        jidString(evt.SenderID),
			"fromMe":        evt.FromMe,
			"timestamp":     rfc3339(evt.Timestamp),
			"hasCiphertext": len(evt.Ciphertext) > 0,
		}
		if evt.Error != nil {
			data["errorCode"] = evt.Error.Code
		}
		return "MediaRetry", data, true, true

	case *events.NewsletterLiveUpdate:
		return "NewsletterLiveUpdate", map[string]interface{}{
			"jid":      jidString(evt.JID),
			"time":     rfc3339(evt.Time),
			"messages": toMap(evt.Messages),
		}, true, true

	case *events.NewsletterMuteChange:
		return "NewsletterMuteChange", map[string]interface{}{
			"id":   jidString(evt.ID),
			"mute": string(evt.Mute),
		}, true, true

	case *events.OfflineSyncPreview:
		return "OfflineSyncPreview", map[string]interface{}{
			"total":          evt.Total,
			"appDataChanges": evt.AppDataChanges,
			"messages":       evt.Messages,
			"notifications":  evt.Notifications,
			"receipts":       evt.Receipts,
		}, true, true

	case *events.RotateADVSecret, *events.ManualLoginReconnect:
		// Handled so that they stop falling into the generic log line, which printed
		// them with %+v: for RotateADVSecret that wrote the old and the NEW ADV secret
		// of the session into the instance log. Never published, never logged with
		// their content.
		return "", nil, false, true
	}
	return "", nil, false, false
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case types.JID:
		return t.String()
	case interface{ String() string }:
		return t.String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
