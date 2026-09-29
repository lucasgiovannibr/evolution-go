package send_service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"go.mau.fi/whatsmeow"
)

// isReachoutError reports whether err is WhatsApp's answer 463
// (NackCallerReachoutTimelocked). whatsmeow only carries the code in the text of
// ErrServerReturnedError ("server returned error 463").
func isReachoutError(err error) bool {
	return err != nil &&
		errors.Is(err, whatsmeow.ErrServerReturnedError) &&
		strings.HasSuffix(strings.TrimSpace(err.Error()), " 463")
}

// explainSendError turns a bare "server returned error 463" into something a person
// can act on. The original error stays wrapped (errors.Is / the code in the text
// still work). lock is the restriction WhatsApp reported for the account, if any.
func explainSendError(err error, lock *whatsmeow_service.ReachoutTimelockStatus, now time.Time) error {
	if !isReachoutError(err) {
		return err
	}

	msg := "this WhatsApp account cannot start a conversation with a contact that has not messaged it first (reachout timelock)"
	if lock.InEffect(now) {
		msg = "WhatsApp restricted this account from starting conversations with new contacts"
		if lock.EndsAt != nil {
			msg += " until " + lock.EndsAt.Format(time.RFC3339)
		}
	}
	return fmt.Errorf("%w: %s. Messages to contacts that already talked to this number still work; otherwise wait or ask the contact to message first", err, msg)
}
