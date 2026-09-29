package message_service

import (
	"encoding/json"
	"testing"
)

func TestSubscribeNumbersAcceptsOneOrAList(t *testing.T) {
	var one SubscribePresenceStruct
	if err := json.Unmarshal([]byte(`{"number":"5511999990001"}`), &one); err != nil {
		t.Fatal(err)
	}
	if one.Number.IsList || len(one.Number.Numbers) != 1 || one.Number.Numbers[0] != "5511999990001" {
		t.Fatalf("single: %+v", one.Number)
	}

	var many SubscribePresenceStruct
	if err := json.Unmarshal([]byte(`{"number":["5511999990001","5511999990002"]}`), &many); err != nil {
		t.Fatal(err)
	}
	if !many.Number.IsList || len(many.Number.Numbers) != 2 {
		t.Fatalf("list: %+v", many.Number)
	}

	// a one-item array is still a list: the caller asked for per-number results
	var oneList SubscribePresenceStruct
	if err := json.Unmarshal([]byte(`{"number":["5511999990001"]}`), &oneList); err != nil || !oneList.Number.IsList {
		t.Fatalf("one-item list: %+v %v", oneList.Number, err)
	}

	for _, body := range []string{`{}`, `{"number":""}`, `{"number":null}`, `{"number":[]}`} {
		var s SubscribePresenceStruct
		if err := json.Unmarshal([]byte(body), &s); err != nil || len(s.Number.Numbers) != 0 {
			t.Errorf("%s: %+v %v", body, s.Number, err)
		}
	}

	var bad SubscribePresenceStruct
	if err := json.Unmarshal([]byte(`{"number":5}`), &bad); err == nil {
		t.Error("a number that is neither a string nor a list must be rejected")
	}
}

func TestSubscribeNumbersRoundTrip(t *testing.T) {
	for _, body := range []string{`{"number":"5511999990001"}`, `{"number":["5511999990001","5511999990002"]}`} {
		var s SubscribePresenceStruct
		if err := json.Unmarshal([]byte(body), &s); err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(s)
		if err != nil || string(out) != body {
			t.Errorf("%s -> %s (%v)", body, out, err)
		}
	}
}

func TestSubscribePresenceValidatesBeforeTouchingTheClient(t *testing.T) {
	m := &messageService{} // no client: reaching it would panic
	if _, err := m.SubscribePresence(&SubscribePresenceStruct{}, nil); err == nil || !IsRequestError(err) {
		t.Fatalf("empty must be a request error, got %v", err)
	}

	many := make([]string, maxSubscribeNumbers+1)
	for i := range many {
		many[i] = "5511999990001"
	}
	if _, err := m.SubscribePresence(&SubscribePresenceStruct{Number: SubscribeNumbers{Numbers: many, IsList: true}}, nil); err == nil || !IsRequestError(err) {
		t.Fatalf("too many must be a request error, got %v", err)
	}
}
