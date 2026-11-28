package service

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// seedDiagInbound also links one client: a client-less inbound is its own advisory,
// separately tested, and must not leak into tests aimed at other codes.
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
	client := &model.ClientRecord{
		Email: fmt.Sprintf("diag-%d@test", id), SubID: fmt.Sprintf("diag%d", id), Enable: true,
	}
	if err := database.GetDB().Create(client).Error; err != nil {
		t.Fatalf("seed client %d: %v", id, err)
	}
	if err := database.GetDB().Create(&model.ClientInbound{
		ClientId: client.Id, InboundId: id,
	}).Error; err != nil {
		t.Fatalf("link client %d: %v", id, err)
	}
}

// seedDiagInboundWithoutClients inserts only the inbound row, leaving
// client_inbounds empty for it.
func seedDiagInboundWithoutClients(t *testing.T, id int, protocol model.Protocol) {
	t.Helper()
	ib := &model.Inbound{
		Id: id, UserId: 1, Tag: fmt.Sprintf("diag-%d", id), Enable: true,
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

// Mixed healthy, dangling, disabled and JSON-incapable inbounds report one problem
// per class in a stable order, and no no_members while one member survives.
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
	// Inbound 1 has a client, so it still contributes and no_members stays away.
	clientCounts := map[int]int{1: 3}
	bal := &model.SubBalancer{Id: 7, Remark: "pure", InboundIds: []int{42, 2, 1, 41}}
	problems := diagnoseSubBalancer(bal, byID, clientCounts, nil)
	if diff := wantCodes(problemCodes(problems), SubBalancerProblemUnknownInbound, SubBalancerProblemDisabledInbound); diff != "" {
		t.Fatal(diff)
	}
	if ids := problems[0].InboundIds; len(ids) != 2 || ids[0] != 42 || ids[1] != 41 {
		t.Fatalf("unknown ids = %v, want [42 41] in selection order", ids)
	}

	allDisabled := &model.SubBalancer{Id: 8, Remark: "off", InboundIds: []int{2}}
	problems = diagnoseSubBalancer(allDisabled, byID, clientCounts, nil)
	if diff := wantCodes(problemCodes(problems), SubBalancerProblemDisabledInbound, SubBalancerProblemNoMembers); diff != "" {
		t.Fatal(diff)
	}
}

// A renderable inbound nobody sits on reaches no subscription, so the balancer
// drops out for everyone — the silence this feature exists to explain.
func TestSubBalancerProblemsInboundWithoutClients(t *testing.T) {
	setupSubBalancerDB(t)
	// Seeding only the inbound leaves client_inbounds empty.
	seedDiagInboundWithoutClients(t, 1, model.VLESS)

	svc := &SubBalancerService{}
	if _, err := svc.Create(&model.SubBalancer{
		Remark: "empty", Strategy: "random", InboundIds: []int{1}, SortOrder: 1, Enabled: true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := svc.ListWithProblems()
	if err != nil {
		t.Fatalf("ListWithProblems: %v", err)
	}
	got := problemCodes(list[0].Problems)
	if diff := wantCodes(got, SubBalancerProblemInboundWithoutClients, SubBalancerProblemNoMembers); diff != "" {
		t.Fatal(diff)
	}
}

// The log is the contract, not the fingerprint map: assert on what was written.
func TestLogSubBalancerProblemsDeduplicates(t *testing.T) {
	xuilogger.InitLogger(logging.DEBUG)
	bal := &model.SubBalancer{Id: 9001, Remark: "dedup-9001", InboundIds: []int{5}}
	problem := []SubBalancerProblem{{Code: SubBalancerProblemUnknownInbound, InboundIds: []int{5}}}

	before := len(logLines())
	LogSubBalancerProblems(bal, problem)
	if got := logLinesSince(before); countContaining(got, "dedup-9001") != 1 {
		t.Fatalf("first listing: want exactly one line, got %v", got)
	}

	// Same problem set again: the journal must not repeat itself.
	before = len(logLines())
	LogSubBalancerProblems(bal, problem)
	if got := logLinesSince(before); countContaining(got, "dedup-9001") != 0 {
		t.Fatalf("repeat listing: want no line, got %v", got)
	}

	// A known problem going quiet is a real transition: exactly one notice.
	before = len(logLines())
	LogSubBalancerProblems(bal, nil)
	lines := logLinesSince(before)
	if countContaining(lines, "no longer has problems") != 1 {
		t.Fatalf("resolution: want one notice, got %v", lines)
	}

	// A balancer that never had a problem must stay silent on every listing.
	healthy := &model.SubBalancer{Id: 9002, Remark: "healthy-9002", InboundIds: []int{6}}
	for i := 0; i < 3; i++ {
		before = len(logLines())
		LogSubBalancerProblems(healthy, nil)
		if got := logLinesSince(before); countContaining(got, "healthy-9002") != 0 {
			t.Fatalf("healthy listing %d: want silence, got %v", i, got)
		}
	}
}

// GetLogs returns newest first and caps at c, so read a large window and take a
// snapshot before each call instead of trusting absolute indices.
func logLines() []string { return xuilogger.GetLogs(1_000_000, "DEBUG") }

func logLinesSince(n int) []string {
	all := logLines()
	if len(all) <= n {
		return nil
	}
	return all[:len(all)-n]
}

func countContaining(lines []string, substr string) int {
	count := 0
	for _, line := range lines {
		if strings.Contains(line, substr) {
			count++
		}
	}
	return count
}

// The map is shared across concurrent listings (settings tab + refetches).
func TestLogSubBalancerProblemsConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			bal := &model.SubBalancer{Id: 9100 + id, Remark: "race", InboundIds: []int{id}}
			problems := []SubBalancerProblem{
				{Code: SubBalancerProblemUnknownInbound, InboundIds: []int{id}},
			}
			LogSubBalancerProblems(bal, problems)
		}(i)
	}
	wg.Wait()
}

// An inbound whose every enabled host excludes the JSON format is closed by
// hostEndpoints: its clients never render, so the balancer emits nothing.
func TestSubBalancerProblemsAllHostsExcludedFromJSON(t *testing.T) {
	setupSubBalancerDB(t)
	seedDiagInbound(t, 1, model.VLESS, true) // also links one client
	for i, types := range [][]string{{"json"}, {"json", "clash"}} {
		host := &model.Host{
			InboundId: 1, Remark: fmt.Sprintf("host-%d", i),
			Address: "cdn.example.com", Port: 443, ExcludeFromSubTypes: types,
		}
		if err := database.GetDB().Create(host).Error; err != nil {
			t.Fatalf("seed host %d: %v", i, err)
		}
	}

	svc := &SubBalancerService{}
	if _, err := svc.Create(&model.SubBalancer{
		Remark: "closed", Strategy: "random", InboundIds: []int{1}, SortOrder: 1, Enabled: true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := svc.ListWithProblems()
	if err != nil {
		t.Fatalf("ListWithProblems: %v", err)
	}
	got := problemCodes(list[0].Problems)
	if diff := wantCodes(got, SubBalancerProblemHostsExcluded, SubBalancerProblemNoMembers); diff != "" {
		t.Fatal(diff)
	}
}

// One host still serving JSON keeps the inbound usable, so no advisory appears.
func TestSubBalancerProblemsPartiallyExcludedHostsAreHealthy(t *testing.T) {
	setupSubBalancerDB(t)
	seedDiagInbound(t, 1, model.VLESS, true)
	hosts := []*model.Host{
		{InboundId: 1, Remark: "closed", Address: "a.example.com", Port: 443, ExcludeFromSubTypes: []string{"json"}},
		{InboundId: 1, Remark: "open", Address: "b.example.com", Port: 443},
	}
	for _, h := range hosts {
		if err := database.GetDB().Create(h).Error; err != nil {
			t.Fatalf("seed host: %v", err)
		}
	}

	svc := &SubBalancerService{}
	if _, err := svc.Create(&model.SubBalancer{
		Remark: "partial", Strategy: "random", InboundIds: []int{1}, SortOrder: 1, Enabled: true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := svc.ListWithProblems()
	if err != nil {
		t.Fatalf("ListWithProblems: %v", err)
	}
	if len(list[0].Problems) != 0 {
		t.Fatalf("Problems = %v, want none", problemCodes(list[0].Problems))
	}
}
