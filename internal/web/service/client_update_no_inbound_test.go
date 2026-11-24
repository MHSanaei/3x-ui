package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestUpdate_PersistsFields_NoInbound(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(c *model.Client)
		readBack func(rec *model.ClientRecord) any
		want     any
	}{
		{
			name:     "subId",
			mutate:   func(c *model.Client) { c.SubID = "new-sub-id" },
			readBack: func(rec *model.ClientRecord) any { return rec.SubID },
			want:     "new-sub-id",
		},
		{
			name:     "totalGB cleared to zero",
			mutate:   func(c *model.Client) { c.TotalGB = 0 },
			readBack: func(rec *model.ClientRecord) any { return rec.TotalGB },
			want:     int64(0),
		},
		{
			name:     "expiryTime",
			mutate:   func(c *model.Client) { c.ExpiryTime = 1700000000 },
			readBack: func(rec *model.ClientRecord) any { return rec.ExpiryTime },
			want:     int64(1700000000),
		},
		{
			name:     "limitIp",
			mutate:   func(c *model.Client) { c.LimitIP = 7 },
			readBack: func(rec *model.ClientRecord) any { return rec.LimitIP },
			want:     7,
		},
		{
			name:     "tgId",
			mutate:   func(c *model.Client) { c.TgID = 9876543210 },
			readBack: func(rec *model.ClientRecord) any { return rec.TgID },
			want:     int64(9876543210),
		},
		{
			name:     "comment cleared to empty",
			mutate:   func(c *model.Client) { c.Comment = "" },
			readBack: func(rec *model.ClientRecord) any { return rec.Comment },
			want:     "",
		},
		{
			name:     "reset",
			mutate:   func(c *model.Client) { c.Reset = 30 },
			readBack: func(rec *model.ClientRecord) any { return rec.Reset },
			want:     30,
		},
		{
			name:     "flow",
			mutate:   func(c *model.Client) { c.Flow = "xtls-rprx-vision" },
			readBack: func(rec *model.ClientRecord) any { return rec.Flow },
			want:     "xtls-rprx-vision",
		},
		{
			name:     "security",
			mutate:   func(c *model.Client) { c.Security = "aes-128-gcm" },
			readBack: func(rec *model.ClientRecord) any { return rec.Security },
			want:     "aes-128-gcm",
		},
		{
			name:     "uuid rotated",
			mutate:   func(c *model.Client) { c.ID = "22222222-2222-2222-2222-222222222222" },
			readBack: func(rec *model.ClientRecord) any { return rec.UUID },
			want:     "22222222-2222-2222-2222-222222222222",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			inboundSvc := &InboundService{}

			email := "noib-" + strings.ReplaceAll(strings.ToLower(tc.name), " ", "-") + "@x"
			rec := &model.ClientRecord{
				Email:      email,
				UUID:       "11111111-1111-1111-1111-111111111111",
				SubID:      email,
				TotalGB:    5,
				ExpiryTime: 1000,
				LimitIP:    1,
				TgID:       1,
				Comment:    "seeded",
				Reset:      1,
				Flow:       "seeded-flow",
				Security:   "seeded-sec",
			}
			if err := database.GetDB().Create(rec).Error; err != nil {
				t.Fatalf("create record: %v", err)
			}

			updated := rec.ToClient()
			tc.mutate(updated)
			if _, err := svc.Update(inboundSvc, rec.Id, *updated, rec.LimitHwid); err != nil {
				t.Fatalf("Update: %v", err)
			}

			got, err := svc.GetByID(rec.Id)
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			if tc.readBack(got) != tc.want {
				t.Fatalf("%s: not persisted for no-inbound client, got %v, want %v", tc.name, tc.readBack(got), tc.want)
			}
		})
	}
}

func TestUpdate_NoInbound_PreservesCredentialsWhenOmitted(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "noib-preserve@x"
	rec := &model.ClientRecord{
		Email:    email,
		UUID:     "11111111-1111-1111-1111-111111111111",
		SubID:    email,
		Password: "seeded-pw",
		Auth:     "seeded-auth",
		Secret:   "seeded-secret",
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}

	updated := rec.ToClient()
	updated.ID = ""
	updated.Password = ""
	updated.Auth = ""
	updated.Secret = ""
	updated.Comment = "only comment changed"
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, rec.LimitHwid); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := svc.GetByID(rec.Id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UUID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("uuid wiped on partial update, got %q", got.UUID)
	}
	if got.Password != "seeded-pw" {
		t.Fatalf("password wiped on partial update, got %q", got.Password)
	}
	if got.Auth != "seeded-auth" {
		t.Fatalf("auth wiped on partial update, got %q", got.Auth)
	}
	if got.Secret != "seeded-secret" {
		t.Fatalf("secret wiped on partial update, got %q", got.Secret)
	}
	if got.Comment != "only comment changed" {
		t.Fatalf("comment not persisted, got %q", got.Comment)
	}
}

// The renewal and auto-disable loops act on client_traffics, which nothing else
// keeps in step once the client's last inbound is detached (#7160).
func TestUpdate_NoInbound_SyncsEnforcementRow(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "noib-enforcement@x"
	rec := &model.ClientRecord{
		Email:      email,
		UUID:       "11111111-1111-1111-1111-111111111111",
		SubID:      email,
		TotalGB:    5,
		ExpiryTime: 1000,
		Reset:      1,
		Enable:     true,
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId:  999,
		Email:      email,
		Enable:     true,
		Up:         123,
		Down:       456,
		ExpiryTime: 1000,
		Total:      5,
		Reset:      1,
	}).Error; err != nil {
		t.Fatalf("create client_traffics row: %v", err)
	}

	updated := rec.ToClient()
	updated.Enable = false
	updated.ExpiryTime = 1700000000
	updated.TotalGB = 10
	updated.Reset = 0
	updated.ResetDay = 15
	updated.ResetMax = 3
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, rec.LimitHwid); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var stat xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&stat).Error; err != nil {
		t.Fatalf("read client_traffics: %v", err)
	}
	type enforcement struct {
		Enable                      bool
		ExpiryTime, Total, Up, Down int64
		Reset, ResetDay, ResetMax   int
	}
	got := enforcement{stat.Enable, stat.ExpiryTime, stat.Total, stat.Up, stat.Down, stat.Reset, stat.ResetDay, stat.ResetMax}
	want := enforcement{false, 1700000000, 10, 123, 456, 0, 15, 3}
	if got != want {
		t.Fatalf("client_traffics row = %+v, want %+v", got, want)
	}
}

func TestApplyClientRecordMerge_MirrorsSyncInboundRules(t *testing.T) {
	row := &model.ClientRecord{
		UUID:     "kept-uuid",
		Password: "kept-pw",
		Flow:     "kept-flow",
		TotalGB:  9,
		Group:    "kept-group",
		Comment:  "kept-comment",
	}
	incoming := &model.ClientRecord{
		Password: "new-pw",
		TotalGB:  0,
		Comment:  "new-comment",
	}

	applyClientRecordMerge(row, incoming)

	if row.UUID != "kept-uuid" {
		t.Fatalf("empty incoming UUID should preserve stored UUID, got %q", row.UUID)
	}
	if row.Password != "new-pw" {
		t.Fatalf("non-empty incoming Password should overwrite, got %q", row.Password)
	}
	if row.Flow != "" {
		t.Fatalf("incoming Flow is unconditional and should overwrite with empty, got %q", row.Flow)
	}
	if row.TotalGB != 0 {
		t.Fatalf("incoming TotalGB is unconditional and should overwrite with zero, got %v", row.TotalGB)
	}
	if row.Group != "kept-group" {
		t.Fatalf("empty incoming Group should preserve stored group, got %q", row.Group)
	}
	if row.Comment != "new-comment" {
		t.Fatalf("incoming Comment is unconditional and should overwrite, got %q", row.Comment)
	}
}
