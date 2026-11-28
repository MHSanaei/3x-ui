package controller

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Attachment jobs belong to the same administrator-only API as bulkAttach.
// Keep a bounded in-memory history of results and run one picker job at a time.
type attachJobStore struct {
	mu      sync.Mutex
	running bool
	jobs    map[string]AttachJobStatus
}

type AttachJobStatus struct {
	Running bool                           `json:"running" example:"false"`
	Result  *service.AttachSelectionResult `json:"result,omitempty"`
	Error   string                         `json:"error,omitempty"`
	ended   time.Time
}

var attachJobs attachJobStore

func (s *attachJobStore) start(run func() (*service.AttachSelectionResult, error)) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return "", errors.New("another client attachment is running")
	}
	if s.jobs == nil {
		s.jobs = make(map[string]AttachJobStatus)
	}
	cutoff := time.Now().Add(-15 * time.Minute)
	var oldestID string
	var oldest time.Time
	for id, status := range s.jobs {
		if status.ended.Before(cutoff) {
			delete(s.jobs, id)
			continue
		}
		if oldestID == "" || status.ended.Before(oldest) {
			oldestID, oldest = id, status.ended
		}
	}
	if len(s.jobs) >= 32 {
		delete(s.jobs, oldestID)
	}
	id := uuid.NewString()
	s.jobs[id] = AttachJobStatus{Running: true}
	s.running = true
	common.GoRecover("client-attachment", func() {
		status := AttachJobStatus{Error: "client attachment interrupted"}
		defer func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			status.ended = time.Now()
			s.jobs[id] = status
			s.running = false
		}()
		result, err := run()
		if err != nil {
			status.Error = err.Error()
			return
		}
		status.Error = ""
		status.Result = result
	})
	return id, nil
}

func (s *attachJobStore) get(id string) (AttachJobStatus, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.jobs[id]
	if !status.Running && time.Since(status.ended) > 15*time.Minute {
		delete(s.jobs, id)
		return AttachJobStatus{}, false
	}
	return status, ok
}

func (a *ClientController) beginBulkAttach(c *gin.Context, targets []int, selection service.AttachSelection) {
	id, err := attachJobs.start(func() (*service.AttachSelectionResult, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		result, restart, err := a.clientService.BulkAttachSelection(ctx, &a.inboundService, targets, selection)
		if restart {
			a.xrayService.SetToNeedRestart()
		}
		if err == nil {
			notifyClientsChanged()
		}
		return result, err
	})
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"jobId": id}, nil)
}

func (a *ClientController) attachStatus(c *gin.Context) {
	status, ok := attachJobs.get(c.Param("id"))
	if !ok {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), errors.New("client attachment status unavailable"))
		return
	}
	jsonObj(c, status, nil)
}
