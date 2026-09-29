package whatsmeow_service

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func chatJID(user string) types.JID { return types.NewJID(user, types.DefaultUserServer) }

func textWithExpiration(exp *uint32) *waE2E.Message {
	return &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text:        proto.String("oi"),
		ContextInfo: &waE2E.ContextInfo{Expiration: exp},
	}}
}

func TestLearnTimerFromReceivedMessage(t *testing.T) {
	const inst = "learn-msg"
	defer forgetInstanceChatTimers(inst)
	chat := chatJID("5511999990001")
	now := time.Now()

	learnChatTimerFromMessage(inst, &events.Message{
		Info:    types.MessageInfo{MessageSource: types.MessageSource{Chat: chat}},
		Message: textWithExpiration(proto.Uint32(86400)),
	}, now)

	if s, ok := lookupChatTimer(inst, chat, now); !ok || s != 86400 {
		t.Fatalf("got (%d,%v), want (86400,true)", s, ok)
	}
}

func TestMessageWithoutTimerDoesNotResetAKnownOne(t *testing.T) {
	const inst = "learn-noreset"
	defer forgetInstanceChatTimers(inst)
	chat := chatJID("5511999990002")
	now := time.Now()
	rememberChatTimer(inst, chat, 604800, now)

	// Older clients omit the expiration, and an explicit zero on a message is not
	// how "off" is announced: neither may clear the timer.
	for _, exp := range []*uint32{nil, proto.Uint32(0)} {
		learnChatTimerFromMessage(inst, &events.Message{
			Info:    types.MessageInfo{MessageSource: types.MessageSource{Chat: chat}},
			Message: textWithExpiration(exp),
		}, now)
	}
	if s, ok := lookupChatTimer(inst, chat, now); !ok || s != 604800 {
		t.Fatalf("got (%d,%v), the known timer must survive", s, ok)
	}
}

func TestEphemeralSettingMessageIsAuthoritativeIncludingOff(t *testing.T) {
	const inst = "learn-setting"
	defer forgetInstanceChatTimers(inst)
	chat := chatJID("5511999990003")
	now := time.Now()

	setting := func(sec uint32) *events.Message {
		return &events.Message{
			Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat}},
			Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
				Type:                waE2E.ProtocolMessage_EPHEMERAL_SETTING.Enum(),
				EphemeralExpiration: proto.Uint32(sec),
			}},
		}
	}

	learnChatTimerFromMessage(inst, setting(7776000), now)
	if s, ok := lookupChatTimer(inst, chat, now); !ok || s != 7776000 {
		t.Fatalf("got (%d,%v), want (7776000,true)", s, ok)
	}
	learnChatTimerFromMessage(inst, setting(0), now)
	if s, ok := lookupChatTimer(inst, chat, now); !ok || s != 0 {
		t.Fatalf("turning it off must be remembered as known-zero, got (%d,%v)", s, ok)
	}
}

func TestLearnTimerFromGroup(t *testing.T) {
	const inst = "learn-group"
	defer forgetInstanceChatTimers(inst)
	group := types.NewJID("120363000000000001", types.GroupServer)
	now := time.Now()

	learnChatTimerFromGroup(inst, group, &types.GroupEphemeral{IsEphemeral: true, DisappearingTimer: 86400}, now)
	if s, ok := lookupChatTimer(inst, group, now); !ok || s != 86400 {
		t.Fatalf("got (%d,%v), want (86400,true)", s, ok)
	}
	learnChatTimerFromGroup(inst, group, &types.GroupEphemeral{IsEphemeral: false, DisappearingTimer: 86400}, now)
	if s, ok := lookupChatTimer(inst, group, now); !ok || s != 0 {
		t.Fatalf("a group that is not ephemeral has no timer, got (%d,%v)", s, ok)
	}
	learnChatTimerFromGroup(inst, group, nil, now) // no ephemeral info: no change
	if _, ok := lookupChatTimer(inst, group, now); !ok {
		t.Fatal("nil info must not erase what is known")
	}
}

func TestTimerExpiresAfterTTLAndIsPerInstance(t *testing.T) {
	const a, b = "ttl-a", "ttl-b"
	defer forgetInstanceChatTimers(a)
	defer forgetInstanceChatTimers(b)
	chat := chatJID("5511999990004")
	now := time.Now()

	rememberChatTimer(a, chat, 86400, now)
	if _, ok := lookupChatTimer(b, chat, now); ok {
		t.Fatal("another instance must not see it")
	}
	if _, ok := lookupChatTimer(a, chat, now.Add(chatTimerTTL-time.Minute)); !ok {
		t.Fatal("still fresh")
	}
	if _, ok := lookupChatTimer(a, chat, now.Add(chatTimerTTL+time.Minute)); ok {
		t.Fatal("an entry never refreshed must be forgotten")
	}
}

func TestForgetInstanceDropsOnlyThatInstance(t *testing.T) {
	chat := chatJID("5511999990005")
	now := time.Now()
	rememberChatTimer("forget-a", chat, 86400, now)
	rememberChatTimer("forget-ab", chat, 86400, now)
	defer forgetInstanceChatTimers("forget-ab")

	forgetInstanceChatTimers("forget-a")
	if _, ok := lookupChatTimer("forget-a", chat, now); ok {
		t.Fatal("forgotten")
	}
	if _, ok := lookupChatTimer("forget-ab", chat, now); !ok {
		t.Fatal("an instance whose id merely starts the same must be kept")
	}
}

func TestBroadcastAndNewsletterAreNeverRemembered(t *testing.T) {
	const inst = "skip"
	defer forgetInstanceChatTimers(inst)
	now := time.Now()
	for _, j := range []types.JID{types.StatusBroadcastJID, types.NewJID("123", types.NewsletterServer), {}} {
		rememberChatTimer(inst, j, 86400, now)
		if _, ok := lookupChatTimer(inst, j, now); ok {
			t.Fatalf("%q must not be remembered", j.String())
		}
	}
}

func TestMessageExpirationReadsContextInfo(t *testing.T) {
	if s, ok := messageExpiration(textWithExpiration(proto.Uint32(3600))); !ok || s != 3600 {
		t.Fatalf("got (%d,%v)", s, ok)
	}
	if _, ok := messageExpiration(textWithExpiration(nil)); ok {
		t.Fatal("no expiration set")
	}
	if _, ok := messageExpiration(&waE2E.Message{Conversation: proto.String("x")}); ok {
		t.Fatal("a plain conversation has no ContextInfo")
	}
	if _, ok := messageExpiration(nil); ok {
		t.Fatal("nil message")
	}
}
