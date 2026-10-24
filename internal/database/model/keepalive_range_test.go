package model

import (
	"encoding/json"
	"testing"
)

func TestClientKeepAliveJSONPreservesRangeAndScalarCompatibility(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  any
	}{
		{name: "range", input: `{"keepAlive":"15-25"}`, want: "15-25"},
		{name: "scalar", input: `{"keepAlive":25}`, want: float64(25)},
		{name: "explicit zero", input: `{"keepAlive":0}`, want: float64(0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client Client
			if err := json.Unmarshal([]byte(tt.input), &client); err != nil {
				t.Fatalf("decode keepAlive: %v", err)
			}
			blob, err := json.Marshal(client)
			if err != nil {
				t.Fatalf("encode client: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(blob, &got); err != nil {
				t.Fatalf("decode client output: %v", err)
			}
			if got["keepAlive"] != tt.want {
				t.Fatalf("keepAlive JSON = %#v, want %#v", got["keepAlive"], tt.want)
			}
		})
	}

	var omitted Client
	blob, err := json.Marshal(omitted)
	if err != nil {
		t.Fatalf("encode omitted client: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("decode omitted output: %v", err)
	}
	if _, ok := got["keepAlive"]; ok {
		t.Fatalf("omitted keepAlive serialized as %#v", got["keepAlive"])
	}
}

func TestKeepAliveValueValidatesNewInputAndPreservesLegacyScalar(t *testing.T) {
	for _, value := range []KeepAliveValue{"0", "25", "65535"} {
		if err := value.Validate(false); err != nil {
			t.Errorf("scalar %q rejected: %v", value, err)
		}
	}
	if err := KeepAliveValue("15-25").Validate(true); err != nil {
		t.Fatalf("AmneziaWG range rejected: %v", err)
	}
	if err := KeepAliveValue("15-25").Validate(false); err == nil || err.Error() != "keepAlive ranges are supported only for AmneziaWG clients" {
		t.Fatalf("WireGuard range error = %v", err)
	}
	for _, value := range []KeepAliveValue{"25-15", "0-65536", "65536", "01-2"} {
		if err := value.Validate(true); err == nil {
			t.Errorf("invalid new value %q accepted", value)
		}
	}

	var legacy KeepAliveValue
	if err := json.Unmarshal([]byte("70000"), &legacy); err != nil {
		t.Fatalf("read legacy numeric value: %v", err)
	}
	blob, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy numeric value: %v", err)
	}
	if string(blob) != "70000" {
		t.Fatalf("legacy numeric value = %s, want 70000", blob)
	}
	if got := legacy.Seconds(); got != 70000 {
		t.Fatalf("legacy scalar accessor = %d, want 70000", got)
	}
	if err := json.Unmarshal([]byte("-5"), &legacy); err != nil {
		t.Fatalf("read legacy negative numeric value: %v", err)
	}
	blob, err = json.Marshal(legacy)
	if err != nil || string(blob) != "-5" {
		t.Fatalf("legacy negative numeric value = %s, %v; want -5", blob, err)
	}
	if err := legacy.Validate(true); err == nil {
		t.Fatal("new-input validator accepted a legacy value above the panel bound")
	}
	if got := legacy.Seconds(); got != -5 {
		t.Fatalf("legacy negative scalar accessor = %d, want -5", got)
	}
}
