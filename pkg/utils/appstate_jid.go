package utils

import (
	"context"

	"go.mau.fi/whatsmeow"
	whatsmeow_types "go.mau.fi/whatsmeow/types"
)

// AppStateChatJID returns the JID to write in an app-state patch (pin, archive, mute,
// chat and message labels) for a one-to-one chat.
//
// The phone matches a patch to a chat by the JID in the patch's index, and it keys
// one-to-one chats by LID, not by phone number. A phone-number JID is therefore
// resolved to its LID whenever the mapping is known. The JID must already be canonical
// (see CanonicalJID): with the "+" that CreateJID adds, the patch targets a chat that
// does not exist and is silently ignored. Groups and LIDs are returned untouched.
func AppStateChatJID(client *whatsmeow.Client, jid whatsmeow_types.JID) whatsmeow_types.JID {
	if jid.Server != whatsmeow_types.DefaultUserServer || client == nil || client.Store == nil || client.Store.LIDs == nil {
		return jid
	}
	if lid, err := client.Store.LIDs.GetLIDForPN(context.Background(), jid); err == nil && !lid.IsEmpty() {
		return lid.ToNonAD()
	}
	return jid
}
