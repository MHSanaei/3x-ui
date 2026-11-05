package controller

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// A handler that binds the body straight into model.Client turns every omitted
// field into its zero: this expiry-only body would disable the client.
func TestUpdateHandlerKeepsFieldsTheBodyOmits(t *testing.T) {
	const email = "partial-body@example.com"
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	dbtest.InitDB(t, filepath.Join(dbDir, "x-ui.db"))
	db := database.GetDB()
	ib := &model.Inbound{
		UserId: 1, Enable: true, Port: 43350, Tag: "in-partial-body",
		Protocol: model.VLESS, Settings: `{"clients": []}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if _, err := (&service.ClientService{}).Create(&service.InboundService{}, &service.ClientCreatePayload{
		Client: model.Client{
			Email: email, ID: "11111111-2222-3333-4444-555555555555", Enable: true,
			LimitIP: 2, TotalGB: 10 << 30, TgID: 42, Comment: "phone",
		},
		InboundIds: []int{ib.Id},
	}); err != nil {
		t.Fatalf("seed client: %v", err)
	}

	a := &ClientController{}
	c, w := postCtx(t, email, map[string]any{"email": email, "expiryTime": 1893456000000})
	a.update(c)

	var rec model.ClientRecord
	if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
		t.Fatalf("reload client: %v (response %s)", err, w.Body.String())
	}
	if rec.ExpiryTime != 1893456000000 {
		t.Fatalf("expiry not applied: %d (response %s)", rec.ExpiryTime, w.Body.String())
	}
	if !rec.Enable || rec.LimitIP != 2 || rec.TotalGB != 10<<30 || rec.TgID != 42 || rec.Comment != "phone" {
		t.Fatalf("handler reset fields the body omitted: %+v", rec)
	}
}

func TestClientQueryAliasPartialUpdateKeepsOmittedFields(t *testing.T) {
	for _, email := range []string{".", ".."} {
		t.Run(email, func(t *testing.T) {
			router, rec, _ := seedLegacyDotClient(t, email)
			db := database.GetDB()
			if err := db.Model(&model.ClientRecord{}).Where("id = ?", rec.Id).Updates(map[string]any{"limit_ip": 2, "total_gb": int64(10 << 30), "comment": "retained"}).Error; err != nil {
				t.Fatal(err)
			}
			w := dotClientRequest(t, router, "POST", "update?email="+email, map[string]any{"email": "recovered-partial", "expiryTime": int64(1893456000000)})
			assertDotClientSuccess(t, w)
			var after model.ClientRecord
			if err := db.First(&after, rec.Id).Error; err != nil {
				t.Fatal(err)
			}
			if after.Email != "recovered-partial" || after.ExpiryTime != 1893456000000 || !after.Enable || after.UUID != rec.UUID || after.SubID != rec.SubID || after.LimitIP != 2 || after.TotalGB != 10<<30 || after.Comment != "retained" {
				t.Fatalf("query alias partial update lost client state: %+v", after)
			}
		})
	}
}
