package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func kernelAWGSettings(clients []model.Client) string {
	b, _ := json.Marshal(map[string]any{
		"server":  map[string]any{"subnetIp": "10.8.1.0", "subnetCidr": 24, "kernelMode": true},
		"clients": clients,
	})
	return string(b)
}

func TestKernelAWGClientWritesRejectUnsafeValuesAndUnsupportedFeatures(t *testing.T) {
	cases := []struct {
		name   string
		client model.Client
	}{
		{"public key hook injection", model.Client{Email: "bad-key@awg", PublicKey: "peer-key\n[Interface]\nPostDown = touch /tmp/pwned", AllowedIPs: []string{"10.8.1.2/32"}}},
		{"forwarded ports", model.Client{Email: "bad-forward@awg", PublicKey: "peer-key", AllowedIPs: []string{"10.8.1.2/32"}, ForwardedPorts: "54321"}},
		{"IPv6 allowed IP", model.Client{Email: "bad-v6@awg", PublicKey: "peer-key", AllowedIPs: []string{"2001:db8::2/128"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			inboundSvc := &InboundService{}
			svc := &ClientService{}
			ib := mkInbound(t, 52000, model.AmneziaWG, kernelAWGSettings(nil))
			before := ib.Settings
			_, err := svc.AddInboundClient(inboundSvc, &model.Inbound{Id: ib.Id, Settings: clientsSettings(t, []model.Client{tc.client})})
			if err == nil {
				t.Fatalf("AddInboundClient accepted a client field unsupported or unsafe in kernel mode: %v", err)
			}
			after, getErr := inboundSvc.GetInbound(ib.Id)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if after.Settings != before {
				t.Fatalf("rejected client changed saved settings:\nbefore %s\nafter %s", before, after.Settings)
			}
		})
	}
}

func TestKernelAWGClientUpdateRejectsUnsafeValuesAndUnsupportedFeatures(t *testing.T) {
	cases := []struct {
		name   string
		update model.Client
	}{
		{"preshared key hook injection", model.Client{Email: "kept@awg", PreSharedKey: "psk\n[Interface]\nPreDown = touch /tmp/pwned"}},
		{"forwarded ports", model.Client{Email: "kept@awg", ForwardedPorts: "54321"}},
		{"IPv6 allowed IP", model.Client{Email: "kept@awg", AllowedIPs: []string{"2001:db8::2/128"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			inboundSvc := &InboundService{}
			svc := &ClientService{}
			seed := model.Client{Email: "kept@awg", Enable: true, PublicKey: "valid-key", PrivateKey: "valid-private", AllowedIPs: []string{"10.8.1.2/32"}}
			ib := mkInbound(t, 52001, model.AmneziaWG, kernelAWGSettings([]model.Client{seed}))
			before := ib.Settings
			_, err := svc.UpdateInboundClient(inboundSvc, &model.Inbound{Id: ib.Id, Settings: clientsSettings(t, []model.Client{tc.update})}, "kept@awg")
			if err == nil {
				t.Fatalf("UpdateInboundClient accepted a client field unsupported or unsafe in kernel mode: %v", err)
			}
			after, getErr := inboundSvc.GetInbound(ib.Id)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if after.Settings != before {
				t.Fatalf("rejected client changed saved settings:\nbefore %s\nafter %s", before, after.Settings)
			}
		})
	}
}
