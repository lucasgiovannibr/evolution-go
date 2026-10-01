package instance_model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInstanceJSONHidesProxyPassword(t *testing.T) {
	i := Instance{Id: "x", Proxy: `{"protocol":"socks5","host":"h","port":"1080","username":"u","password":"s3cret"}`}

	out, err := json.Marshal(i)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "s3cret") || strings.Contains(string(out), "password") {
		t.Fatalf("proxy password leaked: %s", out)
	}
	var back struct{ Proxy string }
	_ = json.Unmarshal(out, &back)
	if !strings.Contains(back.Proxy, `"host":"h"`) || !strings.Contains(back.Proxy, `"username":"u"`) {
		t.Fatalf("host/user must stay visible: %s", back.Proxy)
	}
	if i.Proxy == "" || !strings.Contains(i.Proxy, "s3cret") {
		t.Fatal("the stored value must not be modified")
	}
}

func TestInstanceJSONProxyEdgeCases(t *testing.T) {
	for _, raw := range []string{"", "null"} {
		out, _ := json.Marshal(Instance{Proxy: raw})
		if !strings.Contains(string(out), `"proxy":"`+raw+`"`) {
			t.Errorf("proxy %q changed: %s", raw, out)
		}
	}
	out, _ := json.Marshal(Instance{Proxy: "not json s3cret"})
	if strings.Contains(string(out), "s3cret") {
		t.Fatalf("unparseable proxy echoed: %s", out)
	}
}
