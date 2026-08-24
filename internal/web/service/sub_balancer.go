package service

import (
	"slices"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

var subBalancerStrategies = map[string]struct{}{
	"leastLoad":  {},
	"leastPing":  {},
	"random":     {},
	"roundRobin": {},
}

// SubBalancerService manages client-side JSON-subscription balancers; rows
// are read per request by internal/sub, so mutations need no xray restart.
type SubBalancerService struct{}

func (s *SubBalancerService) validate(b *model.SubBalancer) error {
	b.Remark = strings.TrimSpace(b.Remark)
	if b.Remark == "" {
		return common.NewError("balancer remark is required")
	}
	if len(b.Remark) > 256 {
		return common.NewError("balancer remark too long (max 256)")
	}
	if b.Strategy == "" {
		b.Strategy = "random"
	}
	if _, ok := subBalancerStrategies[b.Strategy]; !ok {
		return common.NewError("invalid balancer strategy:", b.Strategy)
	}
	if len(b.InboundIds) == 0 {
		return common.NewError("balancer must select at least one inbound")
	}
	if err := s.validateWeights(b); err != nil {
		return err
	}
	if b.SortOrder < 1 {
		b.SortOrder = 1
	}
	return nil
}

// validateWeights rejects non-positive weights (a silent default would hide a
// typo like "0" meaning "never pick this node"), drops entries for inbounds no
// longer selected, and errors on weights with any strategy but leastLoad —
// xray would ignore them, so storing them would pretend a knob exists.
func (s *SubBalancerService) validateWeights(b *model.SubBalancer) error {
	if len(b.MemberWeights) == 0 {
		b.MemberWeights = nil
		return nil
	}
	if b.Strategy != "leastLoad" {
		return common.NewError("balancer weights only apply to the leastLoad strategy")
	}
	cleaned := make(map[int]float64, len(b.MemberWeights))
	for id, weight := range b.MemberWeights {
		if !slices.Contains(b.InboundIds, id) {
			continue
		}
		if weight <= 0 {
			return common.NewError("balancer member weights must be greater than 0")
		}
		cleaned[id] = weight
	}
	if len(cleaned) == 0 {
		cleaned = nil
	}
	b.MemberWeights = cleaned
	return nil
}

// List returns all balancers in subscription order.
func (s *SubBalancerService) List() ([]*model.SubBalancer, error) {
	var balancers []*model.SubBalancer
	err := database.GetDB().Model(&model.SubBalancer{}).
		Order("sort_order asc, id asc").Find(&balancers).Error
	return balancers, err
}

func (s *SubBalancerService) Get(id int) (*model.SubBalancer, error) {
	var balancer model.SubBalancer
	if err := database.GetDB().First(&balancer, id).Error; err != nil {
		return nil, err
	}
	return &balancer, nil
}

func (s *SubBalancerService) Create(balancer *model.SubBalancer) (*model.SubBalancer, error) {
	if err := s.validate(balancer); err != nil {
		return nil, err
	}
	if err := database.GetDB().Create(balancer).Error; err != nil {
		return nil, err
	}
	return balancer, nil
}

func (s *SubBalancerService) Update(id int, balancer *model.SubBalancer, enabled *bool) (*model.SubBalancer, error) {
	if err := s.validate(balancer); err != nil {
		return nil, err
	}
	current, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	current.Remark = balancer.Remark
	current.Strategy = balancer.Strategy
	current.InboundIds = balancer.InboundIds
	current.MemberWeights = balancer.MemberWeights
	current.SortOrder = balancer.SortOrder
	if enabled != nil {
		current.Enabled = *enabled
	}
	if err := database.GetDB().Save(current).Error; err != nil {
		return nil, err
	}
	return current, nil
}

func (s *SubBalancerService) Delete(id int) error {
	res := database.GetDB().Delete(&model.SubBalancer{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return common.NewError("sub balancer not found")
	}
	return nil
}
