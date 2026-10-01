package whatsmeow_service

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

type nopJIDLog struct{}

func (nopJIDLog) LogInfo(string, ...interface{}) {}

func jid(s string) types.JID {
	j, err := types.ParseJID(s)
	if err != nil {
		panic(err)
	}
	return j
}

// WhatsApp reports some senders as @lid with the phone number as the alternate: the phone
// number becomes the sender (and the chat, when it is the @lid too) and the @lid the alternate.
func TestNormalizeMessageJIDsSwapsLIDAndPhone(t *testing.T) {
	info := types.MessageInfo{MessageSource: types.MessageSource{
		Sender:    jid("123456789@lid"),
		SenderAlt: jid("5511999999999@s.whatsapp.net"),
		Chat:      jid("123456789@lid"),
	}}
	normalizeMessageJIDs(nopJIDLog{}, "i", &info)
	if info.Sender.String() != "5511999999999@s.whatsapp.net" || info.Chat.String() != "5511999999999@s.whatsapp.net" || info.SenderAlt.String() != "123456789@lid" {
		t.Fatalf("sender %s alt %s chat %s", info.Sender, info.SenderAlt, info.Chat)
	}
}

func TestNormalizeMessageJIDsKeepsAGroupChatInTheSwapCase(t *testing.T) {
	info := types.MessageInfo{MessageSource: types.MessageSource{
		Sender:    jid("123456789@lid"),
		SenderAlt: jid("5511999999999@s.whatsapp.net"),
		Chat:      jid("120363000000000000@g.us"),
		IsGroup:   true,
	}}
	normalizeMessageJIDs(nopJIDLog{}, "i", &info)
	if info.Chat.String() != "120363000000000000@g.us" || info.Sender.String() != "5511999999999@s.whatsapp.net" {
		t.Fatalf("sender %s chat %s", info.Sender, info.Chat)
	}
}

// Without the swap case the ids are only cleaned: the device part is dropped.
func TestNormalizeMessageJIDsCleansDeviceSuffix(t *testing.T) {
	info := types.MessageInfo{MessageSource: types.MessageSource{
		Sender: jid("5511999999999:12@s.whatsapp.net"),
		Chat:   jid("5511999999999@s.whatsapp.net"),
	}}
	normalizeMessageJIDs(nopJIDLog{}, "i", &info)
	if info.Sender.String() != "5511999999999@s.whatsapp.net" {
		t.Fatalf("sender %s", info.Sender)
	}
}

func TestQuotedContext(t *testing.T) {
	quoted := &waE2E.Message{Conversation: proto.String("original")}
	ctx := &waE2E.ContextInfo{QuotedMessage: quoted, StanzaID: proto.String("ABC")}

	for name, m := range map[string]*waE2E.Message{
		"text":     {ExtendedTextMessage: &waE2E.ExtendedTextMessage{ContextInfo: ctx}},
		"image":    {ImageMessage: &waE2E.ImageMessage{ContextInfo: ctx}},
		"audio":    {AudioMessage: &waE2E.AudioMessage{ContextInfo: ctx}},
		"document": {DocumentMessage: &waE2E.DocumentMessage{ContextInfo: ctx}},
		"video":    {VideoMessage: &waE2E.VideoMessage{ContextInfo: ctx}},
	} {
		q, id := quotedContext(m)
		if q != quoted || id != "ABC" {
			t.Errorf("%s: got %v %q", name, q, id)
		}
	}
	if q, id := quotedContext(&waE2E.Message{Conversation: proto.String("plain")}); q != nil || id != "" {
		t.Fatalf("a plain message quotes nothing: %v %q", q, id)
	}
}

func TestButtonClickOfEachKind(t *testing.T) {
	legacy := buttonClickOf(&waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{
		SelectedButtonID: proto.String("b1"), Response: &waE2E.ButtonsResponseMessage_SelectedDisplayText{SelectedDisplayText: "Yes"}}})
	if legacy["type"] != "buttons_response" || legacy["buttonId"] != "b1" || legacy["buttonText"] != "Yes" {
		t.Fatalf("legacy: %v", legacy)
	}

	native := buttonClickOf(&waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
		InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
			NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
				Name: proto.String("quick_reply"), ParamsJSON: proto.String(`{"id":"q1","display_text":"Sure"}`)}}}})
	if native["type"] != "native_flow_response" || native["buttonId"] != "q1" || native["buttonText"] != "Sure" || native["name"] != "quick_reply" {
		t.Fatalf("native flow: %v", native)
	}
	// malformed params: the click is still reported, with empty id and text
	bad := buttonClickOf(&waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
		InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
			NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{ParamsJSON: proto.String("not json")}}}})
	if bad == nil || bad["buttonId"] != "" {
		t.Fatalf("malformed params: %v", bad)
	}

	tmpl := buttonClickOf(&waE2E.Message{TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{
		SelectedID: proto.String("t1"), SelectedDisplayText: proto.String("Go")}})
	if tmpl["type"] != "template_button_reply" || tmpl["buttonId"] != "t1" {
		t.Fatalf("template: %v", tmpl)
	}

	list := buttonClickOf(&waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{
		Title: proto.String("Option"), Description: proto.String("d"),
		SingleSelectReply: &waE2E.ListResponseMessage_SingleSelectReply{SelectedRowID: proto.String("r1")}}})
	if list["type"] != "list_response" || list["buttonId"] != "r1" || list["buttonText"] != "Option" || list["description"] != "d" {
		t.Fatalf("list: %v", list)
	}
}

func TestButtonClickOfOrdinaryMessages(t *testing.T) {
	for _, m := range []*waE2E.Message{nil, {}, {Conversation: proto.String("hi")},
		{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{}}} { // no native flow: nothing to report
		if got := buttonClickOf(m); got != nil {
			t.Fatalf("%+v is not a click: %v", m, got)
		}
	}
}
