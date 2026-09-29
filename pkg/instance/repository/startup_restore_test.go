package instance_repository

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockRepo(t *testing.T) (*instanceRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return &instanceRepository{db: gdb}, mock
}

// Instances caught mid-reconnect must be restored on startup too (see
// startupRestoreCondition), and only those besides the connected ones.
func TestGetAllConnectedInstancesIncludesReconnecting(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "instances" WHERE connected = $1 OR disconnect_reason = $2`)).
		WithArgs(true, ReconnectingReason).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, err := repo.GetAllConnectedInstances(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetAllConnectedInstancesByClientNameKeepsClientScope(t *testing.T) {
	repo, mock := newMockRepo(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "instances" WHERE (connected = $1 OR disconnect_reason = $2) AND client_name = $3`)).
		WithArgs(true, ReconnectingReason, "evolution").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, err := repo.GetAllConnectedInstancesByClientName("evolution"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
