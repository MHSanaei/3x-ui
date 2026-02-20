package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

func TestApplySingleClientUpdate(t *testing.T) {
	svc := &InboundService{}
	inbound := &model.Inbound{Settings: `{"clients":[{"email":"a@example.com","limitIp":1},{"email":"b@example.com","limitIp":2}]}`}

	err := svc.applySingleClientUpdate(inbound, "b@example.com", func(client map[string]any) {
		client["limitIp"] = 9
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		t.Fatalf("unmarshal updated settings: %v", err)
	}
	clients := settings["clients"].([]any)
	if len(clients) != 1 {
		t.Fatalf("expected one updated client payload, got %d", len(clients))
	}
	client := clients[0].(map[string]any)
	if client["email"] != "b@example.com" {
		t.Fatalf("unexpected updated client email: %v", client["email"])
	}
	if int(client["limitIp"].(float64)) != 9 {
		t.Fatalf("expected limitIp=9, got %v", client["limitIp"])
	}
	if _, ok := client["updated_at"]; !ok {
		t.Fatalf("expected updated_at to be set")
	}
}

func TestApplySingleClientUpdateMissingClient(t *testing.T) {
	svc := &InboundService{}
	inbound := &model.Inbound{Settings: `{"clients":[{"email":"a@example.com"}]}`}

	err := svc.applySingleClientUpdate(inbound, "x@example.com", func(client map[string]any) {})
	if err == nil {
		t.Fatalf("expected missing client error")
	}
}
