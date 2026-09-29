package newsletter_service

import (
	"encoding/json"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestChannelActionsRejectAnythingButAChannel(t *testing.T) {
	n := &newsletterService{} // no client: a valid request reaching it would panic
	for _, jid := range []types.JID{{}, types.NewJID("5511999990001", types.DefaultUserServer), types.NewJID("120363000000000001", types.GroupServer)} {
		for name, err := range map[string]error{
			"follow":   n.FollowNewsletter(&GetNewsletterStruct{JID: jid}, nil),
			"unfollow": n.UnfollowNewsletter(&GetNewsletterStruct{JID: jid}, nil),
			"mute":     n.MuteNewsletter(&NewsletterMuteStruct{JID: jid, Mute: true}, nil),
			"viewed":   n.MarkNewsletterViewed(&NewsletterMarkViewedStruct{JID: jid, ServerIDs: []int{1}}, nil),
			"react":    n.ReactNewsletter(&NewsletterReactStruct{JID: jid, ServerID: 1, Reaction: "👍"}, nil),
		} {
			if err == nil || !IsNewsletterRequestError(err) {
				t.Errorf("%s(%q): want a request error, got %v", name, jid.String(), err)
			}
		}
	}
}

func TestMarkViewedAndReactNeedTheMessage(t *testing.T) {
	ch := types.NewJID("120363000000000001", types.NewsletterServer)
	if err := (&NewsletterMarkViewedStruct{JID: ch}).Validate(); err == nil || !IsNewsletterRequestError(err) {
		t.Fatalf("no serverIds must be a request error, got %v", err)
	}
	if err := (&NewsletterReactStruct{JID: ch}).Validate(); err == nil || !IsNewsletterRequestError(err) {
		t.Fatalf("no serverId must be a request error, got %v", err)
	}
	if err := (&NewsletterMarkViewedStruct{JID: ch, ServerIDs: []int{7}}).Validate(); err != nil {
		t.Fatal(err)
	}
	// removing a reaction = empty emoji, which is valid
	if err := (&NewsletterReactStruct{JID: ch, ServerID: 7}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestBodiesDecodeFromJSON(t *testing.T) {
	var r NewsletterReactStruct
	if err := json.Unmarshal([]byte(`{"jid":"120363000000000001@newsletter","serverId":42,"reaction":"👍"}`), &r); err != nil {
		t.Fatal(err)
	}
	if r.JID.Server != types.NewsletterServer || r.ServerID != 42 || r.Reaction != "👍" {
		t.Fatalf("%+v", r)
	}
	var m NewsletterMarkViewedStruct
	if err := json.Unmarshal([]byte(`{"jid":"120363000000000001@newsletter","serverIds":[1,2,3]}`), &m); err != nil || len(m.ServerIDs) != 3 {
		t.Fatalf("%+v %v", m, err)
	}
}
