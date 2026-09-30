package call_stream

import (
	"testing"
	"time"
)

func newTestTickets() (*Tickets, *time.Time) {
	now := time.Unix(1_000_000, 0)
	t := NewTickets()
	t.now = func() time.Time { return now }
	return t, &now
}

func TestATicketWorksOnceForItsCall(t *testing.T) {
	tickets, _ := newTestTickets()
	token, ttl, err := tickets.Issue("inst", "C1")
	if err != nil {
		t.Fatal(err)
	}
	if ttl != TicketTTL || token == "" {
		t.Fatalf("token=%q ttl=%v", token, ttl)
	}

	if instance, ok := tickets.Redeem(token, "C1"); !ok || instance != "inst" {
		t.Fatalf("Redeem = %q, %v", instance, ok)
	}
	if _, ok := tickets.Redeem(token, "C1"); ok {
		t.Fatal("a ticket worked twice")
	}
}

func TestATicketIsBoundToOneCall(t *testing.T) {
	tickets, _ := newTestTickets()
	token, _, _ := tickets.Issue("inst", "C1")

	if _, ok := tickets.Redeem(token, "C2"); ok {
		t.Fatal("a ticket opened another call")
	}
	// the wrong attempt spent it: a guess cannot be retried with the right call
	if _, ok := tickets.Redeem(token, "C1"); ok {
		t.Fatal("a ticket survived a failed attempt")
	}
}

func TestATicketExpires(t *testing.T) {
	tickets, now := newTestTickets()
	token, _, _ := tickets.Issue("inst", "C1")

	*now = now.Add(TicketTTL)

	if _, ok := tickets.Redeem(token, "C1"); ok {
		t.Fatal("an expired ticket worked")
	}
}

func TestUnknownAndEmptyTicketsAreRefused(t *testing.T) {
	tickets, _ := newTestTickets()
	tickets.Issue("inst", "C1")
	for _, token := range []string{"", "nope", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		if _, ok := tickets.Redeem(token, "C1"); ok {
			t.Fatalf("ticket %q worked", token)
		}
	}
}

func TestTicketsAreDistinct(t *testing.T) {
	tickets, _ := newTestTickets()
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		token, _, err := tickets.Issue("inst", "C1")
		if err != nil {
			t.Fatal(err)
		}
		if seen[token] {
			t.Fatal("two tickets are the same")
		}
		seen[token] = true
	}
}

func TestThePendingTicketsAreBounded(t *testing.T) {
	tickets, now := newTestTickets()
	for i := 0; i < maxTickets; i++ {
		if _, _, err := tickets.Issue("inst", "C1"); err != nil {
			t.Fatalf("ticket %d: %v", i, err)
		}
	}
	if _, _, err := tickets.Issue("inst", "C1"); err != ErrTooManyTickets {
		t.Fatalf("err = %v, want ErrTooManyTickets", err)
	}

	// the expired ones make room again
	*now = now.Add(TicketTTL)
	if _, _, err := tickets.Issue("inst", "C1"); err != nil {
		t.Fatalf("expired tickets must be reclaimed: %v", err)
	}
}
