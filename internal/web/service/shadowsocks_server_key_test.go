package service

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func ss2022Settings(method, serverKey string) string {
	clientKey := base64.StdEncoding.EncodeToString(make([]byte, 16))
	if method != "2022-blake3-aes-128-gcm" {
		clientKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	}
	return fmt.Sprintf(`{"method":%q,"password":%q,"network":"tcp,udp","clients":[{"email":"ss-key","password":%q,"enable":true}]}`,
		method, serverKey, clientKey)
}

// xray-core refuses a wrong-size Shadowsocks-2022 server key while building the
// inbound, which keeps the whole core from starting, not just this inbound.
func TestShadowsocks2022ServerKeyRejected(t *testing.T) {
	key32 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	key16 := base64.StdEncoding.EncodeToString(make([]byte, 16))

	t.Run("add", func(t *testing.T) {
		setupConflictDB(t)
		nodeID, fake := setupNodeRuntime(t)
		ib := &model.Inbound{
			UserId: 1, NodeID: &nodeID, Tag: "ss-add", Enable: true, Port: 41801, Protocol: model.Shadowsocks,
			Settings: ss2022Settings("2022-blake3-aes-128-gcm", key32),
		}
		_, _, err := (&InboundService{}).AddInbound(ib)
		if err == nil || !strings.Contains(err.Error(), "2022-blake3-aes-128-gcm needs a 16-byte key") {
			t.Fatalf("AddInbound with a 32-byte aes-128 key: err = %v", err)
		}
		if fake.addInbound.Load() != 0 {
			t.Fatal("rejected inbound dispatched to the runtime")
		}
	})

	t.Run("update", func(t *testing.T) {
		setupConflictDB(t)
		nodeID, fake := setupNodeRuntime(t)
		svc := &InboundService{}
		ib := &model.Inbound{
			UserId: 1, NodeID: &nodeID, Tag: "ss-update", Enable: true, Port: 41802, Protocol: model.Shadowsocks,
			Settings: ss2022Settings("2022-blake3-aes-128-gcm", key16),
		}
		if _, _, err := svc.AddInbound(ib); err != nil {
			t.Fatalf("AddInbound with a valid key: %v", err)
		}
		update := *ib
		update.Settings = ss2022Settings("2022-blake3-aes-256-gcm", key16)
		_, _, err := svc.UpdateInbound(&update)
		if err == nil || !strings.Contains(err.Error(), "2022-blake3-aes-256-gcm needs a 32-byte key") {
			t.Fatalf("UpdateInbound keeping a 16-byte key on aes-256: err = %v", err)
		}
		if fake.updateInbound.Load() != 0 {
			t.Fatal("rejected update dispatched to the runtime")
		}
	})

	// The core reads a non-base64 key as its raw bytes, so this one is valid.
	t.Run("raw key the core accepts", func(t *testing.T) {
		setupConflictDB(t)
		nodeID, _ := setupNodeRuntime(t)
		ib := &model.Inbound{
			UserId: 1, NodeID: &nodeID, Tag: "ss-raw", Enable: true, Port: 41803, Protocol: model.Shadowsocks,
			Settings: ss2022Settings("2022-blake3-aes-128-gcm", "0123456789abcd-!"),
		}
		if _, _, err := (&InboundService{}).AddInbound(ib); err != nil {
			t.Fatalf("AddInbound with a raw 16-byte key: %v", err)
		}
	})
}
