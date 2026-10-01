package auth_middleware

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// randomValue builds JSON values with the things that trip a hand-written scanner:
// escaped quotes and backslashes, braces inside strings, unicode, nesting, whitespace.
func randomValue(r *rand.Rand, depth int) interface{} {
	strs := []string{"", "plain", `quo"te`, `back\slash`, `}{][`, `"number":"1"`, "é😀", "tab\tnew\nline", `\"`, `\\"`}
	switch n := r.Intn(8); {
	case n == 0:
		return nil
	case n == 1:
		return r.Intn(2) == 0
	case n == 2:
		return float64(r.Intn(1000)) / 7
	case n <= 4 || depth == 0:
		return strs[r.Intn(len(strs))]
	case n == 5:
		arr := make([]interface{}, r.Intn(4))
		for i := range arr {
			arr[i] = randomValue(r, depth-1)
		}
		return arr
	default:
		obj := map[string]interface{}{}
		for i := 0; i < r.Intn(4); i++ {
			obj[strs[r.Intn(len(strs))]] = randomValue(r, depth-1)
		}
		return obj
	}
}

func TestBodyEditorLocatesFieldsLikeAJSONParser(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for n := 0; n < 3000; n++ {
		obj := map[string]interface{}{"number": randomValue(r, 3), "other": randomValue(r, 3), "formatJid": randomValue(r, 2)}
		for i := 0; i < r.Intn(4); i++ {
			obj[`k"`+string(rune('a'+i))] = randomValue(r, 3)
		}
		var body []byte
		if r.Intn(2) == 0 {
			body, _ = json.Marshal(obj)
		} else {
			body, _ = json.MarshalIndent(obj, "  ", "\t")
		}

		e, err := newBodyEditor(body, "number", "formatJid")
		if err != nil {
			t.Fatalf("body %s: %v", body, err)
		}
		for _, key := range []string{"number", "formatJid"} {
			raw, ok := e.raw(key)
			if !ok {
				t.Fatalf("key %q not found in %s", key, body)
			}
			var got interface{}
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("raw of %q is not valid JSON (%q) in %s", key, raw, body)
			}
			if !reflect.DeepEqual(got, roundTrip(obj[key])) {
				t.Fatalf("key %q: got %#v, want %#v in %s", key, got, obj[key], body)
			}
		}
	}
}

func roundTrip(v interface{}) interface{} {
	b, _ := json.Marshal(v)
	var out interface{}
	_ = json.Unmarshal(b, &out)
	return out
}

func TestBodyEditorEditsOnlyTheChosenFields(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for n := 0; n < 1000; n++ {
		obj := map[string]interface{}{"number": randomValue(r, 3), "other": randomValue(r, 3), "z": randomValue(r, 2)}
		body, _ := json.MarshalIndent(obj, "", " ")

		e, err := newBodyEditor(body, "number")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.set("number", "EDITED"); err != nil {
			t.Fatal(err)
		}
		out := e.apply()
		var got map[string]interface{}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("edited body is not valid JSON: %v\n%s", err, out)
		}
		want := map[string]interface{}{"number": "EDITED", "other": roundTrip(obj["other"]), "z": roundTrip(obj["z"])}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v want %#v\nbody %s", got, want, body)
		}
	}
}

func TestBodyEditorRejectsWhatIsNotAnObject(t *testing.T) {
	for _, bad := range []string{``, `   `, `[]`, `"s"`, `12`, `null`, `{`, `{"a":`, `{"a":1,}`} {
		if _, err := newBodyEditor([]byte(bad), "a"); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
	if _, err := newBodyEditor([]byte(" \n{ } "), "a"); err != nil {
		t.Errorf("an empty object is fine: %v", err)
	}
}

func TestBodyEditorEscapedKeyIsMatched(t *testing.T) {
	e, err := newBodyEditor([]byte(`{"number":"5511"}`), "number")
	if err != nil {
		t.Fatal(err)
	}
	if raw, ok := e.raw("number"); !ok || string(raw) != `"5511"` {
		t.Fatalf("an escaped key must match, got %q %v", raw, ok)
	}
}

// The point of the rewrite: a body dominated by one big field is neither parsed into a
// map nor re-encoded.
func BenchmarkNormalizeNumberInAHugeBody(b *testing.B) {
	original := []byte(`{"number":"5511999999999","type":"image","url":"` + strings.Repeat("QUJD", 5<<20) + `"}`) // 20 MB
	b.ReportAllocs()
	b.SetBytes(int64(len(original)))
	for i := 0; i < b.N; i++ {
		// what readBody hands over: the body in a buffer with a little room, read once
		b.StopTimer()
		body := make([]byte, len(original), len(original)+8<<10)
		copy(body, original)
		b.StartTimer()

		e, err := newBodyEditor(body, "number", "formatJid")
		if err != nil {
			b.Fatal(err)
		}
		raw, _ := e.raw("number")
		value, changed, msg := normalizeNumber(raw, true)
		if msg != "" || !changed {
			b.Fatal("unexpected")
		}
		_ = e.set("number", value)
		_ = e.apply()
	}
}

// BenchmarkLegacyNormalizeInAHugeBody is the previous approach, for comparison.
func BenchmarkLegacyNormalizeInAHugeBody(b *testing.B) {
	body := []byte(`{"number":"5511999999999","type":"image","url":"` + strings.Repeat("QUJD", 5<<20) + `"}`)
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var m map[string]interface{}
		if err := json.Unmarshal(body, &m); err != nil {
			b.Fatal(err)
		}
		m["number"] = "+5511999999999@s.whatsapp.net"
		out, _ := json.Marshal(m)
		_ = out
	}
}

// The edits are made inside the buffer when it has room (no second copy of the body), and
// in a new buffer when it has not; both give the same bytes.
func TestApplyInPlaceAndGrownAgree(t *testing.T) {
	body := `{"a":"` + strings.Repeat("x", 1000) + `","number":"5511999999999","tail":[1,2,3]}`
	want := `{"a":"` + strings.Repeat("x", 1000) + `","number":"+5511999999999@s.whatsapp.net","tail":[1,2,3]}`

	for name, capacity := range map[string]int{"room": len(body) + 256, "no room": len(body)} {
		buf := make([]byte, len(body), capacity)
		copy(buf, body)
		e, err := newBodyEditor(buf, "number")
		if err != nil {
			t.Fatal(err)
		}
		_ = e.set("number", "+5511999999999@s.whatsapp.net")
		got := e.apply()
		if string(got) != want {
			t.Fatalf("%s: got %q", name, got)
		}
		if name == "room" && &got[0] != &buf[0] {
			t.Fatal("with room in the buffer the edit must not allocate a new one")
		}
	}
}

func TestApplyWithSeveralEditsAndAShorterValue(t *testing.T) {
	body := `{"number":"5511999999999","vcard":{"phone":"1"},"other":"keep","x":"5511999999999"}`
	buf := make([]byte, len(body), len(body)+512)
	copy(buf, body)
	e, _ := newBodyEditor(buf, "number", "x")
	_ = e.set("number", "N")                                // shorter
	_ = e.set("x", "a much longer replacement value for x") // longer
	var got map[string]interface{}
	if err := json.Unmarshal(e.apply(), &got); err != nil {
		t.Fatal(err)
	}
	if got["number"] != "N" || got["x"] != "a much longer replacement value for x" || got["other"] != "keep" {
		t.Fatalf("got %v", got)
	}
}
