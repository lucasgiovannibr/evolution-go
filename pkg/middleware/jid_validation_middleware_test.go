package auth_middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// These tests pin the behaviour of the JID validation middleware: they were written
// against the implementation that parsed every body into a map and re-encoded it, and
// must keep passing for the one that edits the body in place.

type seen struct {
	code int
	body map[string]interface{} // what the handler received, parsed
	raw  string                 // ... and as bytes
	hit  bool
}

func runMiddleware(t *testing.T, mw gin.HandlerFunc, contentType, body string) seen {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var out seen
	r.POST("/x", mw, func(c *gin.Context) {
		out.hit = true
		b, _ := io.ReadAll(c.Request.Body)
		out.raw = string(b)
		_ = json.Unmarshal(b, &out.body)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out.code = w.Code
	return out
}

func runJSON(t *testing.T, mw gin.HandlerFunc, body string) seen {
	return runMiddleware(t, mw, "application/json", body)
}

func mustEqual(t *testing.T, name string, got, want interface{}) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %#v, want %#v", name, got, want)
	}
}

const brNumber = "+5511999999999@s.whatsapp.net"

func TestNumberFieldWithFormatJid(t *testing.T) {
	mw := NewJIDValidationMiddleware().ValidateNumberFieldWithFormatJid()

	// a plain number is normalized to a JID, the other fields untouched
	s := runJSON(t, mw, `{"number":"5511999999999","text":"hello","delay":3,"nested":{"a":[1,2,{"b":"c"}]}}`)
	mustEqual(t, "code", s.code, 200)
	mustEqual(t, "number", s.body["number"], brNumber)
	mustEqual(t, "text", s.body["text"], "hello")
	mustEqual(t, "delay", s.body["delay"], float64(3))
	mustEqual(t, "nested", s.body["nested"], map[string]interface{}{"a": []interface{}{float64(1), float64(2), map[string]interface{}{"b": "c"}}})

	// the Brazilian ninth-digit rule applies for DDD >= 31
	s = runJSON(t, mw, `{"number":"5531999999999"}`)
	mustEqual(t, "ddd 31", s.body["number"], "+553199999999@s.whatsapp.net")

	// an already normalized JID is left as it came (the bytes too)
	in := `{"number":"5511999999999@s.whatsapp.net","text":"x"}`
	s = runJSON(t, mw, in)
	mustEqual(t, "jid code", s.code, 200)
	mustEqual(t, "jid raw", s.raw, in)

	// formatJid=false keeps the number as received
	s = runJSON(t, mw, `{"number":"abc","formatJid":false}`)
	mustEqual(t, "formatJid false code", s.code, 200)
	mustEqual(t, "formatJid false number", s.body["number"], "abc")

	// a non-boolean formatJid is ignored (formatting stays on)
	s = runJSON(t, mw, `{"number":"5511999999999","formatJid":"no"}`)
	mustEqual(t, "formatJid string", s.body["number"], brNumber)

	// arrays
	s = runJSON(t, mw, `{"number":["5511999999999","120363025246125486@g.us"]}`)
	mustEqual(t, "array code", s.code, 200)
	mustEqual(t, "array", s.body["number"], []interface{}{brNumber, "120363025246125486@g.us"})

	// a missing number passes through untouched
	in = `{"text":"no number here"}`
	s = runJSON(t, mw, in)
	mustEqual(t, "missing code", s.code, 200)
	mustEqual(t, "missing raw", s.raw, in)

	// rejections
	for name, body := range map[string]string{
		"empty string":    `{"number":""}`,
		"empty array":     `{"number":[]}`,
		"empty item":      `{"number":["5511999999999",""]}`,
		"wrong type":      `{"number":123}`,
		"invalid number":  `{"number":"abc"}`,
		"invalid in list": `{"number":["5511999999999","abc"]}`,
		"invalid json":    `{"number":`,
		"not an object":   `["number"]`,
		"empty body":      ``,
	} {
		s := runJSON(t, mw, body)
		if s.code != 400 || s.hit {
			t.Errorf("%s: code %d (handler hit: %v), want 400 before the handler", name, s.code, s.hit)
		}
	}

	// not JSON: untouched
	s = runMiddleware(t, mw, "text/plain", "whatever")
	mustEqual(t, "non json code", s.code, 200)
	mustEqual(t, "non json raw", s.raw, "whatever")
}

func TestNumberFieldWithFormatJidKeepsEscapesAndUnicode(t *testing.T) {
	mw := NewJIDValidationMiddleware().ValidateNumberFieldWithFormatJid()

	// the key is found even when other strings contain braces, quotes, escaped quotes and
	// the very text of the number
	body := `{"text":"he said \"number\":\"5511999999999\" {","emoji":"😀","uni":"\u00e9","number":"5511999999999","tail":[ "}" , "\\" ]}`
	s := runJSON(t, mw, body)
	mustEqual(t, "code", s.code, 200)
	mustEqual(t, "number", s.body["number"], brNumber)
	mustEqual(t, "text", s.body["text"], `he said "number":"5511999999999" {`)
	mustEqual(t, "emoji", s.body["emoji"], "😀")
	mustEqual(t, "uni", s.body["uni"], "é")
	mustEqual(t, "tail", s.body["tail"], []interface{}{"}", `\`})

	// whitespace and field order
	s = runJSON(t, mw, "{\n  \"text\" : \"x\" ,\n  \"number\"\t:\t\"5511999999999\"\n}")
	mustEqual(t, "ws number", s.body["number"], brNumber)

	// the last duplicate wins, as with a map
	s = runJSON(t, mw, `{"number":"abc","number":"5511999999999"}`)
	mustEqual(t, "duplicate code", s.code, 200)
	mustEqual(t, "duplicate", s.body["number"], brNumber)
}

func TestNumberFieldWithFormatJidLeavesAHugeFieldAlone(t *testing.T) {
	mw := NewJIDValidationMiddleware().ValidateNumberFieldWithFormatJid()
	huge := strings.Repeat("QUJD", 1<<20) // 4 MB of base64
	s := runJSON(t, mw, `{"number":"5511999999999","url":"`+huge+`"}`)
	mustEqual(t, "code", s.code, 200)
	mustEqual(t, "number", s.body["number"], brNumber)
	if s.body["url"] != huge {
		t.Fatal("the huge field must reach the handler intact")
	}
}

func TestNumberField(t *testing.T) {
	mw := NewJIDValidationMiddleware().ValidateNumberField()

	s := runJSON(t, mw, `{"number":"5511999999999","reaction":"x"}`)
	mustEqual(t, "code", s.code, 200)
	mustEqual(t, "number", s.body["number"], brNumber)

	// unlike the WithFormatJid variant, formatJid does not turn it off
	s = runJSON(t, mw, `{"number":"5511999999999","formatJid":false}`)
	mustEqual(t, "ignores formatJid", s.body["number"], brNumber)

	s = runJSON(t, mw, `{"number":["5511999999999"]}`)
	mustEqual(t, "array", s.body["number"], []interface{}{brNumber})

	for name, body := range map[string]string{
		"empty": `{"number":""}`, "empty array": `{"number":[]}`, "bad": `{"number":"abc"}`,
		"wrong type": `{"number":true}`, "invalid json": `{`, "not object": `1`,
	} {
		if s := runJSON(t, mw, body); s.code != 400 || s.hit {
			t.Errorf("%s: code %d hit %v", name, s.code, s.hit)
		}
	}

	// multipart is validated from the form fields (not touched here)
	s = runMiddleware(t, mw, "multipart/form-data; boundary=x", "--x--")
	mustEqual(t, "multipart without number", s.code, 400)
}

func TestJIDFields(t *testing.T) {
	mw := NewJIDValidationMiddleware().ValidateJIDFields("number", "communityId")

	s := runJSON(t, mw, `{"number":"5511999999999","communityId":"120363025246125486@g.us"}`)
	mustEqual(t, "code", s.code, 200)
	mustEqual(t, "number", s.body["number"], brNumber)
	mustEqual(t, "communityId", s.body["communityId"], "120363025246125486@g.us")

	// a field that is absent is fine
	if s := runJSON(t, mw, `{"number":"5511999999999"}`); s.code != 200 {
		t.Errorf("absent field: code %d", s.code)
	}

	for name, body := range map[string]string{
		"empty":        `{"number":""}`,
		"invalid":      `{"number":"abc"}`,
		"non-string":   `{"number":5}`, // reported as empty, as before
		"invalid json": `nope`,
	} {
		if s := runJSON(t, mw, body); s.code != 400 || s.hit {
			t.Errorf("%s: code %d hit %v", name, s.code, s.hit)
		}
	}
}

func TestMultipleNumbers(t *testing.T) {
	mw := NewJIDValidationMiddleware().ValidateMultipleNumbers("participants")

	s := runJSON(t, mw, `{"participants":["5511999999999","120363@lid"],"name":"g"}`)
	mustEqual(t, "code", s.code, 200)
	mustEqual(t, "participants", s.body["participants"], []interface{}{brNumber, "120363@lid"})
	mustEqual(t, "name", s.body["name"], "g")

	// items that are not strings, and empty strings, are left alone
	s = runJSON(t, mw, `{"participants":["5511999999999",7,"",null]}`)
	mustEqual(t, "code", s.code, 200)
	mustEqual(t, "mixed", s.body["participants"], []interface{}{brNumber, float64(7), "", nil})

	// not an array: ignored
	in := `{"participants":"5511999999999"}`
	s = runJSON(t, mw, in)
	mustEqual(t, "not array code", s.code, 200)
	mustEqual(t, "not array raw", s.raw, in)

	for name, body := range map[string]string{
		"invalid item": `{"participants":["abc"]}`, "invalid json": `{`, "not object": `[]`,
	} {
		if s := runJSON(t, mw, body); s.code != 400 || s.hit {
			t.Errorf("%s: code %d hit %v", name, s.code, s.hit)
		}
	}

	// non-JSON passes
	if s := runMiddleware(t, mw, "text/plain", "x"); s.code != 200 {
		t.Errorf("non json: code %d", s.code)
	}
}

func TestContactFields(t *testing.T) {
	mw := NewJIDValidationMiddleware().ValidateContactFields()

	s := runJSON(t, mw, `{"number":"5511999999999","vcard":{"fullName":"A","phone":"5511888888888"}}`)
	mustEqual(t, "code", s.code, 200)
	mustEqual(t, "number", s.body["number"], brNumber)
	// the vcard phone is validated, not rewritten
	mustEqual(t, "vcard", s.body["vcard"], map[string]interface{}{"fullName": "A", "phone": "5511888888888"})

	// no number: only the vcard is checked
	if s := runJSON(t, mw, `{"vcard":{"phone":"5511888888888"}}`); s.code != 200 {
		t.Errorf("vcard only: code %d", s.code)
	}

	for name, body := range map[string]string{
		"bad number":   `{"number":"abc"}`,
		"bad phone":    `{"number":"5511999999999","vcard":{"phone":"abc"}}`,
		"invalid json": `{"vcard":`,
	} {
		if s := runJSON(t, mw, body); s.code != 400 || s.hit {
			t.Errorf("%s: code %d hit %v", name, s.code, s.hit)
		}
	}

	// a non-string number, or a vcard that is not an object, is not this middleware's business
	if s := runJSON(t, mw, `{"number":5,"vcard":"x"}`); s.code != 200 {
		t.Errorf("lenient cases: code %d", s.code)
	}
}
