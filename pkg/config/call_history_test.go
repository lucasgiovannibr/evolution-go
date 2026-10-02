package config

import "testing"

func TestCallHistoryRetentionDaysFromEnv(t *testing.T) {
	for raw, want := range map[string]int{
		"":    90, // unset: the default
		"abc": 90, // not a number: the default
		"-5":  90, // not allowed: the default
		"30":  30,
		" 7 ": 7,
		"0":   0, // keep for ever
	} {
		t.Setenv("CALL_HISTORY_RETENTION_DAYS", raw)
		if got := callHistoryRetentionDays(); got != want {
			t.Errorf("CALL_HISTORY_RETENTION_DAYS=%q gave %d, want %d", raw, got, want)
		}
	}
}
