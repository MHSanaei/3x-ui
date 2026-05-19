package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/xray"
)

func TestClientWithAttachmentsMarshalJSONIncludesExtras(t *testing.T) {
	c := ClientWithAttachments{
		ClientRecord: model.ClientRecord{Id: 1, Email: "alice@example.com"},
		InboundIds:   []int{3, 5},
		Traffic:      &xray.ClientTraffic{Email: "alice@example.com", Up: 1024, Down: 4096, Enable: true},
	}
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["email"] != "alice@example.com" {
		t.Errorf("expected ClientRecord fields to survive, got %v", parsed)
	}
	ids, ok := parsed["inboundIds"].([]any)
	if !ok {
		t.Fatalf("expected inboundIds to be present as an array, got %T (%s)", parsed["inboundIds"], out)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 inbound ids, got %d", len(ids))
	}
	if _, ok := parsed["traffic"].(map[string]any); !ok {
		t.Errorf("expected traffic to be present as an object, got %T", parsed["traffic"])
	}
}

func TestClientWithAttachmentsMarshalJSONOmitsAbsentTraffic(t *testing.T) {
	c := ClientWithAttachments{
		ClientRecord: model.ClientRecord{Id: 1, Email: "bob@example.com"},
		InboundIds:   nil,
	}
	out, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, present := parsed["traffic"]; present {
		t.Errorf("expected traffic to be omitted when nil, got %v", parsed["traffic"])
	}
	if _, present := parsed["inboundIds"]; !present {
		t.Errorf("expected inboundIds key to always be present, got %s", out)
	}
}
