package tgbot

import (
	"context"
	"errors"
	"html"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func parseRenewalArguments(args []string) (service.ClientRenewalSpec, error) {
	spec := service.ClientRenewalSpec{Mode: "replace"}
	if len(args) != 2 && len(args) != 3 {
		return spec, errors.New("expected quota, duration and optional extend")
	}
	if len(args) == 3 {
		if strings.ToLower(args[2]) != "extend" {
			return spec, errors.New("invalid renewal mode")
		}
		spec.Mode = "extend"
	}
	quota := strings.ToLower(args[0])
	days := strings.ToLower(args[1])
	if !strings.HasSuffix(quota, "gb") || !strings.HasSuffix(days, "d") {
		return spec, errors.New("use whole GB and days")
	}
	q, err := strconv.ParseUint(strings.TrimSuffix(quota, "gb"), 10, 64)
	if err != nil || q > math.MaxInt64/(1<<30) {
		return spec, errors.New("invalid quota")
	}
	d, err := strconv.ParseUint(strings.TrimSuffix(days, "d"), 10, 32)
	if err != nil || d > 36500 {
		return spec, errors.New("invalid duration")
	}
	spec.QuotaBytes, spec.Days = int64(q)*(1<<30), int(d)
	return spec, nil
}

func (t *Tgbot) workflowError(chatID int64, err error) {
	key := "tgbot.messages.workflowError"
	if errors.Is(err, service.ErrClientOperationChanged) {
		key = "tgbot.messages.workflowChanged"
	} else if errors.Is(err, service.ErrClientOperationPending) {
		key = "tgbot.messages.workflowPending"
	}
	message := t.I18nBot(key)
	if key != "tgbot.messages.workflowChanged" {
		detail := []rune(err.Error())
		if len(detail) > 240 {
			detail = append(detail[:240], '…')
		}
		message += "\n<code>" + html.EscapeString(string(detail)) + "</code>"
	}
	t.sendNotice(chatID, message)
}

func (t *Tgbot) handleAdminWorkflowCommand(message *telego.Message) bool {
	if message == nil {
		return false
	}
	command, _, args := tu.ParseCommand(message.Text)
	if command != "add" && command != "renew" && command != "plans" {
		return false
	}
	if message.From == nil || !privateAdmin(message.Chat, message.From.ID) {
		return true
	}
	chatID, owner := message.Chat.ID, message.From.ID
	if command == "plans" {
		t.screenClientPlans(chatID, "", 0)
		return true
	}
	svc := &service.ClientAdminWorkflowService{}
	var op *service.ClientAdminOperation
	var err error
	if command == "add" && len(args) == 2 {
		op, err = svc.PrepareCreate(owner, args[0], args[1])
	} else if command == "renew" && len(args) >= 2 {
		if len(args) == 2 {
			op, err = svc.PrepareRenewPlan(owner, args[0], args[1])
		} else {
			var spec service.ClientRenewalSpec
			spec, err = parseRenewalArguments(args[1:])
			if err == nil {
				op, err = svc.PrepareRenew(owner, args[0], spec)
			}
		}
	} else {
		t.sendNotice(chatID, t.I18nBot("tgbot.messages.workflowUsage"))
		return true
	}
	if err != nil {
		t.workflowError(chatID, err)
	} else {
		t.screenClientOperation(chatID, op)
	}
	return true
}

func (t *Tgbot) handleAdminWorkflowCallback(query *telego.CallbackQuery) bool {
	if query == nil {
		return false
	}
	decoded, err := t.decodeQuery(query.Data)
	if err != nil || !strings.HasPrefix(decoded, "admin_") {
		return false
	}
	if query.Message == nil || !privateAdmin(query.Message.GetChat(), query.From.ID) {
		t.sendCallbackAnswerTgBot(query.ID, t.I18nBot("tgbot.answers.errorOperation"))
		return true
	}
	actor := callbackActor(query)
	parts := strings.Fields(decoded)
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}
	t.answerSilent(query.ID)
	svc := &service.ClientAdminWorkflowService{}
	switch parts[0] {
	case "admin_plans":
		page, _ := strconv.Atoi(arg)
		t.screenClientPlans(actor.chatID, "", page)
	case "admin_renew":
		page := 0
		if len(parts) > 2 {
			page, _ = strconv.Atoi(parts[2])
		}
		if _, err := t.clientService.GetRecordByEmail(nil, arg); err != nil {
			t.workflowError(actor.chatID, err)
			return true
		}
		t.screenClientPlans(actor.chatID, arg, page)
	case "admin_plan_show":
		plan, err := (&service.ClientPlanService{}).Resolve(arg)
		if err != nil {
			t.workflowError(actor.chatID, err)
			return true
		}
		t.screenClientPlan(actor.chatID, *plan)
	case "admin_plan_new", "admin_plan_edit":
		var plan *service.ClientPlan
		if parts[0] == "admin_plan_edit" {
			plan, err = (&service.ClientPlanService{}).Resolve(arg)
			if err != nil {
				t.workflowError(actor.chatID, err)
				return true
			}
		}
		t.startPlanDraft(actor, plan)
	case "admin_plan_name", "admin_plan_mode", "admin_plan_firstuse", "admin_plan_enabled", "admin_plan_save":
		t.handlePlanDraftCallback(actor, parts[0], arg)
	case "admin_plan_cancel":
		addClientDrafts.reset(actor)
		t.screenClientPlans(actor.chatID, "", 0)
	case "admin_plan_delete", "admin_plan_delete_yes":
		plan, err := (&service.ClientPlanService{}).Resolve(arg)
		if err != nil {
			t.workflowError(actor.chatID, err)
			return true
		}
		if parts[0] == "admin_plan_delete" {
			t.renderScreen(actor.chatID, t.newScreen("client", t.I18nBot("tgbot.messages.deletePlanConfirm", "Name=="+html.EscapeString(plan.Name)),
				tu.InlineKeyboardRow(t.btn("tgbot.buttons.delete", t.encodeQuery("admin_plan_delete_yes "+plan.ID+" "+strconv.FormatInt(plan.Revision, 10)))), t.backRow()))
		} else {
			if len(parts) != 3 {
				return true
			}
			revision, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return true
			}
			if err := (&service.ClientPlanService{}).Delete(plan.ID, revision); err != nil {
				t.workflowError(actor.chatID, err)
			} else {
				t.screenClientPlans(actor.chatID, "", 0)
			}
		}
	case "admin_create_from":
		userStateMgr.set(actor, "admin:create:"+arg)
		t.renderScreen(actor.chatID, t.newScreen("wizard", t.I18nBot("tgbot.messages.quickCreateName"), t.backRow()))
	case "admin_renew_custom":
		userStateMgr.set(actor, "admin:renew:"+arg)
		t.renderScreen(actor.chatID, t.newScreen("client", t.I18nBot("tgbot.messages.quickRenewInput"), t.backRow()))
	case "admin_renew_plan":
		if len(parts) != 3 {
			return true
		}
		op, err := svc.PrepareRenewPlan(actor.userID, parts[2], arg)
		if err != nil {
			t.workflowError(actor.chatID, err)
		} else {
			t.screenClientOperation(actor.chatID, op)
		}
	case "admin_cancel":
		if err := svc.Cancel(arg, actor.userID); err != nil {
			t.workflowError(actor.chatID, err)
		} else {
			t.sendNotice(actor.chatID, t.I18nBot("tgbot.messages.cancel"))
		}
	case "admin_keepoff":
		op, err := svc.KeepDisabled(arg, actor.userID)
		if err != nil {
			t.workflowError(actor.chatID, err)
		} else {
			t.screenClientOperation(actor.chatID, op)
		}
	case "admin_confirm":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		op, err := svc.Confirm(ctx, arg, actor.userID)
		if err != nil {
			t.workflowError(actor.chatID, err)
		} else {
			t.searchClientScreen(actor.chatID, op.Email)
		}
	default:
		t.sendNotice(actor.chatID, t.I18nBot("tgbot.answers.errorOperation"))
	}
	return true
}

func (t *Tgbot) handleAdminWorkflowInput(message *telego.Message, actor chatUser, state string) bool {
	if !strings.HasPrefix(state, "admin:") {
		return false
	}
	if message == nil || message.From == nil || !privateAdmin(message.Chat, message.From.ID) || message.From.ID != actor.userID {
		return true
	}
	text := strings.TrimSpace(message.Text)
	svc := &service.ClientAdminWorkflowService{}
	var op *service.ClientAdminOperation
	var err error
	switch {
	case state == "admin:planname":
		draft := addClientDrafts.forActor(actor)
		draft.Lock()
		defer draft.Unlock()
		if draft.plan == nil {
			return true
		}
		if text == "" || strings.ContainsAny(text, " \r\n\t/\\") {
			t.sendNotice(actor.chatID, t.I18nBot("tgbot.messages.workflowError"))
			return true
		}
		draft.email = text
		userStateMgr.clear(actor)
		t.addClient(actor.chatID, draft, "")
		return true
	case strings.HasPrefix(state, "admin:create:"):
		op, err = svc.PrepareCreate(actor.userID, text, strings.TrimPrefix(state, "admin:create:"))
	case strings.HasPrefix(state, "admin:renew:"):
		args := strings.Fields(text)
		if len(args) == 1 {
			op, err = svc.PrepareRenewPlan(actor.userID, strings.TrimPrefix(state, "admin:renew:"), args[0])
		} else {
			var spec service.ClientRenewalSpec
			spec, err = parseRenewalArguments(args)
			if err == nil {
				op, err = svc.PrepareRenew(actor.userID, strings.TrimPrefix(state, "admin:renew:"), spec)
			}
		}
	default:
		return true
	}
	if err != nil {
		t.workflowError(actor.chatID, err)
	} else {
		userStateMgr.clear(actor)
		t.screenClientOperation(actor.chatID, op)
	}
	return true
}

func (t *Tgbot) quotaLabel(bytes int64) string {
	if bytes == 0 {
		return t.I18nBot("tgbot.unlimited")
	}
	return common.FormatTraffic(bytes)
}

func (t *Tgbot) enabledLabel(enabled bool) string {
	if enabled {
		return t.I18nBot("tgbot.messages.yes")
	}
	return t.I18nBot("tgbot.messages.no")
}

func (t *Tgbot) renewalModeLabel(mode string) string {
	if mode == "extend" {
		return t.I18nBot("tgbot.messages.renewExtend")
	}
	return t.I18nBot("tgbot.messages.renewReplace")
}

func inboundIDLabels(ids []int) string {
	labels := make([]string, len(ids))
	for i, id := range ids {
		labels[i] = "#" + strconv.Itoa(id)
	}
	return strings.Join(labels, ", ")
}

func (t *Tgbot) screenClientOperation(chatID int64, op *service.ClientAdminOperation) {
	p := op.Preview
	mode := t.renewalModeLabel(op.Spec.Mode)
	if op.Kind == "create" {
		mode = t.I18nBot("tgbot.messages.quickCreate")
	}
	body := t.I18nBot("tgbot.messages.operationPreview",
		"Email=="+html.EscapeString(op.Email), "Mode=="+mode, "OldQuota=="+t.quotaLabel(p.OldQuotaBytes), "Used=="+common.FormatTraffic(p.UsedBytes),
		"NewQuota=="+t.quotaLabel(p.NewQuotaBytes), "Reset=="+t.enabledLabel(p.ResetUsage), "OldExpiry=="+t.expiryLabel(p.OldExpiry), "NewExpiry=="+t.expiryLabel(p.NewExpiry),
		"OldEnabled=="+t.enabledLabel(p.EnabledBefore), "NewEnabled=="+t.enabledLabel(p.EnabledAfter), "Inbounds=="+inboundIDLabels(p.InboundIDs), "LimitIP=="+strconv.Itoa(p.LimitIP))
	rows := [][]telego.InlineKeyboardButton{tu.InlineKeyboardRow(t.btn("tgbot.buttons.confirm", "admin_confirm "+op.Token), t.btn("tgbot.buttons.cancel", "admin_cancel "+op.Token))}
	if op.Kind == "renew" && !p.EnabledBefore && p.EnabledAfter {
		rows = append(rows, tu.InlineKeyboardRow(t.btn("tgbot.buttons.keepDisabled", "admin_keepoff "+op.Token)))
	}
	rows = append(rows, t.backRow())
	t.renderScreen(chatID, t.newScreen("client", body, rows...))
}

func (t *Tgbot) screenClientPlans(chatID int64, email string, page int) {
	plans, err := (&service.ClientPlanService{}).List()
	if err != nil {
		t.workflowError(chatID, err)
		return
	}
	if page < 0 {
		page = 0
	}
	if page > (len(plans)-1)/8 {
		page = 0
	}
	var rows [][]telego.InlineKeyboardButton
	for _, plan := range plans[page*8 : min(page*8+8, len(plans))] {
		data := "admin_plan_show " + plan.ID
		if email != "" {
			data = "admin_renew_plan " + plan.ID + " " + email
		}
		rows = append(rows, tu.InlineKeyboardRow(t.btn2(plan.Name, t.encodeQuery(data))))
	}
	base := "admin_plans"
	if email != "" {
		base = "admin_renew " + email
	}
	var nav []telego.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, t.btn("tgbot.buttons.prev", t.encodeQuery(base+" "+strconv.Itoa(page-1))))
	}
	if (page+1)*8 < len(plans) {
		nav = append(nav, t.btn("tgbot.buttons.next", t.encodeQuery(base+" "+strconv.Itoa(page+1))))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	body := t.I18nBot("tgbot.messages.clientPlans")
	if email == "" {
		rows = append(rows, tu.InlineKeyboardRow(t.btn("tgbot.buttons.newPlan", "admin_plan_new")))
	} else {
		body = t.clientScreenBody(email, t.I18nBot("tgbot.buttons.renewClient")) + t.I18nBot("tgbot.messages.renewPlanScope")
		rows = append(rows, tu.InlineKeyboardRow(t.btn("tgbot.buttons.custom", t.encodeQuery("admin_renew_custom "+email))))
	}
	rows = append(rows, t.backRow())
	t.renderScreen(chatID, t.newScreen("client", body, rows...))
}

func (t *Tgbot) planBody(plan service.ClientPlan) string {
	return t.I18nBot("tgbot.messages.planDetails", "Name=="+html.EscapeString(plan.Name), "Quota=="+t.quotaLabel(plan.QuotaBytes), "Days=="+strconv.Itoa(plan.Days),
		"FirstUse=="+t.enabledLabel(plan.FirstUse), "Enabled=="+t.enabledLabel(plan.Enabled), "LimitIP=="+strconv.Itoa(plan.LimitIP), "Inbounds=="+inboundIDLabels(plan.InboundIDs), "Mode=="+t.renewalModeLabel(plan.RenewalMode))
}

func (t *Tgbot) screenClientPlan(chatID int64, plan service.ClientPlan) {
	t.renderScreen(chatID, t.newScreen("client", t.planBody(plan),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.usePlan", "admin_create_from "+plan.ID)),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.editPlan", "admin_plan_edit "+plan.ID), t.btn("tgbot.buttons.deletePlan", "admin_plan_delete "+plan.ID)),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.clientPlans", "admin_plans")), t.backRow()))
}

func (t *Tgbot) startPlanDraft(actor chatUser, plan *service.ClientPlan) {
	draft := addClientDrafts.forActor(actor)
	draft.Lock()
	defer draft.Unlock()
	t.resetDraft(draft)
	if plan == nil {
		plan = &service.ClientPlan{Name: "Plan_" + t.randomLowerAndNum(4), QuotaBytes: 30 << 30, Days: 30, Enabled: true, RenewalMode: "replace"}
	}
	copy := *plan
	copy.InboundIDs = slices.Clone(plan.InboundIDs)
	draft.plan = &copy
	draft.planSession = t.randomLowerAndNum(16)
	draft.email = plan.Name
	draft.totalGB = plan.QuotaBytes
	draft.expiryTime = -int64(plan.Days) * 86400000
	draft.limitIP = plan.LimitIP
	draft.enable = plan.Enabled
	draft.receiverInboundIDs = slices.Clone(plan.InboundIDs)
	draft.receiverInboundID = 0
	if len(draft.receiverInboundIDs) > 0 {
		draft.receiverInboundID = draft.receiverInboundIDs[0]
	}
	t.addClient(actor.chatID, draft, "")
}

func draftClientPlan(draft *clientDraft) service.ClientPlan {
	plan := *draft.plan
	plan.Name = draft.email
	plan.QuotaBytes = draft.totalGB
	plan.LimitIP = draft.limitIP
	plan.Enabled = draft.enable
	plan.Days = int(-draft.expiryTime / 86400000)
	plan.InboundIDs = slices.Clone(draft.receiverInboundIDs)
	return plan
}

func (t *Tgbot) planDraftBody(draft *clientDraft) string { return t.planBody(draftClientPlan(draft)) }

func (t *Tgbot) planDraftRows(draft *clientDraft) [][]telego.InlineKeyboardButton {
	return [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.planName", "admin_plan_name")),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.limitTraffic", "add_client_ch_default_traffic"), t.btn("tgbot.buttons.resetExpire", "add_client_ch_default_exp")),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.ipLimit", "add_client_ch_default_ip_limit"), t.btn("tgbot.buttons.planEnabled", "admin_plan_enabled")),
		tu.InlineKeyboardRow(t.btn2(t.I18nBot("tgbot.buttons.attachInbound", "Count=="+strconv.Itoa(len(draft.receiverInboundIDs))), "add_client_attach_more")),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.planFirstUse", "admin_plan_firstuse"), t.btn("tgbot.buttons.planRenewalMode", "admin_plan_mode")),
		tu.InlineKeyboardRow(t.btn("tgbot.buttons.savePlan", "admin_plan_save "+draft.planSession), t.btn("tgbot.buttons.cancel", "admin_plan_cancel")),
	}
}

func (t *Tgbot) handlePlanDraftCallback(actor chatUser, action, session string) {
	draft := addClientDrafts.forActor(actor)
	draft.Lock()
	defer draft.Unlock()
	if draft.plan == nil {
		t.sendNotice(actor.chatID, t.I18nBot("tgbot.messages.workflowChanged"))
		return
	}
	if action == "admin_plan_save" && (session == "" || session != draft.planSession) {
		t.sendNotice(actor.chatID, t.I18nBot("tgbot.messages.workflowChanged"))
		return
	}
	switch action {
	case "admin_plan_name":
		userStateMgr.set(actor, "admin:planname")
		t.wizardPrompt(actor.chatID, draft, t.I18nBot("tgbot.messages.planNamePrompt"))
		return
	case "admin_plan_mode":
		if draft.plan.RenewalMode == "replace" {
			draft.plan.RenewalMode = "extend"
		} else {
			draft.plan.RenewalMode = "replace"
		}
	case "admin_plan_firstuse":
		draft.plan.FirstUse = !draft.plan.FirstUse
	case "admin_plan_enabled":
		draft.enable = !draft.enable
	case "admin_plan_save":
		plan, err := (&service.ClientPlanService{}).Save(draftClientPlan(draft))
		if err != nil {
			t.workflowError(actor.chatID, err)
			return
		}
		addClientDrafts.reset(actor)
		t.screenClientPlan(actor.chatID, *plan)
		return
	}
	t.addClient(actor.chatID, draft, "")
}
