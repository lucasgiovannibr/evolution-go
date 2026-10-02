package config

import "testing"

func TestCallMediaStallFromEnv(t *testing.T) {
	for raw, want := range map[string]int{
		"":    0,  // unset: the engine default
		"abc": 0,  // not a number: the engine default
		"30":  30, // seconds
		" 7 ": 7,
		"0":   -1, // off
		"-5":  -1, // off
	} {
		t.Setenv("CALL_MEDIA_STALL", raw)
		if got := callMediaStall(); got != want {
			t.Errorf("CALL_MEDIA_STALL=%q gave %d, want %d", raw, got, want)
		}
	}
}
