package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInboundMarshalJSONNestsObjectFields(t *testing.T) {
	in := Inbound{
		Id:             7,
		Protocol:       VLESS,
		Port:           443,
		Settings:       `{"clients":[],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp"}`,
		Sniffing:       `{"enabled":true}`,
	}
	out, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, field := range []string{"settings", "streamSettings", "sniffing"} {
		if _, ok := parsed[field].(map[string]any); !ok {
			t.Errorf("expected %s to marshal as a JSON object, got %T", field, parsed[field])
		}
	}
	if strings.Contains(string(out), `"settings":"`) {
		t.Errorf("settings should not be emitted as a JSON string: %s", out)
	}
}

func TestInboundMarshalJSONEmptyFieldsBecomeNull(t *testing.T) {
	in := Inbound{Id: 1, Protocol: VLESS}
	out, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	for _, field := range []string{"settings", "streamSettings", "sniffing"} {
		if parsed[field] != nil {
			t.Errorf("expected %s to be null, got %v", field, parsed[field])
		}
	}
}

func TestInboundUnmarshalJSONAcceptsBothShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "nested objects (modern)",
			body: `{"id":1,"settings":{"clients":[],"decryption":"none"},"streamSettings":{"network":"tcp"},"sniffing":{"enabled":true}}`,
		},
		{
			name: "JSON-encoded strings (legacy)",
			body: `{"id":1,"settings":"{\"clients\":[],\"decryption\":\"none\"}","streamSettings":"{\"network\":\"tcp\"}","sniffing":"{\"enabled\":true}"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var in Inbound
			if err := json.Unmarshal([]byte(tc.body), &in); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if !strings.Contains(in.Settings, `"decryption":"none"`) {
				t.Errorf("Settings not normalised: %q", in.Settings)
			}
			if !strings.Contains(in.StreamSettings, `"network":"tcp"`) {
				t.Errorf("StreamSettings not normalised: %q", in.StreamSettings)
			}
			if !strings.Contains(in.Sniffing, `"enabled":true`) {
				t.Errorf("Sniffing not normalised: %q", in.Sniffing)
			}
		})
	}
}

func TestInboundMarshalJSONInvalidTextFallsBackToString(t *testing.T) {
	in := Inbound{Id: 1, Settings: "not json at all"}
	out, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.Contains(string(out), `"settings":"not json at all"`) {
		t.Errorf("expected invalid settings text to be wrapped as a JSON string, got %s", out)
	}
}

func TestClientRecordMarshalJSONNestsReverse(t *testing.T) {
	rec := ClientRecord{Id: 1, Email: "alice@example.com", Reverse: `{"tag":"vless-in"}`}
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	obj, ok := parsed["reverse"].(map[string]any)
	if !ok {
		t.Fatalf("expected reverse to marshal as a JSON object, got %T", parsed["reverse"])
	}
	if obj["tag"] != "vless-in" {
		t.Errorf("expected tag to be preserved, got %v", obj["tag"])
	}
}

func TestClientRecordMarshalJSONEmptyReverseIsNull(t *testing.T) {
	rec := ClientRecord{Id: 1, Email: "alice@example.com"}
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["reverse"] != nil {
		t.Errorf("expected reverse to be null, got %v", parsed["reverse"])
	}
}

func TestClientRecordUnmarshalJSONAcceptsBothShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "nested object", body: `{"id":1,"reverse":{"tag":"vless-in"}}`},
		{name: "legacy string", body: `{"id":1,"reverse":"{\"tag\":\"vless-in\"}"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rec ClientRecord
			if err := json.Unmarshal([]byte(tc.body), &rec); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if !strings.Contains(rec.Reverse, `"tag":"vless-in"`) {
				t.Errorf("Reverse not normalised: %q", rec.Reverse)
			}
		})
	}
}

func TestInboundClientIpsMarshalJSONNestsArray(t *testing.T) {
	row := InboundClientIps{Id: 1, ClientEmail: "alice@example.com", Ips: `[{"ip":"1.2.3.4","timestamp":1700000000}]`}
	out, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	arr, ok := parsed["ips"].([]any)
	if !ok {
		t.Fatalf("expected ips to marshal as a JSON array, got %T", parsed["ips"])
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 entry, got %d", len(arr))
	}
}

func TestInboundClientIpsUnmarshalJSONAcceptsBothShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "nested array", body: `{"id":1,"ips":[{"ip":"1.2.3.4","timestamp":1}]}`},
		{name: "legacy string", body: `{"id":1,"ips":"[{\"ip\":\"1.2.3.4\",\"timestamp\":1}]"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var row InboundClientIps
			if err := json.Unmarshal([]byte(tc.body), &row); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if !strings.Contains(row.Ips, `"ip":"1.2.3.4"`) {
				t.Errorf("Ips not normalised: %q", row.Ips)
			}
		})
	}
}
