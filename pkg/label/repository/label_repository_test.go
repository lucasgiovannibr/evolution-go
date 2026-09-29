package label_repository

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// A label deleted on the phone is removed for that instance and label id only.
func TestDeleteLabelByLabelIDScopesToTheInstanceAndTheLabel(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sqlmock db: %v", err)
	}
	defer sqlDB.Close()

	var logBuffer bytes.Buffer
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, WithoutReturning: true}), &gorm.Config{
		DryRun:                 true,
		SkipDefaultTransaction: true,
		Logger: gormlogger.New(log.New(&logBuffer, "", 0),
			gormlogger.Config{LogLevel: gormlogger.Info, Colorful: false}),
	})
	if err != nil {
		t.Fatalf("open gorm db: %v", err)
	}

	if err := NewLabelRepository(gormDB).DeleteLabelByLabelID("inst-1", "98"); err != nil {
		t.Fatal(err)
	}

	sql := logBuffer.String()
	for _, want := range []string{"DELETE FROM", "labels", "instance_id", "label_id"} {
		if !strings.Contains(sql, want) {
			t.Errorf("generated SQL lacks %q:\n%s", want, sql)
		}
	}
}
