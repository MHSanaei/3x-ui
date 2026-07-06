package service

import (
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
)

func TestDesiredMtprotoInstancesFiltersDepleted(t *testing.T) {
	setupConflictDB(t)
	svc := &InboundService{}

	seedInboundConflict(t, "mt-desired", "", 46001, model.MTProto,
		"",
		`{"clients":[`+
			`{"email":"alice","secret":"`+mtprotoTestSecretA+`","enable":true},`+
			`{"email":"bob","secret":"`+mtprotoTestSecretB+`","enable":true},`+
			`{"email":"carol","secret":"`+mtprotoTestSecretC+`","enable":false}]}`)
	served := loadInboundByTag(t, "mt-desired")
	seedClientTraffic(t, served.Id, "alice", true)
	seedClientTraffic(t, served.Id, "bob", false)
	seedClientTraffic(t, served.Id, "carol", true)

	seedInboundConflict(t, "mt-all-depleted", "", 46002, model.MTProto,
		"",
		`{"clients":[{"email":"dave","secret":"`+mtprotoTestSecretA+`","enable":true}]}`)
	depleted := loadInboundByTag(t, "mt-all-depleted")
	seedClientTraffic(t, depleted.Id, "dave", false)

	nodeID := 5
	seedInboundConflictNode(t, "mt-node-owned", "", 46003, model.MTProto,
		"",
		`{"clients":[{"email":"erin","secret":"`+mtprotoTestSecretB+`","enable":true}]}`,
		&nodeID)

	instances, err := svc.DesiredMtprotoInstances()
	if err != nil {
		t.Fatalf("DesiredMtprotoInstances: %v", err)
	}

	t.Run("depletedAndDisabledClientsExcluded", func(t *testing.T) {
		if len(instances) != 1 {
			t.Fatalf("expected exactly the served inbound, got %d instances: %+v", len(instances), instances)
		}
		if instances[0].Id != served.Id {
			t.Fatalf("expected inbound %d, got %d", served.Id, instances[0].Id)
		}
		want := []mtproto.SecretEntry{{Name: "alice", Secret: mtprotoTestSecretA}}
		if !reflect.DeepEqual(instances[0].Secrets, want) {
			t.Fatalf("served secrets: got %+v, want %+v", instances[0].Secrets, want)
		}
	})

	t.Run("matchesInteractivePushFiltering", func(t *testing.T) {
		built, err := svc.buildRuntimeInboundForAPI(database.GetDB(), served)
		if err != nil {
			t.Fatalf("buildRuntimeInboundForAPI: %v", err)
		}
		pushInst, ok := mtproto.InstanceFromInbound(built)
		if !ok {
			t.Fatal("push path must produce an instance")
		}
		if !reflect.DeepEqual(pushInst.Secrets, instances[0].Secrets) {
			t.Fatalf("push and job secret sets diverge: push %+v, job %+v", pushInst.Secrets, instances[0].Secrets)
		}
	})
}
