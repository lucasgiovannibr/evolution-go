package logger

import "github.com/evolution-foundation/evolution-go/pkg/config"

// testingT is the part of *testing.T the helper needs (so this package does not import
// "testing").
type testingT interface {
	Helper()
	Cleanup(func())
}

// NewLoggerManagerForTest is NewLoggerManager for tests that log into t.TempDir(): lines are
// written by a goroutine, so without closing the manager a write can still be running when
// the test ends and make the removal of the directory fail ("directory not empty").
func NewLoggerManagerForTest(t testingT, cfg *config.Config) *LoggerManager {
	t.Helper()
	lm := NewLoggerManager(cfg)
	t.Cleanup(lm.Close)
	return lm
}
