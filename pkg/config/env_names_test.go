package config

import "testing"

// The documented names (WADEBUG, LOGTYPE) had no effect: the code read DEBUG_ENABLED and
// LOG_TYPE. Both work now, the documented ones first.
func TestGetenvAny(t *testing.T) {
	t.Setenv("TEST_NEW_NAME", "")
	t.Setenv("TEST_OLD_NAME", "")
	if got := getenvAny("TEST_NEW_NAME", "TEST_OLD_NAME"); got != "" {
		t.Fatalf("nothing set, got %q", got)
	}

	t.Setenv("TEST_OLD_NAME", "legacy")
	if got := getenvAny("TEST_NEW_NAME", "TEST_OLD_NAME"); got != "legacy" {
		t.Fatalf("the fallback must be used when the documented name is empty, got %q", got)
	}

	t.Setenv("TEST_NEW_NAME", "documented")
	if got := getenvAny("TEST_NEW_NAME", "TEST_OLD_NAME"); got != "documented" {
		t.Fatalf("the documented name wins over the fallback, got %q", got)
	}
}
