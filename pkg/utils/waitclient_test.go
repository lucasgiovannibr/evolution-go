package utils

import (
	"sync/atomic"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

func TestWaitForClientGivesUpWhenThereIsNoClient(t *testing.T) {
	start := time.Now()
	if c := WaitForClient(func() *whatsmeow.Client { return nil }, 200*time.Millisecond); c != nil {
		t.Fatal("no client: must return nil")
	}
	if e := time.Since(start); e < 150*time.Millisecond || e > 2*time.Second {
		t.Fatalf("waited %v", e)
	}
}

func TestWaitForClientPicksUpAClientThatAppearsLater(t *testing.T) {
	var calls atomic.Int32
	want := &whatsmeow.Client{Store: &store.Device{}} // unpaired: returned at once
	got := WaitForClient(func() *whatsmeow.Client {
		if calls.Add(1) < 4 {
			return nil
		}
		return want
	}, 5*time.Second)
	if got != want {
		t.Fatal("must return the client once it exists")
	}
}

// An unpaired client is not waited for: it will not log in by waiting (#77).
func TestWaitForClientReturnsAnUnpairedClientImmediately(t *testing.T) {
	c := &whatsmeow.Client{Store: &store.Device{}}
	start := time.Now()
	if got := WaitForClient(func() *whatsmeow.Client { return c }, 10*time.Second); got != c {
		t.Fatal("must return the client")
	}
	if e := time.Since(start); e > time.Second {
		t.Fatalf("took %v for an unpaired client", e)
	}
}

// A paired client that never connects is returned after the timeout, not forever.
func TestWaitForClientReturnsAPairedButDisconnectedClientAfterTheTimeout(t *testing.T) {
	id := types.NewJID("5531999990001", types.DefaultUserServer)
	c := whatsmeow.NewClient(&store.Device{ID: &id}, nil)
	start := time.Now()
	if got := WaitForClient(func() *whatsmeow.Client { return c }, 300*time.Millisecond); got != c {
		t.Fatal("must return the client")
	}
	if e := time.Since(start); e < 250*time.Millisecond || e > 3*time.Second {
		t.Fatalf("waited %v, want about 300ms", e)
	}
}

func TestWaitUntil(t *testing.T) {
	var n atomic.Int32
	start := time.Now()
	if !WaitUntil(5*time.Second, func() bool { return n.Add(1) >= 3 }) {
		t.Fatal("must become true")
	}
	if time.Since(start) > time.Second {
		t.Fatal("must exit as soon as the condition holds")
	}
	if WaitUntil(100*time.Millisecond, func() bool { return false }) {
		t.Fatal("never true: must report false")
	}
}
