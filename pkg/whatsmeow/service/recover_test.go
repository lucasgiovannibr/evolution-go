package whatsmeow_service

import (
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/config"
	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
)

func TestRecoverAndLogContainsPanic(t *testing.T) {
	lw := logger_wrapper.NewLoggerManager(&config.Config{LogDirectory: t.TempDir()})

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer recoverAndLog(lw, "inst", "test")
		panic("boom")
	}()
	<-done // reaching here means the panic did not crash the test process
}
