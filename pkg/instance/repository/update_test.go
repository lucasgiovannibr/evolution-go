package instance_repository

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// One statement, one round trip, and connected/disconnect_reason always change together.
func TestUpdateConnectedIsASingleStatement(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "instances" SET "connected"=$1,"disconnect_reason"=$2 WHERE id = $3`)).
		WithArgs(false, "Logged out", "abc").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateConnected("abc", false, "Logged out"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// Pairing writes only the four columns it owns; the rest of the row is left alone.
func TestMarkPairedTouchesOnlyItsColumns(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "instances" SET "connected"=$1,"disconnect_reason"=$2,"jid"=$3,"qrcode"=$4 WHERE id = $5`)).
		WithArgs(true, "", "5511@s.whatsapp.net", "", "abc").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.MarkPaired("abc", "5511@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
