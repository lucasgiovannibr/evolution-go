package whatsmeow_service

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
)

func group(user string) types.JID { return types.NewJID(user, types.GroupServer) }

func fetcher(calls *atomic.Int32, name string) func() (*types.GroupInfo, error) {
	return func() (*types.GroupInfo, error) {
		calls.Add(1)
		return &types.GroupInfo{GroupName: types.GroupName{Name: name}}, nil
	}
}

func TestGroupInfoIsAskedOnceForAStreamOfMessages(t *testing.T) {
	g := newGroupInfoCache(time.Minute)
	var calls atomic.Int32
	key := groupInfoKey("inst", group("120363"))

	for i := 0; i < 50; i++ {
		info, err := g.get(key, fetcher(&calls, "Team"))
		if err != nil || info.GroupName.Name != "Team" {
			t.Fatalf("get %d: %+v %v", i, info, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("50 messages of one group made %d GetGroupInfo calls, want 1", calls.Load())
	}
}

func TestGroupInfoIsDroppedWhenTheGroupChanges(t *testing.T) {
	g := newGroupInfoCache(time.Minute)
	var calls atomic.Int32
	key := groupInfoKey("inst", group("120363"))

	g.get(key, fetcher(&calls, "Old name"))
	g.forget(key) // what the GroupInfo / JoinedGroup event does
	info, _ := g.get(key, fetcher(&calls, "New name"))

	if calls.Load() != 2 || info.GroupName.Name != "New name" {
		t.Fatalf("after the group changed the next message must see the new info (calls %d, name %q)", calls.Load(), info.GroupName.Name)
	}
}

func TestGroupInfoErrorsAreNotRemembered(t *testing.T) {
	g := newGroupInfoCache(time.Minute)
	key := groupInfoKey("inst", group("1"))
	boom := errors.New("rate limited")

	if _, err := g.get(key, func() (*types.GroupInfo, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	var calls atomic.Int32
	if _, err := g.get(key, fetcher(&calls, "ok")); err != nil || calls.Load() != 1 {
		t.Fatalf("after a failure the next message asks again (err %v, calls %d)", err, calls.Load())
	}
}

func TestGroupInfoExpires(t *testing.T) {
	g := newGroupInfoCache(30 * time.Millisecond)
	var calls atomic.Int32
	key := groupInfoKey("inst", group("1"))
	g.get(key, fetcher(&calls, "a"))
	time.Sleep(80 * time.Millisecond)
	g.get(key, fetcher(&calls, "a"))
	if calls.Load() != 2 {
		t.Fatalf("an expired entry is fetched again, calls = %d", calls.Load())
	}
}

func TestGroupInfoKeysSeparateInstancesAndIgnoreTheDevice(t *testing.T) {
	a := groupInfoKey("inst-a", group("1"))
	b := groupInfoKey("inst-b", group("1"))
	if a == b {
		t.Fatal("two instances must not share a group entry")
	}
	withDevice := types.JID{User: "1", Server: types.GroupServer, Device: 3}
	if groupInfoKey("inst-a", withDevice) != a {
		t.Fatal("the device part must not make a new entry")
	}
}

func TestConcurrentMessagesOfOneGroupShareOneQuery(t *testing.T) {
	g := newGroupInfoCache(time.Minute)
	var calls atomic.Int32
	release := make(chan struct{})
	key := groupInfoKey("inst", group("1"))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.get(key, func() (*types.GroupInfo, error) {
				calls.Add(1)
				<-release
				return &types.GroupInfo{}, nil
			})
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("20 concurrent messages made %d queries, want 1", calls.Load())
	}
}

func TestGroupInfoForgetInstanceDropsOnlyThatInstance(t *testing.T) {
	g := newGroupInfoCache(time.Minute)
	var calls atomic.Int32
	ka, kb := groupInfoKey("a", group("1")), groupInfoKey("b", group("1"))
	g.get(ka, fetcher(&calls, "x"))
	g.get(kb, fetcher(&calls, "x"))

	g.forgetInstance("a")

	g.get(ka, fetcher(&calls, "x"))
	g.get(kb, fetcher(&calls, "x"))
	if calls.Load() != 3 {
		t.Fatalf("only instance a is refetched, calls = %d (want 3)", calls.Load())
	}
}
