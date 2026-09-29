package whatsmeow_service

import "testing"

func TestParseProxyConfigTreatsNoProxyAsEmpty(t *testing.T) {
	for _, raw := range []string{"", "  ", "null"} {
		cfg, err := parseProxyConfig(raw)
		if err != nil || cfg != (ProxyConfig{}) {
			t.Errorf("parseProxyConfig(%q) = %+v, %v; want empty and no error", raw, cfg, err)
		}
	}
	if _, err := parseProxyConfig("{not json"); err == nil {
		t.Error("malformed JSON must still be an error")
	}
}
