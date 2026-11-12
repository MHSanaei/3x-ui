package service

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientHwidBanIndependentOfLimit(t *testing.T) {
	for _, limit := range []int{0, 1} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			initClientHwidTestDB(t)
			rec := seedHwidClient(t, limit)
			svc := &ClientService{}
			const hwid = "device-banned"
			if r, err := svc.EnforceHwidForSubID(rec.SubID, HwidRequest{Hwid: hwid}); err != nil || !r.Allowed {
				t.Fatalf("register: %+v, %v", r, err)
			}
			list, err := svc.ListClientHwids(rec.Email)
			if err != nil || len(list) != 1 {
				t.Fatalf("list: %+v, %v", list, err)
			}
			id := list[0].Id
			if err := svc.SetClientHwidBlocked(rec.Email, id, true); err != nil {
				t.Fatal(err)
			}
			assertBanned := func() {
				t.Helper()
				r, err := svc.EnforceHwidForSubID(rec.SubID, HwidRequest{Hwid: " " + hwid + " "})
				if err != nil || r.Allowed || !r.Blocked || r.MaxDevicesReached || r.NotSupported {
					t.Fatalf("banned device: %+v, %v", r, err)
				}
			}
			assertBanned()
			// A ban must survive both automatic trimming and manual list clearing.
			for _, newLimit := range []int{2, 1, 0} {
				if err := svc.setClientLimitHwidByEmail(rec.Email, newLimit); err != nil {
					t.Fatal(err)
				}
				assertBanned()
			}
			if err := svc.DeleteClientHwid(rec.Email, id); err == nil {
				t.Fatal("deleting a banned row must require unblocking first")
			}
			if err := svc.ClearClientHwids(rec.Email); err != nil {
				t.Fatal(err)
			}
			assertBanned()
			list, err = svc.ListClientHwids(rec.Email)
			if err != nil || len(list) != 1 || !list[0].Blocked {
				t.Fatalf("ban missing from panel: %+v, %v", list, err)
			}
			if err := svc.SetClientHwidBlocked(rec.Email, id, false); err != nil {
				t.Fatal(err)
			}
			if r, err := svc.EnforceHwidForSubID(rec.SubID, HwidRequest{Hwid: hwid}); err != nil || !r.Allowed || r.Blocked {
				t.Fatalf("unblocked: %+v, %v", r, err)
			}
		})
	}
}

func TestClientHwidBanScopeAndSlots(t *testing.T) {
	initClientHwidTestDB(t)
	rec := seedHwidClient(t, 1)
	svc := &ClientService{}
	if _, err := svc.EnforceHwidForSubID(rec.SubID, HwidRequest{Hwid: "shared-device"}); err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListClientHwids(rec.Email)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %+v, %v", list, err)
	}
	other := &model.ClientRecord{Email: "other-ban@example.com", SubID: "other-ban-sub", Enable: true}
	if err := database.GetDB().Create(other).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.SetClientHwidBlocked(other.Email, list[0].Id, true); err == nil {
		t.Fatal("must not ban a foreign subscription's device")
	}
	if err := svc.SetClientHwidBlocked(rec.Email, list[0].Id, true); err != nil {
		t.Fatal(err)
	}
	status, found, err := svc.HwidSlotStatusForSubID(rec.SubID)
	if err != nil || !found || status.Registered != 0 || status.Remaining != 1 || status.Full {
		t.Fatalf("ban must free a slot: %+v, %v", status, err)
	}
	if r, err := svc.EnforceHwidForSubID(rec.SubID, HwidRequest{Hwid: "another-device"}); err != nil || !r.Allowed {
		t.Fatalf("another device should take freed slot: %+v, %v", r, err)
	}
	if r, err := svc.EnforceHwidForSubID(other.SubID, HwidRequest{Hwid: "shared-device"}); err != nil || !r.Allowed {
		t.Fatalf("same HWID in other subscription: %+v, %v", r, err)
	}
	if err := svc.SetClientHwidBlocked(other.Email, list[0].Id, false); err == nil {
		t.Fatal("must not unblock a foreign subscription's device")
	}
}
