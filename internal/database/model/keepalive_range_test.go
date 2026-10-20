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
