package tgbot

import (
	"fmt"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestPlanInboundPickerPagesWithoutLoadingClientConfigurations(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	localizeWithRealBundle(t)
	setTestAdmins(t, adminTgID)
	db := database.GetDB()
	for i := 0; i < 45; i++ {
		inbound := model.Inbound{UserId: 1, Tag: fmt.Sprintf("picker-%02d", i), Remark: fmt.Sprintf("host-%02d", i), Protocol: model.VLESS, Settings: `{"clients":[],"padding":"` + strings.Repeat("x", 1024) + `"}`}
		if err := db.Create(&inbound).Error; err != nil {
			t.Fatal(err)
		}
	}
	loadedSettings, clientQueries := 0, 0
	callback := "test:plan-picker-read-scope"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "clients" || tx.Statement.Table == "client_inbounds" || tx.Statement.Table == "client_traffics" {
			clientQueries++
		}
		if rows, ok := tx.Statement.Dest.(*[]*model.Inbound); ok {
			for _, row := range *rows {
				loadedSettings += len(row.Settings)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
	actor := chatUser{adminTgID, adminTgID}
	addClientDrafts.reset(actor)
	t.Cleanup(func() { addClientDrafts.reset(actor) })
	draft := addClientDrafts.forActor(actor)
	first := attachPickerRows(t, tb, draft)
	if len(first) > 23 {
		t.Fatalf("picker emitted %d buttons instead of a bounded page", len(first))
	}
	next := ""
	for _, button := range first {
		if button.CallbackData == "add_client_attach_page 1" {
			next = button.CallbackData
		}
	}
	if next == "" {
		t.Fatal("large plan picker has no next page")
	}
	adminWorkflowTap(tb, next, adminTgID)
	second := attachPickerRows(t, tb, draft)
	if first[0].CallbackData == second[0].CallbackData {
		t.Fatal("next page repeated the first inbound")
	}
	adminWorkflowTap(tb, second[0].CallbackData, adminTgID)
	if len(draft.receiverInboundIDs) != 1 {
		t.Fatal("selection on another page was lost")
	}
	if loadedSettings != 0 || clientQueries != 0 {
		t.Fatalf("label picker loaded %d configuration bytes and ran %d client-table reads", loadedSettings, clientQueries)
	}
}
