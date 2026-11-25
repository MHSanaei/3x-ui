package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"

	"gorm.io/gorm"
)

func (s *NodeService) restartTarget(id int) (*model.Node, *runtime.Manager, error) {
	if id <= 0 {
		return nil, nil, fmt.Errorf("invalid node ID")
	}
	node, err := s.GetById(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, fmt.Errorf("node not found")
	}
	if err != nil {
		return nil, nil, err
	}
	if !node.Enable {
		return nil, nil, fmt.Errorf("node is disabled")
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, nil, fmt.Errorf("runtime manager unavailable")
	}
	return node, mgr, nil
}

func (s *NodeService) RestartXray(ctx context.Context, id int) error {
	node, mgr, err := s.restartTarget(id)
	if err != nil {
		return err
	}
	rt, err := mgr.RuntimeFor(&node.Id)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return rt.RestartXray(ctx)
}

func (s *NodeService) RestartPanel(ctx context.Context, id int) error {
	node, mgr, err := s.restartTarget(id)
	if err != nil {
		return err
	}
	remote, err := mgr.RemoteFor(node)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return remote.RestartPanel(ctx)
}
