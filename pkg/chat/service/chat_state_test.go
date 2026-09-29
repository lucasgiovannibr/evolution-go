package chat_service

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
)

func TestParseChatIsCanonical(t *testing.T) {
	// CreateJID prefixes a phone number with "+" (and applies the project's Brazilian
	// ninth-digit normalization); the patch index must not carry the "+".
	for _, in := range []string{"5531999990001", "+5531999990001"} {
		jid, err := parseChat(in)
		if err != nil || jid.User != "553199990001" || jid.Server != types.DefaultUserServer {
			t.Errorf("parseChat(%q) = %v, %v", in, jid, err)
		}
	}
	g, err := parseChat("120363000000000001@g.us")
	if err != nil || g.Server != types.GroupServer {
		t.Fatalf("group: %v %v", g, err)
	}
	l, err := parseChat("273117121392855@lid")
	if err != nil || l.Server != types.HiddenUserServer {
		t.Fatalf("lid: %v %v", l, err)
	}
}

func TestParseChatRejects(t *testing.T) {
	for _, in := range []string{"", "   ", "@@"} {
		if _, err := parseChat(in); err == nil || !IsDisappearingRequestError(err) {
			t.Errorf("parseChat(%q) must be a request error, got %v", in, err)
		}
	}
}

func TestParseMuteDuration(t *testing.T) {
	ok := map[string]time.Duration{
		"": time.Hour, "8h": 8 * time.Hour, "1w": 7 * 24 * time.Hour, "7d": 7 * 24 * time.Hour,
		"1d": 24 * time.Hour, "always": muteAlways, " Always ": muteAlways, "30m": 30 * time.Minute,
	}
	for in, want := range ok {
		if got, err := parseMuteDuration(in); err != nil || got != want {
			t.Errorf("parseMuteDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"abc", "-5m", "0s"} {
		if _, err := parseMuteDuration(bad); err == nil || !IsDisappearingRequestError(err) {
			t.Errorf("parseMuteDuration(%q) must be a request error, got %v", bad, err)
		}
	}
}

func TestChatStateValidatesBeforeTouchingTheClient(t *testing.T) {
	c := &chatService{} // no client: reaching it would panic
	for name, err := range map[string]error{
		"pin":      first(c.ChatPin(&BodyStruct{Chat: "@@"}, nil)),
		"unpin":    first(c.ChatUnpin(&BodyStruct{}, nil)),
		"archive":  first(c.ChatArchive(&BodyStruct{Chat: "@@"}, nil)),
		"mute bad": first(c.ChatMute(&BodyStruct{Chat: "5531999990001", Duration: "abc"}, nil)),
	} {
		if err == nil || !IsDisappearingRequestError(err) {
			t.Errorf("%s: want a request error, got %v", name, err)
		}
	}
}

func first(_ string, err error) error { return err }

func TestAppStateChatJIDWithoutAClientKeepsTheJID(t *testing.T) {
	pn := types.NewJID("5531999990001", types.DefaultUserServer)
	if got := appStateChatJID(nil, pn); got != pn {
		t.Fatalf("got %v", got)
	}
	g := types.NewJID("120363000000000001", types.GroupServer)
	if got := appStateChatJID(nil, g); got != g {
		t.Fatalf("got %v", got)
	}
}
