package config

import "testing"

func TestAddInstanceToken(t *testing.T) {
	on := &Config{WebhookIncludeToken: true}
	payload := map[string]interface{}{"event": "Message"}
	on.AddInstanceToken(payload, "tok")
	if payload["instanceToken"] != "tok" {
		t.Fatalf("WEBHOOK_INCLUDE_TOKEN=true must keep the token in the payload, got %v", payload)
	}

	off := &Config{WebhookIncludeToken: false}
	payload = map[string]interface{}{"event": "Message"}
	off.AddInstanceToken(payload, "tok")
	if _, found := payload["instanceToken"]; found {
		t.Fatalf("WEBHOOK_INCLUDE_TOKEN=false must not put the token in the payload, got %v", payload)
	}

	// Tests and tools that build a service without a Config keep the historical payload.
	var none *Config
	payload = map[string]interface{}{}
	none.AddInstanceToken(payload, "tok")
	if payload["instanceToken"] != "tok" {
		t.Fatalf("a nil Config must keep the token, got %v", payload)
	}
}
