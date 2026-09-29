package logger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/evolution-foundation/evolution-go/pkg/config"
)

func TestReleaseFreesTheInstanceLogger(t *testing.T) {
	dir := t.TempDir()
	lm := NewLoggerManager(&config.Config{LogDirectory: dir, LogMaxSize: 1, LogMaxBackups: 1, LogMaxAge: 1})

	lm.GetLogger("inst-1").LogInfo("first line")
	logFile := filepath.Join(dir, "inst-1", "instance.log")
	before, err := os.ReadFile(logFile)
	if err != nil || len(before) == 0 {
		t.Fatalf("the instance log file should exist and have content: %v", err)
	}

	lm.Release("inst-1")

	lm.mu.RLock()
	_, stillCached := lm.loggers["inst-1"]
	lm.mu.RUnlock()
	if stillCached {
		t.Fatal("a released instance must not stay in the logger cache")
	}

	// Logging after the release must not resurrect a file logger nor touch the file.
	lm.GetLogger("inst-1").LogInfo("after release")
	after, _ := os.ReadFile(logFile)
	if string(after) != string(before) {
		t.Fatal("a released instance must not write to its log file any more")
	}
	lm.mu.RLock()
	_, recreated := lm.loggers["inst-1"]
	lm.mu.RUnlock()
	if recreated {
		t.Fatal("GetLogger must not recreate the logger of a released instance")
	}

	// Other instances are unaffected.
	lm.GetLogger("inst-2").LogInfo("still works")
	if _, err := os.Stat(filepath.Join(dir, "inst-2", "instance.log")); err != nil {
		t.Fatalf("other instances must keep their file logger: %v", err)
	}
}
