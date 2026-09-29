package whatsmeow_service

import (
	"testing"
	"time"

	"go.mau.fi/util/jsontime"
	"go.mau.fi/whatsmeow/types/events"
)

func TestReachoutTimelockFromEvent(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	end := now.Add(72 * time.Hour)

	st := reachoutTimelockFromEvent(&events.NotifyAccountReachoutTimelock{
		EnforcementType:     "TEST_TYPE",
		IsActive:            true,
		TimeEnforcementEnds: jsontime.UnixString{Time: end},
	}, now)

	if !st.Active || st.EnforcementType != "TEST_TYPE" || st.EndsAt == nil || !st.EndsAt.Equal(end) {
		t.Fatalf("unexpected status: %#v", st)
	}
	if !st.InEffect(now) {
		t.Fatal("must be in effect before the end time")
	}
	if st.InEffect(end.Add(time.Second)) {
		t.Fatal("must not be in effect after the end time")
	}

	data := st.webhookData()
	if data["active"] != true || data["endsAt"] != end.Format(time.RFC3339) {
		t.Fatalf("unexpected webhook data: %#v", data)
	}
}

func TestReachoutTimelockUnknownEndAndInactive(t *testing.T) {
	now := time.Now()

	unknownEnd := reachoutTimelockFromEvent(&events.NotifyAccountReachoutTimelock{IsActive: true}, now)
	if unknownEnd.EndsAt != nil || !unknownEnd.InEffect(now.Add(1000*time.Hour)) {
		t.Fatalf("an active restriction without an end time stays in effect: %#v", unknownEnd)
	}
	if _, has := unknownEnd.webhookData()["endsAt"]; has {
		t.Fatal("an unknown end must not be published")
	}

	lifted := reachoutTimelockFromEvent(&events.NotifyAccountReachoutTimelock{IsActive: false}, now)
	if lifted.InEffect(now) {
		t.Fatal("a lifted restriction is not in effect")
	}

	var nilStatus *ReachoutTimelockStatus
	if nilStatus.InEffect(now) {
		t.Fatal("a nil status is not in effect")
	}
}

func TestStreamErrorFromEvent(t *testing.T) {
	now := time.Now()
	info := streamErrorFromEvent(&events.StreamError{Code: "weird"}, now)
	if info.Code != "weird" || !info.At.Equal(now) {
		t.Fatalf("unexpected info: %#v", info)
	}
	if info.webhookData()["code"] != "weird" {
		t.Fatalf("unexpected webhook data: %#v", info.webhookData())
	}
}

func TestInvalidateWebVersionCache(t *testing.T) {
	cachedWebVersionMu.Lock()
	cachedWebVersion = &clientVersion{Major: 2, Minor: 3000, Patch: 1}
	cachedWebVersionAt = time.Now()
	cachedWebVersionMu.Unlock()

	invalidateWebVersionCache()

	cachedWebVersionMu.Lock()
	defer cachedWebVersionMu.Unlock()
	if cachedWebVersion != nil {
		t.Fatal("the cached version must be dropped so the next connection fetches a fresh one")
	}
}

func TestRuntimeWarningsForOperationalEvents(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	end := now.Add(time.Hour)
	recent := now.Add(-5 * time.Minute)
	old := now.Add(-3 * time.Hour)

	healthy := RuntimeInfo{ClientRegistered: true, RuntimeActive: true, KillChannel: true, SupervisorCurrent: true, WebsocketConnected: true, LoggedIn: true, DeviceJID: "5511@s.whatsapp.net"}

	i := healthy
	i.ReachoutTimelock = &ReachoutTimelockStatus{Active: true, EndsAt: &end}
	i.LastStreamError = &StreamErrorInfo{Code: "x", At: recent}
	i.ClientOutdatedAt = &recent
	c := codes(runtimeWarningsAt(i, now))
	for _, want := range []string{"reachout_timelock_active", "recent_stream_error", "client_outdated"} {
		if !c[want] {
			t.Fatalf("missing warning %s in %v", want, c)
		}
	}

	stale := healthy
	stale.ReachoutTimelock = &ReachoutTimelockStatus{Active: false}
	stale.LastStreamError = &StreamErrorInfo{Code: "x", At: old}
	stale.ClientOutdatedAt = &old
	if got := runtimeWarningsAt(stale, now); len(got) != 0 {
		t.Fatalf("old or lifted events must not warn: %v", got)
	}
}
