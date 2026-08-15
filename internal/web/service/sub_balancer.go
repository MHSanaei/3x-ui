package service

import (
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
	if b.Strategy == "" {
		b.Strategy = "random"
	}
	if _, ok := subBalancerStrategies[b.Strategy]; !ok {
		return common.NewError("invalid balancer strategy:", b.Strategy)
	}
	if len(b.InboundIds) == 0 {
		return common.NewError("balancer must select at least one inbound")
	}
	if b.SortOrder < 1 {
		b.SortOrder = 1
	}
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

func (s *SubBalancerService) Update(id int, balancer *model.SubBalancer) (*model.SubBalancer, error) {
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
	current.SortOrder = balancer.SortOrder
	current.Enabled = balancer.Enabled
	if err := database.GetDB().Save(current).Error; err != nil {
		return nil, err
	}
	return current, nil
}

func (s *SubBalancerService) Delete(id int) error {
	return database.GetDB().Delete(&model.SubBalancer{}, id).Error
}
