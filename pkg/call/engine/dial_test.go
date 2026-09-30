package call_engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// dialer is a DialFunc that hands out fake calls and remembers what it was asked.
type dialer struct {
	targets []string
	options []DialOptions
	calls   []*fakeCall
	err     error
	during  func() // runs while the call is being placed
}

func (d *dialer) dial(_ context.Context, _ string, target string, opts DialOptions) (Call, error) {
	d.targets = append(d.targets, target)
	d.options = append(d.options, opts)
	if d.during != nil {
		d.during()
	}
	if d.err != nil {
		return nil, d.err
	}
	c := newFake("OUT" + string(rune('A'+len(d.calls))))
	c.phase = PhaseCalling
	d.calls = append(d.calls, c)
	return c, nil
}

func TestDialPlacesAndTracksAnOutgoingCall(t *testing.T) {
	d := &dialer{}
	m, _ := newTestManager(Options{Dial: d.dial})

	tr, err := m.Dial(context.Background(), "inst", "5511999990000@s.whatsapp.net", DialOptions{})

	if err != nil {
		t.Fatal(err)
	}
	if len(d.targets) != 1 || d.targets[0] != "5511999990000@s.whatsapp.net" {
		t.Fatalf("targets = %v", d.targets)
	}
	info := tr.Info()
	if info.Direction != Outgoing || info.Phase != PhaseCalling {
		t.Fatalf("info = %+v", info)
	}
	if got := m.List("inst"); len(got) != 1 {
		t.Fatalf("List = %+v", got)
	}
}

func TestDialNeedsAWorkingEngine(t *testing.T) {
	d := &dialer{}
	m, _ := newTestManager(Options{Dial: d.dial})

	if _, err := m.Dial(context.Background(), "nobody", "x", DialOptions{}); !errors.Is(err, ErrEngineUnavailable) {
		t.Fatalf("instance without engine: err = %v", err)
	}

	m.runtimes["blocked"] = &runtime{status: Status{State: StateBlockedProxy}}
	if _, err := m.Dial(context.Background(), "blocked", "x", DialOptions{}); !errors.Is(err, ErrEngineUnavailable) {
		t.Fatalf("proxied instance: err = %v", err)
	}
	m.runtimes["broken"] = &runtime{status: Status{State: StateHookFailed}}
	if _, err := m.Dial(context.Background(), "broken", "x", DialOptions{}); !errors.Is(err, ErrEngineUnavailable) {
		t.Fatalf("instance whose hook failed: err = %v", err)
	}

	if len(d.targets) != 0 {
		t.Fatalf("placed %d calls without an engine", len(d.targets))
	}
}

func TestDialRespectsTheConcurrentCallLimitBeforePlacingAnything(t *testing.T) {
	d := &dialer{}
	m, _ := newTestManager(Options{MaxConcurrent: 1, Dial: d.dial})
	m.Track("inst", newFake("BUSY"), Incoming)

	_, err := m.Dial(context.Background(), "inst", "x", DialOptions{})

	if !errors.Is(err, ErrTooManyCalls) {
		t.Fatalf("err = %v", err)
	}
	if len(d.targets) != 0 {
		t.Fatal("a call was placed although the instance was at its limit")
	}
}

func TestDialIsRateLimitedPerInstance(t *testing.T) {
	d := &dialer{}
	m, _ := newTestManager(Options{DialsPerMinute: 2, MaxConcurrent: 50, Dial: d.dial})
	m.runtimes["other"] = &runtime{status: Status{State: StateActive}}
	now := time.Unix(1_000_000, 0)
	m.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if _, err := m.Dial(context.Background(), "inst", "x", DialOptions{}); err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
	}
	if _, err := m.Dial(context.Background(), "inst", "x", DialOptions{}); !errors.Is(err, ErrDialRateLimited) {
		t.Fatalf("third dial: err = %v", err)
	}
	if len(d.targets) != 2 {
		t.Fatalf("placed %d calls, want 2", len(d.targets))
	}

	// another instance has its own allowance
	if _, err := m.Dial(context.Background(), "other", "x", DialOptions{}); err != nil {
		t.Fatalf("other instance: %v", err)
	}

	// and the allowance comes back with time
	now = now.Add(61 * time.Second)
	if _, err := m.Dial(context.Background(), "inst", "x", DialOptions{}); err != nil {
		t.Fatalf("after a minute: %v", err)
	}
}

func TestAFailedAttemptStillCountsAgainstTheRate(t *testing.T) {
	d := &dialer{err: errors.New("no devices")}
	m, _ := newTestManager(Options{DialsPerMinute: 2, MaxConcurrent: 50, Dial: d.dial})

	for i := 0; i < 2; i++ {
		m.Dial(context.Background(), "inst", "x", DialOptions{})
	}
	if _, err := m.Dial(context.Background(), "inst", "x", DialOptions{}); !errors.Is(err, ErrDialRateLimited) {
		t.Fatalf("err = %v: failed attempts talk to WhatsApp too and must count", err)
	}
}

func TestAFailedDialTracksNothingAndSaysWhy(t *testing.T) {
	d := &dialer{err: errors.New("peer has no devices")}
	m, _ := newTestManager(Options{Dial: d.dial})

	_, err := m.Dial(context.Background(), "inst", "x", DialOptions{})

	if !errors.Is(err, ErrDialFailed) || !strings.Contains(err.Error(), "peer has no devices") {
		t.Fatalf("err = %v", err)
	}
	if got := m.List("inst"); len(got) != 0 {
		t.Fatalf("List = %+v", got)
	}
}

// Two dials may both pass the check on the limit and only one slot may be left when
// the second call is placed: that call must not be left ringing with nobody following it.
func TestACallThatLosesTheLastSlotIsHungUp(t *testing.T) {
	d := &dialer{}
	m, _ := newTestManager(Options{MaxConcurrent: 1, Dial: d.dial})
	d.during = func() { m.Track("inst", newFake("RACE"), Incoming) } // takes the slot mid-dial

	_, err := m.Dial(context.Background(), "inst", "x", DialOptions{})

	if !errors.Is(err, ErrTooManyCalls) {
		t.Fatalf("err = %v", err)
	}
	if _, hung := d.calls[0].counts(); hung != 1 {
		t.Fatalf("the orphan call was hung up %d times, want 1", hung)
	}
}

func TestAnOutgoingCallNobodyPicksUpIsHungUpAfterTheRingTimeout(t *testing.T) {
	d := &dialer{}
	m, _ := newTestManager(Options{RingTimeout: 20 * time.Millisecond, Dial: d.dial})
	tr, _ := m.Dial(context.Background(), "inst", "x", DialOptions{})

	select {
	case <-tr.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the outgoing call was never dropped")
	}

	rejected, hung := d.calls[0].counts()
	if rejected != 0 || hung != 1 {
		t.Fatalf("rejected=%d hungUp=%d: an outgoing call is hung up, never rejected", rejected, hung)
	}
	if tr.Reason() != "ring_timeout" {
		t.Fatalf("reason = %q", tr.Reason())
	}
}

func TestAnOutgoingCallCannotBeAnswered(t *testing.T) {
	d := &dialer{}
	m, _ := newTestManager(Options{Dial: d.dial})
	tr, _ := m.Dial(context.Background(), "inst", "x", DialOptions{})

	if _, err := m.Answer("inst", tr.Info().CallID); !errors.Is(err, ErrWrongState) {
		t.Fatalf("err = %v", err)
	}
}

func TestDialCanAskForAVideoCall(t *testing.T) {
	d := &dialer{}
	m, _ := newTestManager(Options{MaxConcurrent: 5, Dial: d.dial})

	m.Dial(context.Background(), "inst", "a", DialOptions{})
	m.Dial(context.Background(), "inst", "b", DialOptions{Video: true})

	if len(d.options) != 2 || d.options[0].Video || !d.options[1].Video {
		t.Fatalf("options = %+v", d.options)
	}
}
