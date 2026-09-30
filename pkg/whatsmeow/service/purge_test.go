package whatsmeow_service

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
)

func newSQLiteContainer(t *testing.T) *sqlstore.Container {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_busy_timeout=5000", filepath.ToSlash(filepath.Join(t.TempDir(), "auth.db")))
	c, err := sqlstore.New(context.Background(), "sqlite", dsn, nil)
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	return c
}

// The stored device of a deleted instance is removed, and doing it again is harmless.
func TestPurgeDeviceRemovesTheStoredDevice(t *testing.T) {
	ctx := context.Background()
	c := newSQLiteContainer(t)

	jid := types.JID{User: "5531999990001", Device: 7, Server: types.DefaultUserServer}
	dev := c.NewDevice()
	dev.ID = &jid
	dev.Account = &waAdv.ADVSignedDeviceIdentity{Details: []byte{1}, AccountSignature: make([]byte, 64), AccountSignatureKey: make([]byte, 32), DeviceSignature: make([]byte, 64)}
	if err := dev.Save(ctx); err != nil {
		t.Fatalf("save device: %v", err)
	}
	if got, err := c.GetDevice(ctx, jid); err != nil || got == nil {
		t.Fatalf("the device must exist before the purge: %v %v", got, err)
	}

	if err := purgeDevice(ctx, c, jid.String()); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if got, err := c.GetDevice(ctx, jid); err != nil || got != nil {
		t.Fatalf("the device must be gone after the purge: %v %v", got, err)
	}

	if err := purgeDevice(ctx, c, jid.String()); err != nil {
		t.Fatalf("a second purge must be harmless: %v", err)
	}
}

func TestPurgeDeviceRejectsAnInvalidJID(t *testing.T) {
	if err := purgeDevice(context.Background(), newSQLiteContainer(t), "@@"); err == nil {
		t.Fatal("an invalid jid must be an error")
	}
}

func TestPurgeInstanceDataWithoutAJIDOrPollServiceDoesNothing(t *testing.T) {
	if err := (whatsmeowService{}).PurgeInstanceData("inst", ""); err != nil {
		t.Fatal(err)
	}
}
