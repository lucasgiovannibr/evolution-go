package auth_middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sort"
)

// bodyEditor reads a few top-level fields of a JSON object body and rewrites them in
// place, without parsing or re-encoding the rest.
//
// The JID middleware used to parse every body into map[string]interface{} and, when it
// normalized a number, marshal the whole map back. For a /send/media body carrying the
// file as base64 that meant several full copies of the file (parse, copy of each string,
// marshal) just to touch one short field; with a 100 MB body that was ~350 MB of heap.
// Now the body is validated once (json.Valid does not allocate), the byte range of each
// wanted field is located with a small scanner, and the rewritten body is served as the
// original bytes around the new field value, without copying them.
type bodyEditor struct {
	body  []byte
	spans map[string]span   // top-level key -> byte range of its value (the last one wins, as in a map)
	edits map[string][]byte // key -> replacement for its value
}

type span struct{ start, end int }

var errNotAnObject = errors.New("body is not a JSON object")

// newBodyEditor validates body as JSON and locates the values of keys in the top-level
// object. A body that is not valid JSON, or not an object, is an error.
func newBodyEditor(body []byte, keys ...string) (*bodyEditor, error) {
	if !json.Valid(body) {
		return nil, errors.New("invalid JSON")
	}
	wanted := make(map[string]bool, len(keys))
	for _, k := range keys {
		wanted[k] = true
	}

	e := &bodyEditor{body: body, spans: map[string]span{}, edits: map[string][]byte{}}
	i := skipSpace(body, 0)
	if i >= len(body) || body[i] != '{' {
		return nil, errNotAnObject
	}
	i++

	// The body is valid JSON, so the structure below can be trusted.
	for {
		i = skipSpace(body, i)
		if body[i] == '}' {
			return e, nil
		}
		keyStart := i
		i = skipString(body, i)
		key := string(body[keyStart+1 : i-1])
		if bytes.IndexByte(body[keyStart+1:i-1], '\\') >= 0 {
			var decoded string
			if err := json.Unmarshal(body[keyStart:i], &decoded); err == nil {
				key = decoded
			}
		}

		i = skipSpace(body, i)
		i++ // ':'
		i = skipSpace(body, i)
		valueStart := i
		i = skipValue(body, i)
		if wanted[key] {
			e.spans[key] = span{valueStart, i}
		}

		i = skipSpace(body, i)
		if body[i] == ',' {
			i++
		}
	}
}

func skipSpace(b []byte, i int) int {
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	return i
}

// skipString returns the index after the string that starts at b[i] (a quote).
func skipString(b []byte, i int) int {
	i++
	for b[i] != '"' {
		if b[i] == '\\' {
			i++
		}
		i++
	}
	return i + 1
}

// skipValue returns the index after the JSON value that starts at b[i].
func skipValue(b []byte, i int) int {
	switch b[i] {
	case '"':
		return skipString(b, i)
	case '{', '[':
		depth := 0
		for {
			switch b[i] {
			case '"':
				i = skipString(b, i)
				continue
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return i + 1
				}
			}
			i++
		}
	default: // number, true, false, null
		for i < len(b) && b[i] != ',' && b[i] != '}' && b[i] != ']' && b[i] != ' ' && b[i] != '\t' && b[i] != '\n' && b[i] != '\r' {
			i++
		}
		return i
	}
}

// raw returns the JSON text of a top-level field.
func (e *bodyEditor) raw(key string) (json.RawMessage, bool) {
	s, ok := e.spans[key]
	if !ok {
		return nil, false
	}
	return json.RawMessage(e.body[s.start:s.end]), true
}

// set replaces the value of a top-level field that is present.
func (e *bodyEditor) set(key string, value interface{}) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	e.edits[key] = encoded
	return nil
}

func (e *bodyEditor) modified() bool { return len(e.edits) > 0 }

// reader serves the body with the edits applied. Untouched bytes are not copied.
func (e *bodyEditor) reader() io.Reader {
	if !e.modified() {
		return bytes.NewReader(e.body)
	}
	type edit struct {
		span
		value []byte
	}
	edits := make([]edit, 0, len(e.edits))
	for key, value := range e.edits {
		edits = append(edits, edit{e.spans[key], value})
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })

	var parts []io.Reader
	at := 0
	for _, ed := range edits {
		parts = append(parts, bytes.NewReader(e.body[at:ed.start]), bytes.NewReader(ed.value))
		at = ed.end
	}
	parts = append(parts, bytes.NewReader(e.body[at:]))
	return io.MultiReader(parts...)
}

// size is the length of the body with the edits applied.
func (e *bodyEditor) size() int64 {
	n := int64(len(e.body))
	for key, value := range e.edits {
		s := e.spans[key]
		n += int64(len(value)) - int64(s.end-s.start)
	}
	return n
}
