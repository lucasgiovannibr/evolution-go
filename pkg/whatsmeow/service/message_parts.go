package whatsmeow_service

import (
	"encoding/json"
	"strings"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// Pieces of the received-message handling that need no client, so they can be tested.

// jidLogger is the part of the instance logger normalizeMessageJIDs uses.
type jidLogger interface {
	LogInfo(format string, args ...interface{})
}

// normalizeMessageJIDs cleans the sender JIDs of a received message and handles the case where
// WhatsApp reports the sender as a @lid with the phone number as the alternate: then Sender (and
// Chat, if it is a @lid too) become the @s.whatsapp.net JID and SenderAlt the @lid.
func normalizeMessageJIDs(log jidLogger, userID string, info *types.MessageInfo) {
	senderStr := info.Sender.String()
	senderAltStr := info.SenderAlt.String()
	chatStr := info.Chat.String()

	if strings.Contains(senderStr, "@lid") && strings.Contains(senderAltStr, "@s.whatsapp.net") {
		log.LogInfo("[%s] Detected LID/WhatsApp JID swap case - Sender: %s, SenderAlt: %s", userID, senderStr, senderAltStr)

		// Clean the ids before swapping them.
		cleanSenderAlt := cleanSenderID(senderAltStr)
		cleanSender := cleanSenderID(senderStr)

		if phoneJID, err := types.ParseJID(cleanSenderAlt); err == nil {
			info.Sender = phoneJID
			if strings.Contains(chatStr, "@lid") {
				info.Chat = phoneJID
			}
		}
		if lid, err := types.ParseJID(cleanSender); err == nil {
			info.SenderAlt = lid
		}

		log.LogInfo("[%s] JID swap completed - New Sender: %s, New SenderAlt: %s, New Chat: %s",
			userID, info.Sender.String(), info.SenderAlt.String(), info.Chat.String())
		return
	}

	// Normal case: only clean the ids.
	if jid, err := types.ParseJID(cleanSenderID(senderStr)); err == nil {
		info.Sender = jid
	}
	if lid, err := types.ParseJID(cleanSenderID(senderAltStr)); err == nil {
		info.SenderAlt = lid
	}
}

// quotedContext returns the message a message replies to and its id, for the kinds of message
// that carry a reply context here (text, image, audio, document, video).
func quotedContext(m *waE2E.Message) (*waE2E.Message, string) {
	switch {
	case m.GetExtendedTextMessage() != nil:
		ci := m.GetExtendedTextMessage().GetContextInfo()
		return ci.GetQuotedMessage(), ci.GetStanzaID()
	case m.GetImageMessage() != nil:
		ci := m.GetImageMessage().GetContextInfo()
		return ci.GetQuotedMessage(), ci.GetStanzaID()
	case m.GetAudioMessage() != nil:
		ci := m.GetAudioMessage().GetContextInfo()
		return ci.GetQuotedMessage(), ci.GetStanzaID()
	case m.GetDocumentMessage() != nil:
		ci := m.GetDocumentMessage().GetContextInfo()
		return ci.GetQuotedMessage(), ci.GetStanzaID()
	case m.GetVideoMessage() != nil:
		ci := m.GetVideoMessage().GetContextInfo()
		return ci.GetQuotedMessage(), ci.GetStanzaID()
	}
	return nil, ""
}

// buttonClickOf recognises the answers to interactive messages: legacy buttons, a native flow
// (quick reply, cta_url, cta_call, cta_copy), a template button and a list selection. It returns
// nil for any other message.
func buttonClickOf(m *waE2E.Message) map[string]interface{} {
	if resp := m.GetButtonsResponseMessage(); resp != nil {
		return map[string]interface{}{
			"buttonId":   resp.GetSelectedButtonID(),
			"buttonText": resp.GetSelectedDisplayText(),
			"type":       "buttons_response",
		}
	}
	if resp := m.GetInteractiveResponseMessage(); resp != nil {
		nf := resp.GetNativeFlowResponseMessage()
		if nf == nil {
			return nil
		}
		buttonID, buttonText := "", ""
		if nf.GetParamsJSON() != "" {
			var params map[string]interface{}
			if err := json.Unmarshal([]byte(nf.GetParamsJSON()), &params); err == nil {
				if id, ok := params["id"].(string); ok {
					buttonID = id
				}
				if dt, ok := params["display_text"].(string); ok {
					buttonText = dt
				}
			}
		}
		return map[string]interface{}{
			"buttonId":   buttonID,
			"buttonText": buttonText,
			"type":       "native_flow_response",
			"name":       nf.GetName(),
			"paramsJSON": nf.GetParamsJSON(),
		}
	}
	if resp := m.GetTemplateButtonReplyMessage(); resp != nil {
		return map[string]interface{}{
			"buttonId":   resp.GetSelectedID(),
			"buttonText": resp.GetSelectedDisplayText(),
			"type":       "template_button_reply",
		}
	}
	if resp := m.GetListResponseMessage(); resp != nil {
		return map[string]interface{}{
			"buttonId":    resp.GetSingleSelectReply().GetSelectedRowID(),
			"buttonText":  resp.GetTitle(),
			"type":        "list_response",
			"description": resp.GetDescription(),
		}
	}
	return nil
}
