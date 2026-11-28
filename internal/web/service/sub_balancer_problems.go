package service

import (
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// SubBalancerProblem codes. They are part of the API contract (the panel UI
// maps them to i18n keys), so treat them as stable identifiers.
const (
	// An import or a restored backup can bring back the id of a deleted inbound.
	SubBalancerProblemUnknownInbound  = "unknown_inbound"
	SubBalancerProblemDisabledInbound = "disabled_inbound"
	// The protocol renders share links and Clash proxies but has no JSON outbound.
	SubBalancerProblemJSONIncapable = "json_incapable_protocol"
	// No selected inbound can contribute, so the JSON subscription omits the balancer.
	SubBalancerProblemNoMembers = "no_members"
	// Enabled and renderable, but with no client it reaches no subscription.
	SubBalancerProblemInboundWithoutClients = "inbound_without_clients"
	// Every enabled host excludes the JSON format, so hostEndpoints closes the inbound.
	SubBalancerProblemHostsExcluded = "hosts_excluded_from_json"
)

// SubBalancerProblem is one reason a balancer cannot emit what an operator expects.
// Advisories never change the emitted config, and per-client losses are out of scope.
type SubBalancerProblem struct {
	Code       string `json:"code" example:"unknown_inbound"`
	InboundIds []int  `json:"inboundIds,omitempty" example:"[7,9]"`
	Detail     string `json:"detail,omitempty" example:"amneziawg"`
}

// jsonIncapableProtocols mirrors the Protocol switch in internal/sub/json_service.go;
// sub_balancer_problems_parity_test.go fails when the two drift.
var jsonIncapableProtocols = map[string]struct{}{
	"amneziawg": {},
	"tuic":      {},
	"mtproto":   {},
}

// SubBalancerWithProblems embeds the row by value so it marshals flat; Problems is
// always non-nil so the response shape is stable.
type SubBalancerWithProblems struct {
	model.SubBalancer
	Problems []SubBalancerProblem `json:"problems"`
}

// ListWithProblems returns every balancer with its advisories attached.
func (s *SubBalancerService) ListWithProblems() ([]*SubBalancerWithProblems, error) {
	balancers, err := s.List()
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(balancers))
	for _, b := range balancers {
		ids = append(ids, b.InboundIds...)
	}
	byID, clientCounts, hostsClosed, err := loadBalancerMemberInbounds(ids)
	if err != nil {
		return nil, err
	}
	out := make([]*SubBalancerWithProblems, 0, len(balancers))
	for _, b := range balancers {
		problems := diagnoseSubBalancer(b, byID, clientCounts, hostsClosed)
		out = append(out, &SubBalancerWithProblems{SubBalancer: *b, Problems: problems})
	}
	return out, nil
}

// loadBalancerMemberInbounds reads only the columns the diagnosis needs, plus
// grouped client and host counts that decide reachability per inbound.
func loadBalancerMemberInbounds(ids []int) (map[int]*model.Inbound, map[int]int, map[int]bool, error) {
	byID := map[int]*model.Inbound{}
	clients := map[int]int{}
	hostsClosed := map[int]bool{}
	unique := make([]int, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(unique, id) {
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return byID, clients, hostsClosed, nil
	}
	var inbounds []*model.Inbound
	err := database.GetDB().Model(&model.Inbound{}).
		Select("id", "protocol", "enable").
		Where("id IN ?", unique).Find(&inbounds).Error
	if err != nil {
		return nil, nil, nil, err
	}
	for _, ib := range inbounds {
		byID[ib.Id] = ib
	}

	var counts []struct {
		InboundId int
		Total     int
	}
	err = database.GetDB().Model(&model.ClientInbound{}).
		Select("inbound_id, COUNT(*) AS total").
		Where("inbound_id IN ?", unique).
		Group("inbound_id").Scan(&counts).Error
	if err != nil {
		return nil, nil, nil, err
	}
	for _, c := range counts {
		clients[c.InboundId] = c.Total
	}

	// An inbound whose enabled hosts all exclude JSON is closed by hostEndpoints,
	// so its clients never render. The column is a JSON array, hence the quoted match.
	var hostRows []struct {
		InboundId          int
		Enabled            int
		ExcludingJSONCount int
	}
	err = database.GetDB().Model(&model.Host{}).
		Select(`inbound_id, COUNT(*) AS enabled, `+
			`SUM(CASE WHEN exclude_from_sub_types LIKE '%"json"%' THEN 1 ELSE 0 END) AS excluding_json_count`).
		Where("inbound_id IN ? AND is_disabled = ?", unique, false).
		Group("inbound_id").Scan(&hostRows).Error
	if err != nil {
		return nil, nil, nil, err
	}
	for _, h := range hostRows {
		if h.Enabled > 0 && h.ExcludingJSONCount == h.Enabled {
			hostsClosed[h.InboundId] = true
		}
	}
	return byID, clients, hostsClosed, nil
}

// diagnoseSubBalancer groups per-inbound problems by code in selection order,
// then appends no_members when nothing could be emitted.
func diagnoseSubBalancer(b *model.SubBalancer, byID map[int]*model.Inbound, clientCounts map[int]int, hostsClosed map[int]bool) []SubBalancerProblem {
	unknown := []int{}
	disabled := []int{}
	incapable := []int{}
	empty := []int{}
	closedHosts := []int{}
	incapableDetail := ""
	contributing := 0
	for _, id := range b.InboundIds {
		ib, ok := byID[id]
		if !ok {
			unknown = append(unknown, id)
			continue
		}
		if !ib.Enable {
			disabled = append(disabled, id)
			continue
		}
		if _, bad := jsonIncapableProtocols[string(ib.Protocol)]; bad {
			incapable = append(incapable, id)
			if incapableDetail == "" {
				incapableDetail = string(ib.Protocol)
			}
			continue
		}
		if clientCounts[id] == 0 {
			// Renderable but unreachable: with no client on it the inbound never
			// enters any subscription, so the balancer drops out for everyone.
			empty = append(empty, id)
			continue
		}
		if hostsClosed[id] {
			// Every enabled host excludes the JSON format, so hostEndpoints closes
			// the inbound and none of its clients render.
			closedHosts = append(closedHosts, id)
			continue
		}
		contributing++
	}
	problems := make([]SubBalancerProblem, 0, 6)
	if len(unknown) > 0 {
		problems = append(problems, SubBalancerProblem{Code: SubBalancerProblemUnknownInbound, InboundIds: unknown})
	}
	if len(disabled) > 0 {
		problems = append(problems, SubBalancerProblem{Code: SubBalancerProblemDisabledInbound, InboundIds: disabled})
	}
	if len(incapable) > 0 {
		problems = append(problems, SubBalancerProblem{Code: SubBalancerProblemJSONIncapable, InboundIds: incapable, Detail: incapableDetail})
	}
	if len(empty) > 0 {
		problems = append(problems, SubBalancerProblem{Code: SubBalancerProblemInboundWithoutClients, InboundIds: empty})
	}
	if len(closedHosts) > 0 {
		problems = append(problems, SubBalancerProblem{Code: SubBalancerProblemHostsExcluded, InboundIds: closedHosts})
	}
	if contributing == 0 {
		problems = append(problems, SubBalancerProblem{Code: SubBalancerProblemNoMembers})
	}
	return problems
}

// loggedProblems remembers the last reported fingerprint per balancer, so the
// read path warns when something changes instead of on every listing.
var loggedProblems = struct {
	sync.Mutex
	byID map[int]string
}{byID: map[int]string{}}

// LogSubBalancerProblems warns when a balancer's problems change. Identical listings
// stay silent, and a resolution is reported only when a problem was known before.
func LogSubBalancerProblems(b *model.SubBalancer, problems []SubBalancerProblem) {
	fingerprint := problemsFingerprint(problems)

	loggedProblems.Lock()
	previous, seen := loggedProblems.byID[b.Id]
	if fingerprint == "" {
		delete(loggedProblems.byID, b.Id)
	} else {
		loggedProblems.byID[b.Id] = fingerprint
	}
	loggedProblems.Unlock()

	if fingerprint == "" {
		// Nothing to report for a balancer that never had a problem; a known one
		// going quiet is a real transition worth one line.
		if seen {
			logger.Noticef("[SubBalancer] balancer %q (#%d) no longer has problems", b.Remark, b.Id)
		}
		return
	}
	if seen && previous == fingerprint {
		return
	}
	logger.Warningf("[SubBalancer] balancer %q (#%d) has %d problem(s), inboundIds=%v: %+v",
		b.Remark, b.Id, len(problems), b.InboundIds, problems)
}

// problemsFingerprint renders the advisories deterministically; empty means
// healthy, which also clears the stored entry.
func problemsFingerprint(problems []SubBalancerProblem) string {
	if len(problems) == 0 {
		return ""
	}
	parts := make([]string, 0, len(problems))
	for _, p := range problems {
		ids := make([]string, 0, len(p.InboundIds))
		for _, id := range p.InboundIds {
			ids = append(ids, strconv.Itoa(id))
		}
		parts = append(parts, p.Code+"["+strings.Join(ids, ",")+"]"+p.Detail)
	}
	return strings.Join(parts, ";")
}
