package whatsmeow_service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// These tests need a PostgreSQL: PG_TEST_DSN=postgres://user:pass@host:5432/db?sslmode=disable
// Two lockers are two replicas.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_TEST_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(10)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func uniqueID(t *testing.T) string { return fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano()) }

func TestOnlyOneReplicaRunsAnInstance(t *testing.T) {
	db := testDB(t)
	a, b := NewPostgresLocker(db, nil), NewPostgresLocker(db, nil)
	defer a.Close()
	defer b.Close()
	id := uniqueID(t)
	ctx := context.Background()

	if ok, err := a.TryLock(ctx, id); err != nil || !ok {
		t.Fatalf("a: %v %v", ok, err)
	}
	if ok, _ := a.TryLock(ctx, id); !ok {
		t.Fatal("taking it twice in the same process must be a no-op")
	}
	if ok, err := b.TryLock(ctx, id); err != nil || ok {
		t.Fatalf("b took an instance a runs: %v %v", ok, err)
	}
	if free, _ := b.Probe(ctx, id); free {
		t.Fatal("probe says free while a holds it")
	}

	a.Unlock(id)
	if free, _ := b.Probe(ctx, id); !free {
		t.Fatal("not free after unlock")
	}
	if ok, _ := b.TryLock(ctx, id); !ok {
		t.Fatal("b could not take it after a let go")
	}
}

// A process that dies (its session closes) frees its instances without anyone cleaning up.
func TestLocksDieWithTheProcess(t *testing.T) {
	db := testDB(t)
	a, b := NewPostgresLocker(db, nil), NewPostgresLocker(db, nil)
	defer b.Close()
	id := uniqueID(t)
	ctx := context.Background()

	a.TryLock(ctx, id)
	a.Close() // as a crash: nobody calls Unlock
	deadline := time.Now().Add(3 * time.Second)
	for {
		if ok, _ := b.TryLock(ctx, id); ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the instance was not freed when the owner went away")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func terminateSessionOf(t *testing.T, db *sql.DB, key int64) {
	t.Helper()
	// Find the backend that holds the advisory lock and terminate it.
	_, err := db.Exec(`SELECT pg_terminate_backend(pid) FROM pg_locks WHERE locktype='advisory' AND granted AND ((classid::bigint << 32) | objid::bigint) = $1`, key)
	if err != nil {
		t.Fatal(err)
	}
}

// If the session is lost (network, failover) the watchdog opens another and takes the locks
// back, and nothing else gets the instance in between.
func TestLocksAreTakenBackAfterTheSessionIsLost(t *testing.T) {
	db := testDB(t)
	var lost sync.Map
	a := NewPostgresLocker(db, func(id string) { lost.Store(id, true) }).(*pgLocker)
	defer a.Close()
	id := uniqueID(t)
	ctx := context.Background()
	a.TryLock(ctx, id)

	terminateSessionOf(t, db, lockKey(id))
	time.Sleep(200 * time.Millisecond)
	a.check() // the watchdog tick

	b := NewPostgresLocker(db, nil)
	defer b.Close()
	if ok, _ := b.TryLock(ctx, id); ok {
		t.Fatal("another replica got the instance after a's session was restored")
	}
	if _, isLost := lost.Load(id); isLost {
		t.Fatal("the instance must not be reported lost")
	}
}

// If another replica took the instance while the session was gone, it is given up.
func TestInstanceIsGivenUpWhenAnotherReplicaTookItDuringTheOutage(t *testing.T) {
	db := testDB(t)
	lost := make(chan string, 1)
	a := NewPostgresLocker(db, func(id string) { lost <- id }).(*pgLocker)
	defer a.Close()
	b := NewPostgresLocker(db, nil)
	defer b.Close()
	id := uniqueID(t)
	ctx := context.Background()
	a.TryLock(ctx, id)

	terminateSessionOf(t, db, lockKey(id))
	deadline := time.Now().Add(3 * time.Second)
	for { // b takes over as soon as the server frees it
		if ok, _ := b.TryLock(ctx, id); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("b never got the freed instance")
		}
		time.Sleep(50 * time.Millisecond)
	}
	for i := 0; i < lockRelockTries; i++ {
		a.check()
	}
	select {
	case got := <-lost:
		if got != id {
			t.Fatalf("lost %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("the instance was not reported lost")
	}
}
