package httpx

import (
	"bytes"
	"encoding/json"
	"testing"
)

// dataKey reports how `data` appears on the wire for an envelope: absent, explicit null, or a
// present value. Marshalling the envelope directly isolates MarshalJSON from the HTTP style.
func dataState(t *testing.T, env Envelope) (raw string, present bool, value any) {
	t.Helper()
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rawData, ok := m["data"]
	if !ok {
		return string(b), false, nil
	}
	var v any
	if err := json.Unmarshal(rawData, &v); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	return string(b), true, v
}

// An unset Data omits the key - the shape every error envelope sends, and any route that
// answers no data.
func TestEnvelope_DataUnsetOmitsKey(t *testing.T) {
	raw, present, _ := dataState(t, Envelope{Code: 401, Message: "Unauthorized.", Status: False()})
	if present {
		t.Fatalf("data key should be absent, got %s", raw)
	}
	if bytes.Contains([]byte(raw), []byte("data")) {
		t.Fatalf("data must not appear at all: %s", raw)
	}
}

// A non-nil empty slice sends `data: []`, NOT an omitted key. This is the bug omitempty caused:
// /sheet/open-jobs builds make([]T,0,n) on purpose, and Node answers [] for an empty company.
func TestEnvelope_EmptySliceIsArray(t *testing.T) {
	raw, present, value := dataState(t, Envelope{Code: 200, Message: "Operation successful.", Data: []int{}})
	if !present {
		t.Fatalf("data key should be present as [], got %s", raw)
	}
	arr, ok := value.([]any)
	if !ok || len(arr) != 0 {
		t.Fatalf("data should be an empty array, got %#v (%s)", value, raw)
	}
	if !bytes.Contains([]byte(raw), []byte(`"data":[]`)) {
		t.Fatalf(`expected "data":[] in %s`, raw)
	}
}

// The Null sentinel sends `data: null`, distinct from an omitted key.
func TestEnvelope_NullSentinelIsNull(t *testing.T) {
	raw, present, value := dataState(t, Envelope{Code: 200, Message: "Operation successful.", Status: True(), Data: Null})
	if !present {
		t.Fatalf("data key should be present as null, got %s", raw)
	}
	if value != nil {
		t.Fatalf("data should be null, got %#v (%s)", value, raw)
	}
	if !bytes.Contains([]byte(raw), []byte(`"data":null`)) {
		t.Fatalf(`expected "data":null in %s`, raw)
	}
}

// An ordinary value is marshalled as-is and the other fields keep their shape (status omitted
// when nil, message present).
func TestEnvelope_ValuePreserved(t *testing.T) {
	raw, present, value := dataState(t, Envelope{Code: 200, Message: "Operation successful.", Data: map[string]any{"a": 1}})
	if !present {
		t.Fatalf("data key should be present, got %s", raw)
	}
	obj, ok := value.(map[string]any)
	if !ok || obj["a"] != float64(1) {
		t.Fatalf("data object wrong: %#v (%s)", value, raw)
	}
	if bytes.Contains([]byte(raw), []byte("status")) {
		t.Fatalf("status must be omitted when nil: %s", raw)
	}
}

// A populated slice is unchanged by the new marshaller - the common case must not regress.
func TestEnvelope_NonEmptySlice(t *testing.T) {
	raw, present, value := dataState(t, Envelope{Code: 200, Data: []int{1, 2, 3}})
	if !present {
		t.Fatalf("data key should be present, got %s", raw)
	}
	arr, ok := value.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("data should be a 3-element array, got %#v (%s)", value, raw)
	}
}
