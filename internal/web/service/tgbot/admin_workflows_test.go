package tgbot

import (
	"context"
	"strings"
	"testing"

	"github.com/mymmrac/telego"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type adminBotRuntime struct{ runtime.Runtime }

func (*adminBotRuntime) Name() string { return "local" }

func (*adminBotRuntime) AddUser(context.Context, *model.Inbound, map[string]any) error { return nil }

func (*adminBotRuntime) UpdateUser(context.Context, *model.Inbound, string, model.Client) error {
	return nil
}

func adminWorkflowBot(t *testing.T) *Tgbot {
	t.Helper()
	actor := chatUser{adminTgID, adminTgID}
	addClientDrafts.reset(actor)
	t.Cleanup(func() { addClientDrafts.reset(actor); userStateMgr.clear(actor) })
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	localizeWithRealBundle(t)
	setTestAdmins(t, adminTgID)
	seedAdminClient(t, adminCardEmail)
	ids, err := tb.clientService.GetInboundIdsForEmail(nil, adminCardEmail)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&service.ClientPlanService{}).Save(service.ClientPlan{Name: "A", Days: 30, QuotaBytes: 30 << 30, InboundIDs: ids, Enabled: true, RenewalMode: "replace"}); err != nil {
		t.Fatal(err)
	}
	mgr := runtime.NewManager(runtime.LocalDeps{})
	mgr.SetLocalRuntimeOverride(&adminBotRuntime{})
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })
	return tb
}

func adminWorkflowTap(tb *Tgbot, data string, userID int64) {
	tb.answerCallback(&telego.CallbackQuery{ID: "admin-workflow", From: telego.User{ID: userID}, Data: data, Message: &telego.Message{Chat: telego.Chat{ID: userID, Type: "private"}}}, true)
}

func workflowCallback(t *testing.T, tb *Tgbot, prefix string) string {
	t.Helper()
	screen, ok := tb.screens().get(adminTgID)
	if !ok {
		t.Fatal("no workflow screen")
	}
	for _, row := range screen.rows {
		for _, button := range row {
			if strings.HasPrefix(button.CallbackData, prefix) {
				return button.CallbackData
			}
		}
	}
	t.Fatalf("screen offers no %q: %+v", prefix, screen.rows)
	return ""
}

func TestQuickCreateCommandRequiresPrivateAdminAndExplicitConfirmation(t *testing.T) {
	tb := adminWorkflowBot(t)
	message := &telego.Message{Text: "/add ali A", From: &telego.User{ID: adminTgID}, Chat: telego.Chat{ID: adminTgID, Type: "private"}}
	tb.answerCommand(message, adminTgID, true)
	confirm := workflowCallback(t, tb, "admin_confirm ")
	var count int64
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", "ali").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("command created before confirmation")
	}
	adminWorkflowTap(tb, confirm, ownerTgID)
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", "ali").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("foreign confirmation created client")
	}
	adminWorkflowTap(tb, confirm, adminTgID)
	record, err := tb.clientService.GetRecordByEmail(nil, "ali")
	if err != nil {
		t.Fatal(err)
	}
	if record.TotalGB != 30<<30 || record.UUID == "" {
		t.Fatalf("incorrect quick creation: %+v", record)
	}
	adminWorkflowTap(tb, confirm, adminTgID)
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", "ali").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("repeated callback duplicated client")
	}
	_ = workflowCallback(t, tb, "client_export_menu ")
}

func TestQuickRenewCommandPreviewDisclosesResetAndAdvancedMode(t *testing.T) {
	tb := adminWorkflowBot(t)
	message := &telego.Message{Text: "/renew " + adminCardEmail + " 30gb 30d", From: &telego.User{ID: adminTgID}, Chat: telego.Chat{ID: adminTgID, Type: "private"}}
	tb.answerCommand(message, adminTgID, true)
	confirm := workflowCallback(t, tb, "admin_confirm ")
	op, err := (&service.ClientAdminWorkflowService{}).Load(strings.TrimPrefix(confirm, "admin_confirm "), adminTgID)
	if err != nil {
		t.Fatal(err)
	}
	if !op.Preview.ResetUsage || op.Spec.Mode != "replace" || op.Preview.NewQuotaBytes != 30<<30 {
		t.Fatalf("wrong default: %+v", op)
	}
	writeBotScreenEvidence(t, tb, adminTgID, "renewal")
	message.Text += " extend"
	tb.answerCommand(message, adminTgID, true)
	confirm = workflowCallback(t, tb, "admin_confirm ")
	op, err = (&service.ClientAdminWorkflowService{}).Load(strings.TrimPrefix(confirm, "admin_confirm "), adminTgID)
	if err != nil {
		t.Fatal(err)
	}
	if op.Preview.ResetUsage || op.Spec.Mode != "extend" {
		t.Fatalf("advanced mode silently resets: %+v", op)
	}
	for _, invalid := range []string{"/renew " + adminCardEmail + " -1gb 30d", "/renew " + adminCardEmail + " 30gb 1.5d", "/renew " + adminCardEmail + " 9999999999999999999999gb 30d"} {
		message.Text = invalid
		tb.answerCommand(message, adminTgID, true)
		if current := workflowCallback(t, tb, "admin_confirm "); current != confirm {
			t.Fatal("invalid command replaced valid pending operation")
		}
	}
}

func TestPlanEditorReusesWizardAndCannotCreateAClient(t *testing.T) {
	tb := adminWorkflowBot(t)
	adminWorkflowTap(tb, "admin_plan_new", adminTgID)
	draft := addClientDrafts.forActor(chatUser{adminTgID, adminTgID})
	draft.Lock()
	if draft.plan == nil {
		draft.Unlock()
		t.Fatal("plan editor did not open")
	}
	draft.email = "B"
	ids, err := tb.clientService.GetInboundIdsForEmail(nil, adminCardEmail)
	if err != nil {
		draft.Unlock()
		t.Fatal(err)
	}
	draft.receiverInboundIDs = ids
	draft.Unlock()
	adminWorkflowTap(tb, "add_client_limit_traffic_c 60", adminTgID)
	adminWorkflowTap(tb, "add_client_reset_exp_c 90", adminTgID)
	writeBotScreenEvidence(t, tb, adminTgID, "plan-editor")
	adminWorkflowTap(tb, "add_client_submit_enable", adminTgID)
	if _, err := tb.clientService.GetRecordByEmail(nil, "B"); err == nil {
		t.Fatal("forged client submit created preset name as client")
	}
	adminWorkflowTap(tb, workflowCallback(t, tb, "admin_plan_save "), adminTgID)
	plan, err := (&service.ClientPlanService{}).Resolve("B")
	if err != nil {
		t.Fatal(err)
	}
	if plan.QuotaBytes != 60<<30 || plan.Days != 90 || len(plan.InboundIDs) != 1 {
		t.Fatalf("wizard changes missing: %+v", plan)
	}
}

func TestPlanEditorOldSaveCannotPersistANewDraft(t *testing.T) {
	tb := adminWorkflowBot(t)
	adminWorkflowTap(tb, "admin_plan_new", adminTgID)
	oldSave := workflowCallback(t, tb, "admin_plan_save ")
	adminWorkflowTap(tb, "admin_plan_new", adminTgID)
	draft := addClientDrafts.forActor(chatUser{adminTgID, adminTgID})
	draft.Lock()
	draft.email = "B"
	ids, err := tb.clientService.GetInboundIdsForEmail(nil, adminCardEmail)
	if err != nil {
		draft.Unlock()
		t.Fatal(err)
	}
	draft.receiverInboundIDs = ids
	draft.Unlock()
	adminWorkflowTap(tb, oldSave, adminTgID)
	if _, err := (&service.ClientPlanService{}).Resolve("B"); err == nil {
		t.Fatal("old save persisted a different plan draft")
	}
}
