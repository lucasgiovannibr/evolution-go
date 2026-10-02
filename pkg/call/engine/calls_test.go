package call_engine

import (
	"errors"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// fakeCall behaves like the library's call where it matters here: Reject and Hangup
// end the call locally (firing OnEnd) before they try the network, and fail when the
// network is gone.
type fakeCall struct {
	id    string
	video bool

	mu       sync.Mutex
	phase    Phase
	onReady  func()
	onEnd    func(string)
	rejected int
	hungUp   int
	netErr   error
	answered int
	ansErr   error
	sink     AudioSink
	src      AudioSource
	vsink    VideoSink
	onVS     func(VideoState)
	onKF     func()
}

func newFake(id string) *fakeCall { return &fakeCall{id: id, phase: PhaseRinging} }

func (f *fakeCall) ID() string      { return f.id }
func (f *fakeCall) IsVideo() bool   { return f.video }
func (f *fakeCall) Peer() types.JID { return types.NewJID("5511999990000", types.DefaultUserServer) }
func (f *fakeCall) Answer() error {
	f.mu.Lock()
	f.answered++
	f.phase = PhaseConnecting
	f.mu.Unlock()
	return f.answerErr()
}

func (f *fakeCall) answerErr() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ansErr
}

func (f *fakeCall) Receive(sink AudioSink) { f.mu.Lock(); f.sink = sink; f.mu.Unlock() }
func (f *fakeCall) Play(src AudioSource)   { f.mu.Lock(); f.src = src; f.mu.Unlock() }

func (f *fakeCall) attached() (AudioSink, AudioSource) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sink, f.src
}

func (f *fakeCall) Phase() Phase      { f.mu.Lock(); defer f.mu.Unlock(); return f.phase }
func (f *fakeCall) OnReady(fn func()) { f.mu.Lock(); f.onReady = fn; f.mu.Unlock() }
func (f *fakeCall) OnEnd(fn func(string)) {
	f.mu.Lock()
	f.onEnd = fn
	f.mu.Unlock()
}

func (f *fakeCall) ReceiveVideo(s VideoSink)              { f.mu.Lock(); f.vsink = s; f.mu.Unlock() }
func (f *fakeCall) SendVideo([]byte, time.Duration) error { return nil }
func (f *fakeCall) OnVideoState(fn func(VideoState))      { f.mu.Lock(); f.onVS = fn; f.mu.Unlock() }
func (f *fakeCall) OnVideoKeyframeRequest(fn func())      { f.mu.Lock(); f.onKF = fn; f.mu.Unlock() }
func (f *fakeCall) StartVideo() error                     { return nil }
func (f *fakeCall) AcceptVideo() error                    { return nil }
func (f *fakeCall) StopVideo() error                      { return nil }
func (f *fakeCall) SetVideoEnabled(bool) error            { return nil }
func (f *fakeCall) SetVideoOrientation(int) error         { return nil }
func (f *fakeCall) IsSendingVideo() bool                  { return false }
func (f *fakeCall) IsReceivingVideo() bool                { return false }

func (f *fakeCall) end(reason string) {
	f.mu.Lock()
	f.phase = PhaseEnded
	fn := f.onEnd
	f.mu.Unlock()
	if fn != nil {
		fn(reason)
	}
}

func (f *fakeCall) Reject() error {
	f.mu.Lock()
	f.rejected++
	err := f.netErr
	f.mu.Unlock()
	f.end("rejected")
	return err
}

func (f *fakeCall) Hangup() error {
	f.mu.Lock()
	f.hungUp++
	err := f.netErr
	f.mu.Unlock()
	f.end("hangup")
	return err
}

func (f *fakeCall) becomeReady() {
	f.mu.Lock()
	f.phase = PhaseActive
	fn := f.onReady
	f.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (f *fakeCall) counts() (rejected, hungUp int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rejected, f.hungUp
}

type event struct {
	instance, name string
	data           map[string]interface{}
}

type eventLog struct {
	mu     sync.Mutex
	events []event
}

func (l *eventLog) notify(instance, name string, data map[string]interface{}) {
	l.mu.Lock()
	l.events = append(l.events, event{instance, name, data})
	l.mu.Unlock()
}

func (l *eventLog) names() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := []string{}
	for _, e := range l.events {
		out = append(out, e.name)
	}
	return out
}

func (l *eventLog) last() event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.events[len(l.events)-1]
}

// waitEnded returns the latest CallEnded event, waiting for it: finish closes Done
// before it publishes the event, so a test that has just seen Done may be early.
func (l *eventLog) waitEnded(t *testing.T) event {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		l.mu.Lock()
		for i := len(l.events) - 1; i >= 0; i-- {
			if l.events[i].name == "CallEnded" {
				e := l.events[i]
				l.mu.Unlock()
				return e
			}
		}
		l.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("no CallEnded event was published")
		}
		time.Sleep(time.Millisecond)
	}
}

func newTestManager(opts Options) (*Manager, *eventLog) {
	log := &eventLog{}
	opts.Notify = log.notify
	m := NewManager(opts)
	// An engine per instance, as Attach leaves it, without a whatsmeow client.
	m.runtimes["inst"] = &runtime{status: Status{State: StateActive}, log: &recordingLogger{}}
	return m, log
}

func closed(t *Tracked) bool {
	select {
	case <-t.Done():
		return true
	default:
		return false
	}
}

// WhatsApp puts no reason in the terminate message of a call the other side hangs up
// (only the duration), so the library reports an empty one. It is named, so the event
// and the stream's stop do not leave a client guessing; a reason WhatsApp does give is
// kept (see TestTrackListsTheCallUntilItEnds).
func TestACallTheOtherSideEndedIsNotReportedWithoutAReason(t *testing.T) {
	m, log := newTestManager(Options{})
	c := answered(newFake("C1"))
	tr, _ := m.Track("inst", c, Incoming)

	c.end("")

	if !closed(tr) || tr.Reason() != ReasonPeerHangup {
		t.Fatalf("done=%v reason=%q, want %q", closed(tr), tr.Reason(), ReasonPeerHangup)
	}
	if e := log.waitEnded(t); e.data["reason"] != ReasonPeerHangup {
		t.Fatalf("event = %+v", e)
	}
}

func TestTrackListsTheCallUntilItEnds(t *testing.T) {
	m, log := newTestManager(Options{})
	c := newFake("C1")

	tr, err := m.Track("inst", c, Incoming)
	if err != nil {
		t.Fatal(err)
	}
	list := m.List("inst")
	if len(list) != 1 || list[0].CallID != "C1" || list[0].Direction != Incoming || list[0].Phase != PhaseRinging {
		t.Fatalf("List = %+v", list)
	}
	if st, _ := m.Status("inst"); st.ActiveCalls != 1 {
		t.Fatalf("ActiveCalls = %d, want 1", st.ActiveCalls)
	}

	c.end("terminate")

	if !closed(tr) || tr.Reason() != "terminate" {
		t.Fatalf("done=%v reason=%q", closed(tr), tr.Reason())
	}
	if got := m.List("inst"); len(got) != 0 {
		t.Fatalf("an ended call is still listed: %+v", got)
	}
	if st, _ := m.Status("inst"); st.ActiveCalls != 0 {
		t.Fatalf("ActiveCalls = %d, want 0", st.ActiveCalls)
	}
	ended := log.waitEnded(t)
	if ended.name != "CallEnded" || ended.data["reason"] != "terminate" || ended.data["callId"] != "C1" || ended.data["direction"] != "incoming" {
		t.Fatalf("event = %+v", ended)
	}
}

func TestTrackingTheSameCallTwiceKeepsOneEntry(t *testing.T) {
	m, _ := newTestManager(Options{})
	c := newFake("C1")
	first, _ := m.Track("inst", c, Incoming)
	second, err := m.Track("inst", c, Incoming)
	if err != nil || second != first || len(m.List("inst")) != 1 {
		t.Fatalf("second=%p first=%p err=%v list=%v", second, first, err, m.List("inst"))
	}
}

func TestCallsOfOneInstanceAreInvisibleToAnother(t *testing.T) {
	m, _ := newTestManager(Options{})
	m.Track("inst", newFake("C1"), Incoming)

	if _, ok := m.Get("other", "C1"); ok {
		t.Fatal("another instance can see the call")
	}
	if got := m.List("other"); len(got) != 0 {
		t.Fatalf("List(other) = %+v", got)
	}
	if tracked, _ := m.Reject("other", "C1"); tracked {
		t.Fatal("another instance rejected the call")
	}
}

func TestTheLimitRejectsTheNextIncomingCall(t *testing.T) {
	m, log := newTestManager(Options{MaxConcurrent: 2})
	m.Track("inst", newFake("C1"), Incoming)
	m.Track("inst", newFake("C2"), Incoming)

	if _, err := m.Track("inst", newFake("C3"), Outgoing); !errors.Is(err, ErrTooManyCalls) {
		t.Fatalf("err = %v, want ErrTooManyCalls", err)
	}

	third := newFake("C3")
	m.onIncoming("inst", third)

	if rejected, _ := third.counts(); rejected != 1 {
		t.Fatalf("the call over the limit was rejected %d times, want 1", rejected)
	}
	if len(m.List("inst")) != 2 {
		t.Fatalf("List = %+v", m.List("inst"))
	}
	if e := log.last(); e.name != "CallEnded" || e.data["reason"] != "rejected_busy" || e.data["callId"] != "C3" {
		t.Fatalf("event = %+v", e)
	}

	// a slot frees up when a call ends
	m.calls["inst"]["C1"].call.(*fakeCall).end("terminate")
	if _, err := m.Track("inst", newFake("C4"), Incoming); err != nil {
		t.Fatalf("a free slot must be usable: %v", err)
	}
}

func TestRejectEndsTheCallEvenWhenTheNetworkIsGone(t *testing.T) {
	m, _ := newTestManager(Options{})
	c := newFake("C1")
	c.netErr = errors.New("not connected")
	tr, _ := m.Track("inst", c, Incoming)

	tracked, err := m.Reject("inst", "C1")

	if !tracked || err == nil {
		t.Fatalf("tracked=%v err=%v: the caller must learn the peer may not have been told", tracked, err)
	}
	if !closed(tr) || len(m.List("inst")) != 0 {
		t.Fatal("the call must be forgotten even though sending failed")
	}
	if tr.Reason() != "rejected" {
		t.Fatalf("reason = %q", tr.Reason())
	}
}

func TestRejectOfAnUnknownCallSendsNothing(t *testing.T) {
	m, _ := newTestManager(Options{})
	if tracked, err := m.Reject("inst", "nope"); tracked || err != nil {
		t.Fatalf("tracked=%v err=%v", tracked, err)
	}
}

func TestReadyIsPublished(t *testing.T) {
	m, log := newTestManager(Options{})
	c := newFake("C1")
	m.Track("inst", c, Incoming)

	c.becomeReady()

	if got := log.names(); len(got) != 1 || got[0] != "CallReady" {
		t.Fatalf("events = %v", got)
	}
}

func TestAnUnansweredCallIsDroppedAfterTheRingTimeout(t *testing.T) {
	m, log := newTestManager(Options{RingTimeout: 20 * time.Millisecond})
	c := newFake("C1")
	tr, _ := m.Track("inst", c, Incoming)

	select {
	case <-tr.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the call was never dropped")
	}

	if rejected, _ := c.counts(); rejected != 1 {
		t.Fatalf("rejected %d times, want 1", rejected)
	}
	if tr.Reason() != "ring_timeout" {
		t.Fatalf("reason = %q, want ring_timeout", tr.Reason())
	}
	if e := log.waitEnded(t); e.data["reason"] != "ring_timeout" {
		t.Fatalf("event = %+v", e)
	}
}

func TestAnAnsweredCallSurvivesTheRingTimeout(t *testing.T) {
	m, _ := newTestManager(Options{RingTimeout: 20 * time.Millisecond})
	c := newFake("C1")
	tr, _ := m.Track("inst", c, Incoming)
	c.mu.Lock()
	c.phase = PhaseConnecting // answered through the API
	c.mu.Unlock()

	time.Sleep(120 * time.Millisecond)

	if closed(tr) {
		t.Fatal("an answered call was dropped by the ring timeout")
	}
	if rejected, hungUp := c.counts(); rejected+hungUp != 0 {
		t.Fatalf("rejected=%d hungUp=%d", rejected, hungUp)
	}
}

func TestDetachEndsEveryCallOfTheInstance(t *testing.T) {
	m, log := newTestManager(Options{})
	a, b := newFake("A"), newFake("B")
	ta, _ := m.Track("inst", a, Incoming)
	tb, _ := m.Track("inst", b, Outgoing)
	other, _ := m.Track("other", newFake("Z"), Incoming)

	m.Detach("inst")

	if !closed(ta) || !closed(tb) {
		t.Fatal("Detach must end the calls of the instance")
	}
	if ta.Reason() != "instance_stopped" || tb.Reason() != "instance_stopped" {
		t.Fatalf("reasons: %q %q", ta.Reason(), tb.Reason())
	}
	if closed(other) {
		t.Fatal("Detach touched another instance")
	}
	if got := m.List("inst"); len(got) != 0 {
		t.Fatalf("still listed: %+v", got)
	}

	// the hang up is sent in the background
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, ha := a.counts()
		_, hb := b.counts()
		if ha == 1 && hb == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("hangups: a=%d b=%d, want 1 each", ha, hb)
		}
		time.Sleep(5 * time.Millisecond)
	}

	n := 0
	for _, name := range log.names() {
		if name == "CallEnded" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("CallEnded published %d times, want once per call (2)", n)
	}
}

func TestReplacingTheEngineEndsTheCallsOfTheOldOne(t *testing.T) {
	m, _ := newTestManager(Options{})
	tr, _ := m.Track("inst", newFake("C1"), Incoming)

	m.store("inst", &runtime{status: Status{State: StateActive}})

	if !closed(tr) || tr.Reason() != "instance_stopped" {
		t.Fatalf("done=%v reason=%q", closed(tr), tr.Reason())
	}
}

func TestAnEndReportedTwiceIsPublishedOnce(t *testing.T) {
	m, log := newTestManager(Options{})
	c := newFake("C1")
	tr, _ := m.Track("inst", c, Incoming)

	c.end("terminate")
	m.finish("inst", tr, "again")
	m.Reject("inst", "C1")

	n := 0
	for _, name := range log.names() {
		if name == "CallEnded" {
			n++
		}
	}
	if n != 1 || tr.Reason() != "terminate" {
		t.Fatalf("CallEnded x%d, reason %q", n, tr.Reason())
	}
}

func TestATrackedCallAlreadyOverIsNotKept(t *testing.T) {
	m, _ := newTestManager(Options{})
	c := newFake("C1")
	c.phase = PhaseEnded

	tr, _ := m.Track("inst", c, Incoming)

	if !closed(tr) || len(m.List("inst")) != 0 {
		t.Fatal("a call that already ended must not stay in the registry")
	}
}

func TestANilManagerHasNoCalls(t *testing.T) {
	var m *Manager
	if _, ok := m.Get("inst", "C1"); ok {
		t.Fatal("Get on a nil Manager")
	}
	if got := m.List("inst"); got == nil || len(got) != 0 {
		t.Fatalf("List = %v, want an empty list", got)
	}
	m.Detach("inst")
}

func TestAPanickingSubscriberDoesNotBreakTheCallLifecycle(t *testing.T) {
	m := NewManager(Options{Notify: func(string, string, map[string]interface{}) { panic("boom") }})
	m.runtimes["inst"] = &runtime{status: Status{State: StateActive}, log: &recordingLogger{}}
	c := newFake("C1")
	tr, _ := m.Track("inst", c, Incoming)

	c.end("terminate")

	if !closed(tr) {
		t.Fatal("the call did not finish because the subscriber panicked")
	}
}

func TestACallTurnedAwayForBeingOverTheLimitIsRecordedAsBusy(t *testing.T) {
	m, _ := newTestManager(Options{MaxConcurrent: 1})
	var got []Record
	m.SetOnFinished(func(r Record) { got = append(got, r) })
	m.Track("inst", newFake("C1"), Incoming)

	m.onIncoming("inst", newFake("C2"))

	if len(got) != 1 {
		t.Fatalf("%d records, want 1", len(got))
	}
	if r := got[0]; r.CallID != "C2" || r.Direction != Incoming || r.Outcome != OutcomeBusy || r.Reason != "rejected_busy" || !r.AnsweredAt.IsZero() {
		t.Fatalf("record = %+v", r)
	}
}
