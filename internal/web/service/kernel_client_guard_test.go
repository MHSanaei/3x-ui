package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
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
		name, want string
		client     model.Client
	}{
		{"public key hook injection", "control characters", model.Client{Email: "bad-key@awg", Enable: true, PublicKey: "peer-key\n[Interface]\nPostDown = touch /tmp/pwned", AllowedIPs: []string{"10.8.1.2/32"}}},
		{"forwarded ports", "port forwarding is not supported", model.Client{Email: "bad-forward@awg", Enable: true, PublicKey: "peer-key", AllowedIPs: []string{"10.8.1.2/32"}, ForwardedPorts: "54321"}},
		{"IPv6 allowed IP", "per-peer IPv6 egress is not supported", model.Client{Email: "bad-v6@awg", Enable: true, PublicKey: "peer-key", AllowedIPs: []string{"2001:db8::2/128"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			inboundSvc := &InboundService{}
			svc := &ClientService{}
			ib := mkInbound(t, 52000, model.AmneziaWG, kernelAWGSettings(nil))
			tc.client.AllowedIPsByInbound = map[int][]string{ib.Id: tc.client.AllowedIPs}
			_, err := svc.CreateOne(inboundSvc, ib.Id, tc.client)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("CreateOne error = %v, want an error containing %q", err, tc.want)
			}
			var records int64
			if dbErr := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", tc.client.Email).Count(&records).Error; dbErr != nil {
				t.Fatal(dbErr)
			}
			if records != 0 {
				t.Fatalf("rejected create persisted %d client records", records)
			}
			after, getErr := inboundSvc.GetInbound(ib.Id)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if len(mustAWGClients(t, after)) != 0 {
				t.Fatal("rejected create changed inbound settings")
			}
		})
	}
}

func mustAWGClients(t *testing.T, ib *model.Inbound) []model.Client {
	t.Helper()
	clients, err := (&InboundService{}).GetClients(ib)
	if err != nil {
		t.Fatal(err)
	}
	return clients
}

func TestKernelAWGClientUpdateRejectsUnsafeValuesAndUnsupportedFeatures(t *testing.T) {
	cases := []struct {
		name, want string
		update     model.Client
	}{
		{"preshared key hook injection", "control characters", model.Client{Email: "kept@awg", PreSharedKey: "psk\n[Interface]\nPreDown = touch /tmp/pwned"}},
		{"forwarded ports", "port forwarding is not supported", model.Client{Email: "kept@awg", ForwardedPorts: "54321"}},
		{"IPv6 allowed IP", "per-peer IPv6 egress is not supported", model.Client{Email: "kept@awg", AllowedIPs: []string{"2001:db8::2/128"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			inboundSvc := &InboundService{}
			svc := &ClientService{}
			seed := model.Client{Email: "kept@awg", Enable: true, PublicKey: "valid-key", PrivateKey: "valid-private", AllowedIPs: []string{"10.8.1.2/32"}}
			ib := mkInbound(t, 52001, model.AmneziaWG, kernelAWGSettings(nil))
			if _, err := svc.CreateOne(inboundSvc, ib.Id, seed); err != nil {
				t.Fatalf("seed client: %v", err)
			}
			rec := lookupClientRecord(t, seed.Email)
			before, getErr := inboundSvc.GetInbound(ib.Id)
			if getErr != nil {
				t.Fatal(getErr)
			}
			update := tc.update
			if len(update.AllowedIPs) > 0 {
				update.AllowedIPsByInbound = map[int][]string{ib.Id: update.AllowedIPs}
			}
			_, err := svc.Update(inboundSvc, rec.Id, update, 0)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ClientService.Update error = %v, want an error containing %q", err, tc.want)
			}
			after, err := inboundSvc.GetInbound(ib.Id)
			if err != nil {
				t.Fatal(err)
			}
			if after.Settings != before.Settings {
				t.Fatalf("rejected update changed settings")
			}
			var unchanged model.ClientRecord
			if err := database.GetDB().First(&unchanged, rec.Id).Error; err != nil {
				t.Fatal(err)
			}
			if unchanged.PreSharedKey != rec.PreSharedKey || unchanged.AllowedIPs != rec.AllowedIPs {
				t.Fatalf("rejected update changed client record: before %+v after %+v", rec, unchanged)
			}
		})
	}
}

func TestKernelAWGClientFeatureChecksPreserveSupportedPaths(t *testing.T) {
	setupBulkDB(t)
	inboundSvc := &InboundService{}
	svc := &ClientService{}
	kernel := mkInbound(t, 52002, model.AmneziaWG, kernelAWGSettings(nil))
	if _, err := svc.CreateOne(inboundSvc, kernel.Id, model.Client{Email: "valid@kernel", Enable: true, PublicKey: "valid-key", AllowedIPs: []string{"10.8.1.2/32"}}); err != nil {
		t.Fatalf("valid IPv4 kernel peer was rejected: %v", err)
	}
	embedded := mkInbound(t, 52003, model.AmneziaWG, amneziawgClientTestSettings)
	if _, err := svc.CreateOne(inboundSvc, embedded.Id, model.Client{Email: "forward@embedded", Enable: true, PublicKey: "valid-key", AllowedIPs: []string{"10.8.1.3/32"}, ForwardedPorts: "54321"}); err != nil {
		t.Fatalf("embedded AWG port forwarding was rejected: %v", err)
	}
}

func TestKernelAWGSharedCreateAndUpdateValidateAllTargetsBeforeWrites(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		setupBulkDB(t)
		inboundSvc := &InboundService{}
		svc := &ClientService{}
		kernel := mkInbound(t, 52005, model.AmneziaWG, kernelAWGSettings(nil))
		embedded := mkInbound(t, 52006, model.AmneziaWG, amneziawgClientTestSettings)
		_, err := svc.Create(inboundSvc, &ClientCreatePayload{
			Client:     model.Client{Email: "shared-create@awg", Enable: true, PublicKey: "valid-key", AllowedIPs: []string{"10.8.1.2/32"}, ForwardedPorts: "54321"},
			InboundIds: []int{embedded.Id, kernel.Id},
		})
		if err == nil || !strings.Contains(err.Error(), "port forwarding is not supported") {
			t.Fatalf("Create error = %v, want kernel feature rejection", err)
		}
		for _, ib := range []*model.Inbound{embedded, kernel} {
			after, getErr := inboundSvc.GetInbound(ib.Id)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if len(mustAWGClients(t, after)) != 0 {
				t.Fatalf("failed shared create partially changed inbound %d", ib.Id)
			}
		}
		var records int64
		if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", "shared-create@awg").Count(&records).Error; err != nil {
			t.Fatal(err)
		}
		if records != 0 {
			t.Fatalf("failed shared create persisted %d client records", records)
		}
	})

	t.Run("update", func(t *testing.T) {
		setupBulkDB(t)
		inboundSvc := &InboundService{}
		svc := &ClientService{}
		kernel := mkInbound(t, 52007, model.AmneziaWG, kernelAWGSettings(nil))
		embedded := mkInbound(t, 52008, model.AmneziaWG, amneziawgClientTestSettings)
		seed := model.Client{Email: "shared-update@awg", Enable: true, PublicKey: "valid-key", AllowedIPsByInbound: map[int][]string{kernel.Id: {"10.8.1.2/32"}, embedded.Id: {"10.8.1.3/32"}}}
		if _, err := svc.Create(inboundSvc, &ClientCreatePayload{Client: seed, InboundIds: []int{kernel.Id, embedded.Id}}); err != nil {
			t.Fatalf("seed shared client: %v", err)
		}
		beforeKernel, _ := inboundSvc.GetInbound(kernel.Id)
		beforeEmbedded, _ := inboundSvc.GetInbound(embedded.Id)
		rec := lookupClientRecord(t, seed.Email)
		_, err := svc.Update(inboundSvc, rec.Id, model.Client{Email: seed.Email, ForwardedPorts: "54321"}, 0)
		if err == nil || !strings.Contains(err.Error(), "port forwarding is not supported") {
			t.Fatalf("Update error = %v, want kernel feature rejection", err)
		}
		afterKernel, getErr := inboundSvc.GetInbound(kernel.Id)
		if getErr != nil {
			t.Fatal(getErr)
		}
		afterEmbedded, getErr := inboundSvc.GetInbound(embedded.Id)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if afterKernel.Settings != beforeKernel.Settings || afterEmbedded.Settings != beforeEmbedded.Settings {
			t.Fatal("failed shared update partially changed inbound settings")
		}
		var unchanged model.ClientRecord
		if err := database.GetDB().First(&unchanged, rec.Id).Error; err != nil {
			t.Fatal(err)
		}
		if unchanged.ForwardedPorts != rec.ForwardedPorts {
			t.Fatal("failed shared update changed the client record")
		}
	})
}
