package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestBulkCreatePersistsClientHostAssignments(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	first := mkInbound(t, 41501, model.VLESS, `{"clients":[]}`)
	second := mkInbound(t, 41502, model.VLESS, `{"clients":[]}`)
	if err := database.GetDB().Create(&model.Host{
		GroupId:   "group-a",
		InboundId: first.Id,
		Remark:    "host-a",
		Address:   "a.example.com",
	}).Error; err != nil {
		t.Fatalf("create host a: %v", err)
	}
	if err := database.GetDB().Create(&model.Host{
		GroupId:   "group-b",
		InboundId: second.Id,
		Remark:    "host-b",
		Address:   "b.example.com",
	}).Error; err != nil {
		t.Fatalf("create host b: %v", err)
	}

	result, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{
		{
			Client: model.Client{
				Email:  "bulk-host@x",
				ID:     "66666666-7777-8888-9999-000000000000",
				SubID:  "sub-bulk-host",
				Enable: true,
			},
			InboundIds:   []int{first.Id, second.Id},
			HostGroupIds: []string{"group-a"},
		},
	})
	if err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	if result.Created != 1 || len(result.Skipped) != 0 {
		t.Fatalf("result = %+v, want one created and no skipped", result)
	}

	rec := lookupClientRecord(t, "bulk-host@x")
	got, err := (&ClientHostService{}).GetGroupIDs(rec.Id)
	if err != nil {
		t.Fatalf("GetGroupIDs: %v", err)
	}
	if len(got) != 1 || got[0] != "group-a" {
		t.Fatalf("host assignments = %#v, want [group-a]", got)
	}

	invalid, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{
		{
			Client: model.Client{
				Email:  "invalid-bulk-host@x",
				ID:     "77777777-8888-9999-0000-111111111111",
				SubID:  "sub-invalid-bulk-host",
				Enable: true,
			},
			InboundIds:   []int{first.Id},
			HostGroupIds: []string{"missing-group"},
		},
	})
	if err != nil {
		t.Fatalf("BulkCreate with invalid host group returned fatal error: %v", err)
	}
	if invalid.Created != 0 || len(invalid.Skipped) != 1 {
		t.Fatalf("invalid result = %+v, want one skipped", invalid)
	}
	if database.GetDB().Where("email = ?", "invalid-bulk-host@x").Find(&model.ClientRecord{}).RowsAffected != 0 {
		t.Fatal("client with invalid host group was created")
	}
}
