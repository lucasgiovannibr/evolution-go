package send_service

import (
	"encoding/json"
	"testing"
)

func decodeParams(t *testing.T, raw string) map[string]string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("buttonParamsJSON is not valid JSON: %v (%q)", err, raw)
	}
	return m
}

func TestBuildCarouselButton(t *testing.T) {
	cases := []struct {
		name     string
		btn      CarouselButtonStruct
		wantName string
		wantKey  string
		wantVal  string
	}{
		{"reply default", CarouselButtonStruct{DisplayText: "Info", Id: "card1"}, "quick_reply", "id", "card1"},
		{"url via id", CarouselButtonStruct{Type: "url", DisplayText: "Site", Id: "https://a.b"}, "cta_url", "url", "https://a.b"},
		{"url explicit field wins", CarouselButtonStruct{Type: "URL", DisplayText: "Site", URL: "https://x.y", Id: "ignored"}, "cta_url", "url", "https://x.y"},
		{"call via id", CarouselButtonStruct{Type: "CALL", DisplayText: "Ligar", Id: "5511999999999"}, "cta_call", "phone_number", "5511999999999"},
		{"call explicit field", CarouselButtonStruct{Type: "CALL", DisplayText: "Ligar", PhoneNumber: "5511888888888"}, "cta_call", "phone_number", "5511888888888"},
		{"copy", CarouselButtonStruct{Type: "COPY", DisplayText: "Copiar", CopyCode: "PROMO"}, "cta_copy", "copy_code", "PROMO"},
		{"COPY_CODE alias (issue #51)", CarouselButtonStruct{Type: "COPY_CODE", DisplayText: "Copiar", CopyCode: "PROMO"}, "cta_copy", "copy_code", "PROMO"},
	}
	for _, c := range cases {
		name, raw := buildCarouselButton(c.btn)
		if name != c.wantName {
			t.Errorf("%s: name = %q, want %q", c.name, name, c.wantName)
		}
		if got := decodeParams(t, raw)[c.wantKey]; got != c.wantVal {
			t.Errorf("%s: %s = %q, want %q", c.name, c.wantKey, got, c.wantVal)
		}
	}
}

// A quote or backslash in a label used to break the hand-built JSON.
func TestBuildCarouselButtonEscapesSpecialCharacters(t *testing.T) {
	_, raw := buildCarouselButton(CarouselButtonStruct{Type: "URL", DisplayText: `Diga "oi" \ ok`, URL: `https://a.b/?q="x"`})
	m := decodeParams(t, raw)
	if m["display_text"] != `Diga "oi" \ ok` || m["url"] != `https://a.b/?q="x"` {
		t.Fatalf("values were not preserved: %#v", m)
	}
}
