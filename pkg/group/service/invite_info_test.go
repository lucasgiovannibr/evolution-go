package group_service

import "testing"

func TestNormalizeInviteCode(t *testing.T) {
	cases := map[string]string{
		"F9QXBSxONUk9t4WzMTI7Af":                                    "F9QXBSxONUk9t4WzMTI7Af",
		"https://chat.whatsapp.com/F9QXBSxONUk9t4WzMTI7Af":          "F9QXBSxONUk9t4WzMTI7Af",
		"chat.whatsapp.com/F9QXBSxONUk9t4WzMTI7Af":                  "F9QXBSxONUk9t4WzMTI7Af",
		"  https://chat.whatsapp.com/F9QXBSxONUk9t4WzMTI7Af/  ":     "F9QXBSxONUk9t4WzMTI7Af",
		"https://chat.whatsapp.com/F9QXBSxONUk9t4WzMTI7Af?mode=r_c": "F9QXBSxONUk9t4WzMTI7Af",
		"https://chat.whatsapp.com/F9QXBSxONUk9t4WzMTI7Af#top":      "F9QXBSxONUk9t4WzMTI7Af",
		"":  "",
		"/": "",
	}
	for in, want := range cases {
		if got := normalizeInviteCode(in); got != want {
			t.Errorf("normalizeInviteCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGroupInviteValidation(t *testing.T) {
	if err := (&GroupInviteStruct{}).validate(); err == nil {
		t.Fatal("an empty code must be rejected")
	}
	if err := (&GroupInviteStruct{Code: "ABC"}).validate(); err != nil {
		t.Fatalf("a link code alone is valid: %v", err)
	}
	if err := (&GroupInviteStruct{Code: "ABC", GroupJID: "120363000000000000@g.us"}).validate(); err == nil {
		t.Fatal("an invite message needs the inviter")
	}
	full := &GroupInviteStruct{Code: "ABC", GroupJID: "120363000000000000@g.us", Inviter: "5511999999999", Expiration: 1790000000}
	if err := full.validate(); err != nil || !full.IsInviteMessage() {
		t.Fatalf("a complete invite message must validate: %v", err)
	}
	if (&GroupInviteStruct{Code: "ABC"}).IsInviteMessage() {
		t.Fatal("without groupJid it is a link")
	}
}

func TestJoinGroupInviteRequiresAnInviteMessage(t *testing.T) {
	g := &groupService{}
	if err := g.JoinGroupInvite(&GroupInviteStruct{Code: "ABC"}, nil); err == nil {
		t.Fatal("joining from a bare link code on the invite-message route must be refused before touching the client")
	}
}
