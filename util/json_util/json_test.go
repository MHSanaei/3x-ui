package json_util

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRawMessage_MarshalEmptyIsNull(t *testing.T) {
	var m RawMessage
	out, err := m.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON on empty returned error: %v", err)
	}
	if !bytes.Equal(out, []byte("null")) {
		t.Fatalf("empty RawMessage marshaled to %q, want %q", out, "null")
	}
}

func TestRawMessage_MarshalPassthrough(t *testing.T) {
	payload := []byte(`{"a":1}`)
	m := RawMessage(payload)
	out, err := m.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON returned error: %v", err)
	}
	if !bytes.Equal(out, payload) {
		t.Fatalf("MarshalJSON = %q, want %q", out, payload)
	}
}

func TestRawMessage_UnmarshalCopiesData(t *testing.T) {
	var m RawMessage
	src := []byte(`{"k":"v"}`)
	if err := m.UnmarshalJSON(src); err != nil {
		t.Fatalf("UnmarshalJSON returned error: %v", err)
	}
	if !bytes.Equal(m, src) {
		t.Fatalf("UnmarshalJSON stored %q, want %q", []byte(m), src)
	}

	src[0] = 'X'
	if m[0] == 'X' {
		t.Fatal("UnmarshalJSON kept a reference to the caller's buffer; expected a copy")
	}
}

func TestRawMessage_UnmarshalNilReceiverErrors(t *testing.T) {
	var m *RawMessage
	if err := m.UnmarshalJSON([]byte("123")); err == nil {
		t.Fatal("expected error for nil receiver")
	}
}

func TestRawMessage_RoundTripInsideStruct(t *testing.T) {
	type wrapper struct {
		Body RawMessage `json:"body"`
	}
	in := wrapper{Body: RawMessage(`{"x":42}`)}
	encoded, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	want := `{"body":{"x":42}}`
	if string(encoded) != want {
		t.Fatalf("Marshal = %s, want %s", encoded, want)
	}

	var out wrapper
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if string(out.Body) != `{"x":42}` {
		t.Fatalf("round-trip Body = %s, want %s", out.Body, `{"x":42}`)
	}
}
