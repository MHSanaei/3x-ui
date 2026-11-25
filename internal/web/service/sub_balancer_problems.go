package service

import (
	"slices"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// SubBalancerProblem codes. They are part of the API contract (the panel UI
// maps them to i18n keys), so treat them as stable identifiers.
const (
	// SubBalancerProblemUnknownInbound: InboundIds names an inbound that no
	// longer exists. Deletes clean these up, but an import or a restored
	// backup can bring the dangling id back.
	SubBalancerProblemUnknownInbound = "unknown_inbound"
	// SubBalancerProblemDisabledInbound: the inbound exists but is disabled.
	SubBalancerProblemDisabledInbound = "disabled_inbound"
	// SubBalancerProblemJSONIncapable: the protocol renders share links and
	// Clash proxies but has no JSON outbound, so it can never be a member.
	SubBalancerProblemJSONIncapable = "json_incapable_protocol"
	// SubBalancerProblemNoMembers: none of the selected inbounds can contribute
	// a member, so the balancer is omitted from the JSON subscription.
	SubBalancerProblemNoMembers = "no_members"
)

// SubBalancerProblem reports one reason a balancer cannot emit the members an
// operator expects. InboundIds is set for per-inbound codes and empty for
// balancer-wide ones (no_members). Advisories never change what is emitted —
// the runtime still omits a balancer with no resolvable intersection.
//
// Known blind spot by design: a member can also vanish for reasons that belong to
// one client, not to the row — a WireGuard client with no private key renders no
// outbound, and the render budget can cut the tail of a large subscription. Those
// are per-subscriber and cannot be attributed to a stored balancer.
type SubBalancerProblem struct {
	Code       string `json:"code" example:"unknown_inbound"`
	InboundIds []int  `json:"inboundIds,omitempty" example:"[7,9]"`
	Detail     string `json:"detail,omitempty" example:"amneziawg"`
}

// jsonIncapableProtocols mirrors the switch in internal/sub/json_service.go:
// these protocols fall through to `continue` and never produce an outbound.
// Keep it in lockstep with that switch — sub/json_capable_parity_test.go fails
// when the two lists drift.
var jsonIncapableProtocols = map[string]struct{}{
	"amneziawg": {},
	"tuic":      {},
	"mtproto":   {},
}

// SubBalancerWithProblems is the list-API shape: the stored row plus the
// cosmetic advisories an operator needs to see. The row is embedded by value so
// it marshals flat (the panel reads the same fields it always did); Problems is
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
	byID, err := loadBalancerMemberInbounds(ids)
	if err != nil {
		return nil, err
	}
	out := make([]*SubBalancerWithProblems, 0, len(balancers))
	for _, b := range balancers {
		problems := diagnoseSubBalancer(b, byID)
		out = append(out, &SubBalancerWithProblems{SubBalancer: *b, Problems: problems})
	}
	return out, nil
}

// loadBalancerMemberInbounds fetches only the columns the diagnosis needs;
// settings blobs would be decoded for nothing.
func loadBalancerMemberInbounds(ids []int) (map[int]*model.Inbound, error) {
	byID := map[int]*model.Inbound{}
	unique := make([]int, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(unique, id) {
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return byID, nil
	}
	var inbounds []*model.Inbound
	err := database.GetDB().Model(&model.Inbound{}).
		Select("id", "protocol", "enable").
		Where("id IN ?", unique).Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	for _, ib := range inbounds {
		byID[ib.Id] = ib
	}
	return byID, nil
}

// diagnoseSubBalancer groups per-inbound problems by code in selection order,
// then appends no_members when nothing could be emitted. Disabled inbounds the
// subscription hides entirely still resolve when every client is enabled, so
// disabling is advisory; a hidden inbound is never advisory on its own.
func diagnoseSubBalancer(b *model.SubBalancer, byID map[int]*model.Inbound) []SubBalancerProblem {
	unknown := []int{}
	disabled := []int{}
	incapable := []int{}
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
		contributing++
	}
	problems := make([]SubBalancerProblem, 0, 4)
	if len(unknown) > 0 {
		problems = append(problems, SubBalancerProblem{Code: SubBalancerProblemUnknownInbound, InboundIds: unknown})
	}
	if len(disabled) > 0 {
		problems = append(problems, SubBalancerProblem{Code: SubBalancerProblemDisabledInbound, InboundIds: disabled})
	}
	if len(incapable) > 0 {
		problems = append(problems, SubBalancerProblem{Code: SubBalancerProblemJSONIncapable, InboundIds: incapable, Detail: incapableDetail})
	}
	if contributing == 0 {
		problems = append(problems, SubBalancerProblem{Code: SubBalancerProblemNoMembers})
	}
	return problems
}

// LogSubBalancerProblems warns once per balancer about anything an operator
// would otherwise discover only by diffing a client's config.
func LogSubBalancerProblems(b *model.SubBalancer, problems []SubBalancerProblem) {
	if len(problems) == 0 {
		return
	}
	logger.Warningf("[SubBalancer] balancer %q (#%d) has %d problem(s), inboundIds=%v: %+v",
		b.Remark, b.Id, len(problems), b.InboundIds, problems)
}
