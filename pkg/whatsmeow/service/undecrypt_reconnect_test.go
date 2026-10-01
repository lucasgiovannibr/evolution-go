package whatsmeow_service

import (
	"testing"
	"time"
)

func TestUndecryptableReconnectHasACooldown(t *testing.T) {
	mycli := &MyClient{}
	now := time.Now()

	if !mycli.allowUndecryptReconnect(now) {
		t.Fatal("the first forced reconnect must be allowed")
	}
	if mycli.allowUndecryptReconnect(now.Add(time.Minute)) {
		t.Fatal("a second one within the cooldown must be refused: the message id is chosen by the sender")
	}
	if !mycli.allowUndecryptReconnect(now.Add(undecryptReconnectCooldown + time.Second)) {
		t.Fatal("after the cooldown a reconnect is allowed again")
	}
}
