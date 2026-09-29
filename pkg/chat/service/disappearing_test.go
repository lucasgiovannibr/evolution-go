package chat_service

import (
	"errors"
	"testing"
	"time"
)

func TestParseDisappearingTimer(t *testing.T) {
	ok := map[string]time.Duration{
		"off": 0, "0": 0,
		"24h": 24 * time.Hour, "1d": 24 * time.Hour, "86400": 24 * time.Hour,
		"7d": 7 * 24 * time.Hour, "604800": 7 * 24 * time.Hour,
		"90d": 90 * 24 * time.Hour, "7776000": 90 * 24 * time.Hour,
	}
	for in, want := range ok {
		got, err := parseDisappearingTimer(in)
		if err != nil || got != want {
			t.Errorf("parseDisappearingTimer(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "5m", "30d", "abc"} {
		_, err := parseDisappearingTimer(bad)
		if err == nil || !IsDisappearingRequestError(err) {
			t.Errorf("parseDisappearingTimer(%q) must be a request error, got %v", bad, err)
		}
	}
}

func TestSetDisappearingValidatesBeforeTouchingTheClient(t *testing.T) {
	c := &chatService{} // no client: reaching it would panic
	for name, d := range map[string]*DisappearingStruct{
		"no timer":  {Chat: "5511999990001"},
		"bad chat":  {Chat: "@@", Timer: "24h"},
		"newsletter": {Chat: "123@newsletter", Timer: "24h"},
	} {
		err := c.SetDisappearing(d, nil)
		if err == nil || !IsDisappearingRequestError(err) {
			t.Errorf("%s: want a request error, got %v", name, err)
		}
	}
	if IsDisappearingRequestError(errors.New("boom")) {
		t.Error("a foreign error is not a request error")
	}
}
