package utils

import (
	"time"

	"go.mau.fi/whatsmeow"
)

const waitPollInterval = 50 * time.Millisecond

// InstanceStartTimeout is how long a request waits for an instance it just started to
// connect. The connection is awaited, not slept for: this is only the upper bound.
const InstanceStartTimeout = 10 * time.Second

// WaitForClient waits for the client of an instance that was just started.
//
// StartInstance runs the client in a goroutine, so the caller has to wait for it to
// appear in the registry and then to connect. Every service did that with a fixed
// time.Sleep(2s) followed by a check: a connection that took 2.1 s failed the request,
// and one that took 0.3 s still cost the full two seconds.
//
// It returns as soon as the client is connected and logged in, or when timeout passes.
// The client is returned even when it is not connected, so the caller can tell "no
// client" (nil) from "client, but not connected" and report it. A client without a
// paired device (Store.ID nil) is returned at once: it will not log in by waiting, and
// the caller reports "instance is not logged in" straight away (#77).
func WaitForClient(get func() *whatsmeow.Client, timeout time.Duration) *whatsmeow.Client {
	deadline := time.Now().Add(timeout)

	for {
		client := get()
		if client != nil {
			if client.Store == nil || client.Store.ID == nil {
				return client
			}
			client.WaitForConnection(time.Until(deadline))
			return client
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		time.Sleep(waitPollInterval)
	}
}

// WaitUntil polls cond until it is true or timeout passes, and reports whether it became
// true. It replaces a fixed sleep with an early exit.
func WaitUntil(timeout time.Duration, cond func() bool) bool {
	return WaitUntilEvery(timeout, waitPollInterval, cond)
}

// WaitUntilEvery is WaitUntil with the polling interval chosen by the caller (for
// conditions that cost a query).
func WaitUntilEvery(timeout, interval time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(interval)
	}
}
