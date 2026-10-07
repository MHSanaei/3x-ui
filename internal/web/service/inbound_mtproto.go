package service

import (
	"context"
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// DesiredMtprotoInstances derives the mtg sidecar configs this panel should be
// running: one instance per enabled local mtproto inbound, serving only the
// secrets of clients that are both enabled in the inbound settings and not
// depletion-disabled in client_traffics. That is the same effective client set
// buildInboundForLocalRuntime pushes on interactive edits, so the reconcile job
// and the push paths agree on one fingerprint — a disagreement would surface
// as a needless mtg restart, and a job that read only the raw settings would
// keep serving depleted clients until an unrelated restart. Inbounds whose
// every secret is filtered away are omitted so Reconcile stops their sidecar.
func (s *InboundService) DesiredMtprotoInstances() ([]mtproto.Instance, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).
		Where("protocol = ? AND enable = ? AND node_id IS NULL", model.MTProto, true).
		Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	if len(inbounds) == 0 {
		return nil, nil
	}

	instances := make([]mtproto.Instance, 0, len(inbounds))
	for _, ib := range inbounds {
		inst, ok := mtproto.InstanceFromInbound(ib)
		if !ok {
			continue
		}
		instances = append(instances, inst)
	}
	emails := make([]string, 0)
	for _, inst := range instances {
		for _, e := range inst.Secrets {
			emails = append(emails, e.Name)
		}
	}
	disabled, err := trafficDisabledEmails(db, emails)
	if err != nil {
		return nil, err
	}
	served := instances[:0]
	for _, inst := range instances {
		kept := make([]mtproto.SecretEntry, 0, len(inst.Secrets))
		for _, e := range inst.Secrets {
			if _, off := disabled[e.Name]; !off {
				kept = append(kept, e)
			}
		}
		inst.Secrets = kept
		if len(kept) > 0 {
			served = append(served, inst)
		}
	}
	return served, nil
}

// applyLocalMtproto pushes a single local mtproto inbound's current client set
// to its mtg sidecar right after a client edit commits, so an add, removal,
// re-key or enable-toggle takes effect immediately instead of waiting up to
// 10s for the reconcile job. With a reload-capable mtg the change is applied in
// place without dropping other clients; older binaries fall back to a restart
// inside the manager. It re-reads the inbound so it sees the committed settings,
// filters depleted clients exactly like the reconcile job, and is a no-op for
// node-owned or non-mtproto inbounds. Failures are logged and swallowed: the
// reconcile job is the backstop, and an xray restart cannot help the sidecar.
func (s *InboundService) applyLocalMtproto(inboundId int) {
	inbound, err := s.GetInbound(inboundId)
	if err != nil || inbound == nil || inbound.Protocol != model.MTProto || inbound.NodeID != nil {
		return
	}
	rt, err := s.runtimeFor(inbound)
	if err != nil {
		return
	}
	payload := inbound
	if inbound.Enable {
		if built, bErr := s.buildInboundForLocalRuntime(database.GetDB(), inbound); bErr == nil {
			payload = built
		}
	}
	if err := rt.UpdateInbound(context.Background(), inbound, payload); err != nil {
		logger.Debug("mtproto: immediate client apply failed for inbound", inboundId, ":", err)
	}
}

func (s *InboundService) resetMtprotoClientQuota(email string) {
	s.resetMtprotoClientQuotas([]string{email})
}

// resetMtprotoClientQuotas zeroes the sidecar's own quota counter for each local
// MTProto client in emails, or it keeps blocking a client the panel just reset.
func (s *InboundService) resetMtprotoClientQuotas(emails []string) {
	mgr := mtproto.GetManager()
	if !mgr.HasRunning() || len(emails) == 0 {
		return
	}
	var inbounds []*model.Inbound
	if err := database.GetDB().Model(model.Inbound{}).
		Where("protocol = ? AND node_id IS NULL", model.MTProto).
		Find(&inbounds).Error; err != nil {
		return
	}
	want := make(map[string]struct{}, len(emails))
	for _, e := range emails {
		want[e] = struct{}{}
	}
	var hit []string
	for _, ib := range inbounds {
		inst, ok := mtproto.InstanceFromInbound(ib)
		if !ok {
			continue
		}
		applied := false
		for _, sec := range inst.Secrets {
			if _, ok := want[sec.Name]; !ok {
				continue
			}
			if !applied {
				s.applyLocalMtproto(ib.Id)
				applied = true
			}
			hit = append(hit, sec.Name)
		}
	}
	for _, email := range hit {
		mgr.ResetQuota(email)
	}
}

func (s *InboundService) resetAllMtprotoQuotas() {
	mgr := mtproto.GetManager()
	if !mgr.HasRunning() {
		return
	}
	desired, err := s.DesiredMtprotoInstances()
	if err != nil {
		return
	}
	mgr.Reconcile(desired)
	for _, inst := range desired {
		for _, sec := range inst.Secrets {
			mgr.ResetQuota(sec.Name)
		}
	}
}

// validateMtprotoSettings rejects a warm DC pool size mtg cannot use; zero or
// absent leaves the size to mtg. Other mtproto inbound options are free-form.
func validateMtprotoSettings(inbound *model.Inbound) error {
	if inbound == nil || inbound.Protocol != model.MTProto {
		return nil
	}
	var parsed struct {
		DCPool *struct {
			Size int `json:"size"`
		} `json:"dcPool"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil || parsed.DCPool == nil {
		return nil
	}
	if size := parsed.DCPool.Size; size < 0 || size > mtproto.DCPoolMaxSize {
		return common.NewErrorf("mtproto dcPool.size must be between 1 and %d", mtproto.DCPoolMaxSize)
	}
	return nil
}
