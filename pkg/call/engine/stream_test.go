package call_engine

import (
	"errors"
	"testing"
	"time"
)

type nullSink struct{}

func (nullSink) WriteFrame([]float32) error { return nil }
func (nullSink) Close() error               { return nil }

func audioOnly() Endpoints { return Endpoints{Sink: nullSink{}, Source: nullSource{}} }

type nullSource struct{}

func (nullSource) ReadFrame() ([]float32, error) { return nil, nil }
func (nullSource) Close() error                  { return nil }

func answered(f *fakeCall) *fakeCall {
	f.mu.Lock()
	f.phase = PhaseActive
	f.mu.Unlock()
	return f
}

func TestAnswerAnswersARingingIncomingCall(t *testing.T) {
	m, _ := newTestManager(Options{})
	c := newFake("C1")
	m.Track("inst", c, Incoming)

	tr, err := m.Answer("inst", "C1")

	if err != nil || tr == nil {
		t.Fatalf("err = %v", err)
	}
	if c.answered != 1 {
		t.Fatalf("answered %d times, want 1", c.answered)
	}
	if got := m.List("inst")[0].Phase; got != PhaseConnecting {
		t.Fatalf("phase = %s", got)
	}
}

func TestAnswerRefusesWhatIsNotARingingIncomingCall(t *testing.T) {
	m, _ := newTestManager(Options{})

	if _, err := m.Answer("inst", "nope"); !errors.Is(err, ErrCallNotFound) {
		t.Fatalf("unknown call: err = %v", err)
	}

	m.Track("inst", newFake("OUT"), Outgoing)
	if _, err := m.Answer("inst", "OUT"); !errors.Is(err, ErrWrongState) {
		t.Fatalf("outgoing call: err = %v", err)
	}

	active := answered(newFake("ACT"))
	m.Track("inst", active, Incoming)
	if _, err := m.Answer("inst", "ACT"); !errors.Is(err, ErrWrongState) {
		t.Fatalf("already answered: err = %v", err)
	}
	if active.answered != 0 {
		t.Fatal("answered a call that was not ringing")
	}

	if _, err := m.Answer("other", "OUT"); !errors.Is(err, ErrCallNotFound) {
		t.Fatalf("another instance's call: err = %v", err)
	}
}

func TestAnswerTwiceAnswersOnce(t *testing.T) {
	m, _ := newTestManager(Options{})
	c := newFake("C1")
	m.Track("inst", c, Incoming)

	m.Answer("inst", "C1")
	if _, err := m.Answer("inst", "C1"); !errors.Is(err, ErrWrongState) {
		t.Fatalf("second answer: err = %v", err)
	}
	if c.answered != 1 {
		t.Fatalf("answered %d times", c.answered)
	}
}

func TestAnswerReportsALibraryFailure(t *testing.T) {
	m, _ := newTestManager(Options{})
	c := newFake("C1")
	c.ansErr = errors.New("relay unreachable")
	m.Track("inst", c, Incoming)

	if _, err := m.Answer("inst", "C1"); err == nil || errors.Is(err, ErrWrongState) {
		t.Fatalf("err = %v, want the library's failure", err)
	}
}

func TestHangupRejectsARingingCallAndHangsUpTheRest(t *testing.T) {
	m, _ := newTestManager(Options{})
	ringing := newFake("R")
	running := answered(newFake("A"))
	m.Track("inst", ringing, Incoming)
	m.Track("inst", running, Incoming)

	tr, err := m.Hangup("inst", "R")
	if err != nil || tr.Reason() != "rejected" {
		t.Fatalf("ringing: err=%v reason=%q", err, tr.Reason())
	}
	if rej, hung := ringing.counts(); rej != 1 || hung != 0 {
		t.Fatalf("ringing call: rejected=%d hungUp=%d", rej, hung)
	}

	tr, err = m.Hangup("inst", "A")
	if err != nil || tr.Reason() != "hangup" {
		t.Fatalf("running: err=%v reason=%q", err, tr.Reason())
	}
	if rej, hung := running.counts(); rej != 0 || hung != 1 {
		t.Fatalf("running call: rejected=%d hungUp=%d", rej, hung)
	}
	if len(m.List("inst")) != 0 {
		t.Fatal("hung up calls are still listed")
	}

	if _, err := m.Hangup("inst", "A"); !errors.Is(err, ErrCallNotFound) {
		t.Fatalf("second hangup: err = %v", err)
	}
}

func TestHangupEndsTheCallEvenWhenTheNetworkIsGone(t *testing.T) {
	m, _ := newTestManager(Options{})
	c := answered(newFake("A"))
	c.netErr = errors.New("not connected")
	m.Track("inst", c, Incoming)

	tr, err := m.Hangup("inst", "A")

	if err == nil || !closed(tr) {
		t.Fatalf("err=%v closed=%v: the call must end and the caller must learn the peer may not know", err, closed(tr))
	}
}

func TestOnlyOneStreamPerCall(t *testing.T) {
	m, _ := newTestManager(Options{})
	c := newFake("C1")
	m.Track("inst", c, Incoming)

	_, detach, err := m.AttachStream("inst", "C1", audioOnly(), &StreamStats{})
	if err != nil {
		t.Fatal(err)
	}
	if sink, src := c.attached(); sink == nil || src == nil {
		t.Fatal("the sink and the source were not attached to the call")
	}

	if _, _, err := m.AttachStream("inst", "C1", audioOnly(), &StreamStats{}); !errors.Is(err, ErrStreamBusy) {
		t.Fatalf("second stream: err = %v", err)
	}

	detach()
	detach() // detaching twice is harmless
	if sink, _ := c.attached(); sink != nil {
		t.Fatal("the sink stays attached after detach")
	}
	if _, _, err := m.AttachStream("inst", "C1", audioOnly(), &StreamStats{}); err != nil {
		t.Fatalf("a new stream must be able to attach after the old one left: %v", err)
	}
}

func TestStreamOfAnUnknownOrEndedCall(t *testing.T) {
	m, _ := newTestManager(Options{})
	if _, _, err := m.AttachStream("inst", "nope", audioOnly(), &StreamStats{}); !errors.Is(err, ErrCallNotFound) {
		t.Fatalf("err = %v", err)
	}
	m.Track("other", newFake("C1"), Incoming)
	if _, _, err := m.AttachStream("inst", "C1", audioOnly(), &StreamStats{}); !errors.Is(err, ErrCallNotFound) {
		t.Fatalf("another instance's call: err = %v", err)
	}
}

func TestARunningCallWithoutAStreamIsHungUpAfterTheGrace(t *testing.T) {
	m, log := newTestManager(Options{StreamGrace: 20 * time.Millisecond})
	c := answered(newFake("C1"))
	tr, _ := m.Track("inst", c, Incoming)
	_, detach, _ := m.AttachStream("inst", "C1", audioOnly(), &StreamStats{})

	detach()

	select {
	case <-tr.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the call was never hung up")
	}
	if tr.Reason() != "stream_closed" {
		t.Fatalf("reason = %q", tr.Reason())
	}
	if _, hung := c.counts(); hung != 1 {
		t.Fatalf("hung up %d times", hung)
	}
	if e := log.waitEnded(t); e.data["reason"] != "stream_closed" {
		t.Fatalf("event = %+v", e)
	}
}

func TestAStreamThatComesBackKeepsTheCall(t *testing.T) {
	m, _ := newTestManager(Options{StreamGrace: 60 * time.Millisecond})
	c := answered(newFake("C1"))
	tr, _ := m.Track("inst", c, Incoming)
	_, detach, _ := m.AttachStream("inst", "C1", audioOnly(), &StreamStats{})
	detach()

	if _, _, err := m.AttachStream("inst", "C1", audioOnly(), &StreamStats{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	if closed(tr) {
		t.Fatal("the call was hung up although its stream came back in time")
	}
}

func TestACallThatWasNeverAnsweredHasNoStreamClock(t *testing.T) {
	m, _ := newTestManager(Options{StreamGrace: 20 * time.Millisecond})
	c := newFake("C1") // still ringing
	tr, _ := m.Track("inst", c, Incoming)
	_, detach, _ := m.AttachStream("inst", "C1", audioOnly(), &StreamStats{})

	detach()
	time.Sleep(120 * time.Millisecond)

	if closed(tr) {
		t.Fatal("a ringing call was dropped because its stream left")
	}
}

func TestACallThatEndsWhileStreamedDoesNotStartTheClock(t *testing.T) {
	m, _ := newTestManager(Options{StreamGrace: 20 * time.Millisecond})
	c := answered(newFake("C1"))
	tr, _ := m.Track("inst", c, Incoming)
	_, detach, _ := m.AttachStream("inst", "C1", audioOnly(), &StreamStats{})

	c.end("terminate")
	detach()
	time.Sleep(100 * time.Millisecond)

	if tr.Reason() != "terminate" {
		t.Fatalf("reason = %q, want the peer's", tr.Reason())
	}
	if _, hung := c.counts(); hung != 0 {
		t.Fatal("hung up a call that was already over")
	}
}

func TestStreamStatsAreReportedWithTheCall(t *testing.T) {
	m, _ := newTestManager(Options{})
	c := answered(newFake("C1"))
	m.Track("inst", c, Incoming)
	if got := m.List("inst")[0].Stream; got != nil {
		t.Fatalf("stream = %+v before any stream attached", got)
	}

	stats := &StreamStats{}
	_, detach, _ := m.AttachStream("inst", "C1", audioOnly(), stats)
	stats.ToClient.Add(3)
	stats.DroppedFromClient.Add(1)

	got := m.List("inst")[0].Stream
	if got == nil || !got.Attached || got.ToClient != 3 || got.DroppedFromClient != 1 {
		t.Fatalf("stream = %+v", got)
	}

	detach()
	if got := m.List("inst")[0].Stream; got == nil || got.Attached || got.ToClient != 3 {
		t.Fatalf("after detach: %+v", got)
	}
}
