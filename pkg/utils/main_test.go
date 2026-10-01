package utils

import (
	"os"
	"testing"
)

// The existing tests serve files from httptest servers on 127.0.0.1, which the outbound
// policy refuses by default (SSRF protection). The tests of the policy itself turn it
// back on (see ssrf_test.go).
func TestMain(m *testing.M) {
	SetAllowPrivateURLs(true)
	os.Exit(m.Run())
}
