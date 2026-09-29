package instance_service

import "testing"

func TestParseProxyConfigTreatsNoProxyAsEmpty(t *testing.T) {
	for _, raw := range []string{"", "  ", "null"} {
		cfg, err := parseProxyConfig(raw)
		if err != nil || cfg != (ProxyConfig{}) {
			t.Errorf("parseProxyConfig(%q) = %+v, %v; want empty and no error", raw, cfg, err)
		}
	}
}

func TestParseProxyConfigReadsAProxy(t *testing.T) {
	cfg, err := parseProxyConfig(`{"host":"proxy.example","port":"1080","username":"u","password":"p","protocol":"socks5"}`)
	if err != nil || cfg.Host != "proxy.example" || cfg.Port != "1080" || cfg.Protocol != "socks5" {
		t.Fatalf("%+v %v", cfg, err)
	}
}

func TestParseProxyConfigRejectsGarbage(t *testing.T) {
	if _, err := parseProxyConfig("{not json"); err == nil {
		t.Fatal("malformed JSON must still be an error")
	}
}
