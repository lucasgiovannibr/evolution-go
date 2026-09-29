package message_service

import "testing"

func TestRerequestResolve(t *testing.T) {
	// one-to-one: the sender defaults to the chat
	chat, sender, err := (&RerequestStruct{Chat: "5511999990001", MessageID: "ABC"}).resolve()
	if err != nil || chat.User != "5511999990001" || sender != chat {
		t.Fatalf("got %v %v %v", chat, sender, err)
	}

	// a leading "+" must not reach the message key
	chat, _, err = (&RerequestStruct{Chat: "+5511999990001", MessageID: "ABC"}).resolve()
	if err != nil || chat.User != "5511999990001" {
		t.Fatalf("chat must be canonical, got %v %v", chat, err)
	}

	// group: an explicit sender is kept
	chat, sender, err = (&RerequestStruct{Chat: "120363000000000001@g.us", MessageID: "ABC", Sender: "5511999990002@s.whatsapp.net"}).resolve()
	if err != nil || chat.Server != "g.us" || sender.User != "5511999990002" {
		t.Fatalf("got %v %v %v", chat, sender, err)
	}
}

func TestRerequestResolveRejects(t *testing.T) {
	bad := map[string]*RerequestStruct{
		"no id":            {Chat: "5511999990001"},
		"no chat":          {MessageID: "ABC"},
		"group no sender":  {Chat: "120363000000000001@g.us", MessageID: "ABC"},
		"channel":          {Chat: "120363000000000001@newsletter", MessageID: "ABC"},
		"status":           {Chat: "status@broadcast", MessageID: "ABC"},
		"unparseable chat": {Chat: "@@", MessageID: "ABC"},
	}
	for name, d := range bad {
		_, _, err := d.resolve()
		if err == nil || !IsRequestError(err) {
			t.Errorf("%s: want a request error, got %v", name, err)
		}
	}
}

func TestRerequestMessageValidatesBeforeTouchingTheClient(t *testing.T) {
	m := &messageService{} // no client: reaching it would panic
	if _, err := m.RerequestMessage(&RerequestStruct{Chat: "5511999990001"}, nil); err == nil || !IsRequestError(err) {
		t.Fatalf("got %v", err)
	}
}
