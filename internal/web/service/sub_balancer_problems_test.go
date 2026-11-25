package service

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// seedDiagInbound inserts one inbound with the columns the diagnosis reads.
func seedDiagInbound(t *testing.T, id int, protocol model.Protocol, enable bool) {
	t.Helper()
	ib := &model.Inbound{
		Id: id, UserId: 1, Tag: fmt.Sprintf("diag-%d", id), Enable: enable,
		Listen: "203.0.113.9", Port: 4000 + id, Protocol: protocol,
		Remark: fmt.Sprintf("diag-%d", id), Settings: `{}`, StreamSettings: `{}`,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed inbound %d: %v", id, err)
	}
}

// problemCodes flattens the codes so table assertions stay readable.
func problemCodes(problems []SubBalancerProblem) []string {
	codes := make([]string, 0, len(problems))
	for _, p := range problems {
		codes = append(codes, p.Code)
	}
	return codes
}

func wantCodes(got []string, want ...string) string {
	if len(got) != len(want) {
		return fmt.Sprintf("codes = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			return fmt.Sprintf("codes = %v, want %v", got, want)
		}
	}
	return ""
}

// A balancer selecting a mix of healthy, dangling, disabled and JSON-incapable
// inbounds reports one problem per class, in a stable order, and is not
// flagged no_members because one member survives.
func TestSubBalancerProblemsMixedClasses(t *testing.T) {
	setupSubBalancerDB(t)
	seedDiagInbound(t, 1, model.VLESS, true)
	seedDiagInbound(t, 2, model.VLESS, false)
	seedDiagInbound(t, 3, model.AmneziaWG, true)

	svc := &SubBalancerService{}
	if _, err := svc.Create(&model.SubBalancer{
		Remark: "mixed", Strategy: "random", InboundIds: []int{1, 2, 3, 99}, SortOrder: 1, Enabled: true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := svc.ListWithProblems()
	if err != nil {
		t.Fatalf("ListWithProblems: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	got := problemCodes(list[0].Problems)
	if diff := wantCodes(got, SubBalancerProblemUnknownInbound, SubBalancerProblemDisabledInbound, SubBalancerProblemJSONIncapable); diff != "" {
		t.Fatal(diff)
	}
	if ids := list[0].Problems[0].InboundIds; len(ids) != 1 || ids[0] != 99 {
		t.Fatalf("unknown ids = %v, want [99]", ids)
	}
	if ids := list[0].Problems[1].InboundIds; len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("disabled ids = %v, want [2]", ids)
	}
	if detail := list[0].Problems[2].Detail; detail != "amneziawg" {
		t.Fatalf("detail = %q, want amneziawg", detail)
	}
}

// Every selected inbound unusable means the balancer is omitted at render
// time; the advisory says so once, without per-inbound codes meaning nothing.
func TestSubBalancerProblemsNoMembers(t *testing.T) {
	setupSubBalancerDB(t)
	seedDiagInbound(t, 1, model.TUIC, true)

	svc := &SubBalancerService{}
	if _, err := svc.Create(&model.SubBalancer{
		Remark: "dead", Strategy: "random", InboundIds: []int{1}, SortOrder: 1, Enabled: true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := svc.ListWithProblems()
	if err != nil {
		t.Fatalf("ListWithProblems: %v", err)
	}
	got := problemCodes(list[0].Problems)
	if diff := wantCodes(got, SubBalancerProblemJSONIncapable, SubBalancerProblemNoMembers); diff != "" {
		t.Fatal(diff)
	}
}

// A purely healthy balancer carries an empty, non-nil slice so the JSON shape
// stays "problems": [] rather than dropping the field.
func TestSubBalancerProblemsHealthyIsEmpty(t *testing.T) {
	setupSubBalancerDB(t)
	seedDiagInbound(t, 1, model.VLESS, true)

	svc := &SubBalancerService{}
	if _, err := svc.Create(&model.SubBalancer{
		Remark: "fine", Strategy: "random", InboundIds: []int{1}, SortOrder: 1, Enabled: true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := svc.ListWithProblems()
	if err != nil {
		t.Fatalf("ListWithProblems: %v", err)
	}
	if list[0].Problems == nil {
		t.Fatal("Problems = nil, want non-nil empty slice")
	}
	if len(list[0].Problems) != 0 {
		t.Fatalf("Problems = %v, want empty", list[0].Problems)
	}
}

// diagnoseSubBalancer is pure; unknown ids keep selection order and no_members
// is never reported while one member contributes.
func TestDiagnoseSubBalancerOrderAndNoMembers(t *testing.T) {
	byID := map[int]*model.Inbound{
		1: {Id: 1, Protocol: model.VLESS, Enable: true},
		2: {Id: 2, Protocol: model.VLESS, Enable: false},
	}
	bal := &model.SubBalancer{Id: 7, Remark: "pure", InboundIds: []int{42, 2, 1, 41}}
	problems := diagnoseSubBalancer(bal, byID)
	if diff := wantCodes(problemCodes(problems), SubBalancerProblemUnknownInbound, SubBalancerProblemDisabledInbound); diff != "" {
		t.Fatal(diff)
	}
	if ids := problems[0].InboundIds; len(ids) != 2 || ids[0] != 42 || ids[1] != 41 {
		t.Fatalf("unknown ids = %v, want [42 41] in selection order", ids)
	}

	allDisabled := &model.SubBalancer{Id: 8, Remark: "off", InboundIds: []int{2}}
	problems = diagnoseSubBalancer(allDisabled, byID)
	if diff := wantCodes(problemCodes(problems), SubBalancerProblemDisabledInbound, SubBalancerProblemNoMembers); diff != "" {
		t.Fatal(diff)
	}
}
