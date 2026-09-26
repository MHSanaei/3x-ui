package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientWeeklyRenewCRUD(t *testing.T) {
	setupBulkDB(t)
	svc, inboundSvc := &ClientService{}, &InboundService{}
	first := mkInbound(t, 41701, model.VLESS, `{"clients":[]}`)
	second := mkInbound(t, 41702, model.VLESS, `{"clients":[]}`)
	client := model.Client{
		Email: "weekly-crud", ID: "11111111-1111-1111-1111-111111111111", Enable: true,
		ResetWeekday: 7, ResetMax: 3, ExpiryTime: time.Now().Add(-time.Hour).UnixMilli(),
	}
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{Client: client, InboundIds: []int{first.Id, second.Id}}); err != nil {
		t.Fatal(err)
	}
	record, err := svc.GetRecordByEmail(nil, client.Email)
	if err != nil {
		t.Fatal(err)
	}
	assertSchedule := func(want int) {
		t.Helper()
		got, err := svc.GetRecordByEmail(nil, client.Email)
		if err != nil || got.ResetWeekday != want || got.ToClient().ResetWeekday != want {
			t.Fatalf("record schedule/error = %+v/%v, want %d", got, err, want)
		}
		traffic := readTraffic(t, database.GetDB(), client.Email)
		if traffic.ResetWeekday != want || traffic.ResetMax != 3 {
			t.Fatalf("traffic policy = %+v, want weekday %d and cap 3", traffic, want)
		}
		for _, id := range []int{first.Id, second.Id} {
			var inbound model.Inbound
			if err := database.GetDB().First(&inbound, id).Error; err != nil {
				t.Fatal(err)
			}
			clients, err := inboundSvc.GetClients(&inbound)
			if err != nil || len(clients) != 1 || clients[0].ResetWeekday != want {
				t.Fatalf("inbound %d schedule/error = %+v/%v, want %d", id, clients, err, want)
			}
		}
		filter := "on"
		if want == 0 {
			filter = "off"
		}
		page, err := svc.ListPaged(inboundSvc, nil, ClientPageParams{AutoRenew: filter})
		if err != nil || len(page.Items) != 1 || page.Items[0].ResetWeekday != want {
			t.Fatalf("renewal filter/projection = %+v/%v, want weekday %d", page, err, want)
		}
	}
	assertSchedule(7)
	if deleted, _, err := svc.DelDepleted(inboundSvc); err != nil || deleted != 0 {
		t.Fatalf("expired weekly client was purged: deleted/error = %d/%v", deleted, err)
	}
	for _, weekday := range []int{2, 0} {
		updated := *record.ToClient()
		updated.ResetWeekday = weekday
		if _, err := svc.Update(inboundSvc, record.Id, updated, 0); err != nil {
			t.Fatal(err)
		}
		assertSchedule(weekday)
	}
	if deleted, _, err := svc.DelDepleted(inboundSvc); err != nil || deleted != 1 {
		t.Fatalf("non-renewing expired client was not purged: deleted/error = %d/%v", deleted, err)
	}
}
