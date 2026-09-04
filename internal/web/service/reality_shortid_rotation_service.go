package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const defaultRealityShortIDRotationDays = 30

type RealityShortIDRotationRunResult struct {
	Initialized int
	Rotated     int
	Retired     int
	NeedRestart bool
}

type realityShortIDTransition struct {
	Initialized bool
	Rotated     bool
	Retired     bool
	Changed     bool
	NeedRestart bool
}

// Ordinary edits preserve server-owned state; manual shortIds edits reset it
// to a fresh active set without a retiring suffix.
func normalizeRealityShortIDRotation(inbound, old *model.Inbound, now time.Time) error {
	if !inbound.RealityShortIdsRotationEnabled {
		clearRealityShortIDRotationState(inbound)
		return nil
	}
	if inbound.Protocol != model.VLESS {
		return errors.New("automatic REALITY short ID rotation is supported only for VLESS inbounds")
	}
	if inbound.RealityShortIdsRotationDays <= 0 {
		inbound.RealityShortIdsRotationDays = defaultRealityShortIDRotationDays
	}
	if inbound.RealityShortIdsGraceHours < 0 {
		return errors.New("REALITY short ID grace period cannot be negative")
	}
	if inbound.RealityShortIdsGraceHours >= inbound.RealityShortIdsRotationDays*24 {
		return errors.New("REALITY short ID grace period must be shorter than the rotation interval")
	}

	_, _, incomingIDs, err := parseRealityShortIDs(inbound.StreamSettings)
	if err != nil {
		return err
	}
	if len(incomingIDs) == 0 {
		return errors.New("REALITY shortIds must contain at least one entry")
	}

	resetState := old == nil || !old.RealityShortIdsRotationEnabled
	oldDays := 0
	if old != nil {
		oldDays = old.RealityShortIdsRotationDays
		inbound.RealityShortIdsActiveCount = old.RealityShortIdsActiveCount
		inbound.RealityShortIdsRotationCursor = old.RealityShortIdsRotationCursor
		inbound.RealityShortIdsLastRotationTime = old.RealityShortIdsLastRotationTime
		inbound.RealityShortIdsNextRotationTime = old.RealityShortIdsNextRotationTime
		inbound.RealityShortIdsRetireAt = old.RealityShortIdsRetireAt

		if old.RealityShortIdsRotationEnabled {
			_, _, oldIDs, oldErr := parseRealityShortIDs(old.StreamSettings)
			if oldErr != nil || !slices.Equal(oldIDs, incomingIDs) {
				resetState = true
			}
		}
	}

	if resetState {
		inbound.RealityShortIdsActiveCount = len(incomingIDs)
		inbound.RealityShortIdsRotationCursor = 0
		inbound.RealityShortIdsLastRotationTime = 0
		inbound.RealityShortIdsNextRotationTime = now.AddDate(0, 0, inbound.RealityShortIdsRotationDays).UnixMilli()
		inbound.RealityShortIdsRetireAt = 0
	} else if oldDays != inbound.RealityShortIdsRotationDays {
		inbound.RealityShortIdsNextRotationTime = now.AddDate(0, 0, inbound.RealityShortIdsRotationDays).UnixMilli()
		retireAt := now.Add(time.Duration(inbound.RealityShortIdsGraceHours) * time.Hour).UnixMilli()
		if inbound.RealityShortIdsRetireAt > retireAt {
			inbound.RealityShortIdsRetireAt = retireAt
		}
	}

	activeCount := inbound.RealityShortIdsActiveCount
	if activeCount <= 0 || activeCount > len(incomingIDs) {
		return fmt.Errorf("REALITY active short ID count %d is invalid for list length %d", activeCount, len(incomingIDs))
	}
	if count := inbound.RealityShortIdsRotationCount; count < 0 || count > activeCount {
		return fmt.Errorf("REALITY rotation count %d must be between 0 and %d", count, activeCount)
	}
	return nil
}

func clearRealityShortIDRotationState(inbound *model.Inbound) {
	inbound.RealityShortIdsActiveCount = 0
	inbound.RealityShortIdsRotationCursor = 0
	inbound.RealityShortIdsLastRotationTime = 0
	inbound.RealityShortIdsNextRotationTime = 0
	inbound.RealityShortIdsRetireAt = 0
}

// ProcessRealityShortIDRotations advances every due inbound independently, so
// one malformed row cannot prevent healthy inbounds from rotating.
func (s *InboundService) ProcessRealityShortIDRotations(now time.Time) (RealityShortIDRotationRunResult, error) {
	result := RealityShortIDRotationRunResult{}
	nowMs := now.UnixMilli()
	var ids []int
	err := database.GetDB().Model(&model.Inbound{}).
		Where("enable = ? AND protocol = ? AND reality_short_ids_rotation_enabled = ?", true, model.VLESS, true).
		Where(`reality_short_ids_next_rotation_time = 0
            OR (reality_short_ids_next_rotation_time > 0 AND reality_short_ids_next_rotation_time <= ?)
            OR (reality_short_ids_retire_at > 0 AND reality_short_ids_retire_at <= ?)`, nowMs, nowMs).
		Order("id ASC").
		Pluck("id", &ids).Error
	if err != nil {
		return result, err
	}

	var joined error
	for _, id := range ids {
		transition, transitionErr := s.processRealityShortIDTransition(id, now)
		if transition.Initialized {
			result.Initialized++
		}
		if transition.Rotated {
			result.Rotated++
		}
		if transition.Retired {
			result.Retired++
		}
		result.NeedRestart = result.NeedRestart || transition.NeedRestart
		if transitionErr != nil {
			joined = errors.Join(joined, fmt.Errorf("inbound %d: %w", id, transitionErr))
		}
	}
	return result, joined
}

func (s *InboundService) processRealityShortIDTransition(id int, now time.Time) (realityShortIDTransition, error) {
	transition := realityShortIDTransition{}
	nowMs := now.UnixMilli()
	var oldSnapshot, updatedSnapshot model.Inbound

	err := runSerializedTx(func(tx *gorm.DB) error {
		var inbound model.Inbound
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&inbound, id).Error; err != nil {
			return err
		}
		if !inbound.Enable || inbound.Protocol != model.VLESS || !inbound.RealityShortIdsRotationEnabled {
			return nil
		}
		if err := normalizeStoredRealityShortIDRotation(&inbound); err != nil {
			return err
		}
		oldSnapshot = inbound

		if inbound.RealityShortIdsNextRotationTime == 0 {
			inbound.RealityShortIdsNextRotationTime = now.AddDate(0, 0, inbound.RealityShortIdsRotationDays).UnixMilli()
			transition.Initialized = true
		}

		if inbound.RealityShortIdsRetireAt > 0 && inbound.RealityShortIdsRetireAt <= nowMs {
			cleaned, retired, err := retireRealityShortIDs(inbound.StreamSettings, inbound.RealityShortIdsActiveCount)
			if err != nil {
				return err
			}
			inbound.StreamSettings = cleaned
			inbound.RealityShortIdsRetireAt = 0
			transition.Retired = len(retired) > 0
			transition.Changed = transition.Changed || transition.Retired
		}

		if inbound.RealityShortIdsRetireAt == 0 &&
			inbound.RealityShortIdsNextRotationTime > 0 &&
			inbound.RealityShortIdsNextRotationTime <= nowMs {
			rotation, err := rotateRealityShortIDs(
				inbound.StreamSettings,
				inbound.RealityShortIdsActiveCount,
				inbound.RealityShortIdsRotationCursor,
				inbound.RealityShortIdsRotationCount,
				nil,
			)
			if err != nil {
				return err
			}
			inbound.StreamSettings = rotation.StreamSettings
			inbound.RealityShortIdsActiveCount = rotation.ActiveCount
			inbound.RealityShortIdsRotationCursor = rotation.NextCursor
			inbound.RealityShortIdsLastRotationTime = nowMs
			inbound.RealityShortIdsNextRotationTime = now.AddDate(0, 0, inbound.RealityShortIdsRotationDays).UnixMilli()
			inbound.RealityShortIdsRetireAt = now.Add(time.Duration(inbound.RealityShortIdsGraceHours) * time.Hour).UnixMilli()
			transition.Rotated = true
			transition.Changed = true

			if inbound.RealityShortIdsGraceHours == 0 {
				cleaned, retired, retireErr := retireRealityShortIDs(inbound.StreamSettings, inbound.RealityShortIdsActiveCount)
				if retireErr != nil {
					return retireErr
				}
				inbound.StreamSettings = cleaned
				inbound.RealityShortIdsRetireAt = 0
				transition.Retired = len(retired) > 0
			}
		}

		updates := map[string]any{
			"stream_settings":                      inbound.StreamSettings,
			"reality_short_ids_active_count":       inbound.RealityShortIdsActiveCount,
			"reality_short_ids_rotation_cursor":    inbound.RealityShortIdsRotationCursor,
			"reality_short_ids_last_rotation_time": inbound.RealityShortIdsLastRotationTime,
			"reality_short_ids_next_rotation_time": inbound.RealityShortIdsNextRotationTime,
			"reality_short_ids_retire_at":          inbound.RealityShortIdsRetireAt,
			"reality_short_ids_rotation_days":      inbound.RealityShortIdsRotationDays,
			"reality_short_ids_grace_hours":        inbound.RealityShortIdsGraceHours,
		}
		if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Updates(updates).Error; err != nil {
			return err
		}
		if transition.Changed && inbound.NodeID != nil {
			if err := (&NodeService{}).MarkNodeDirtyTx(tx, *inbound.NodeID); err != nil {
				return err
			}
		}
		updatedSnapshot = inbound
		return nil
	})
	if err != nil || !transition.Changed {
		return transition, err
	}

	rt, push, _, planErr := s.nodePushPlan(&updatedSnapshot)
	if planErr != nil {
		if updatedSnapshot.NodeID == nil {
			transition.NeedRestart = true
		}
		return transition, planErr
	}
	if !push {
		if updatedSnapshot.NodeID == nil {
			transition.NeedRestart = true
		}
		return transition, nil
	}
	buildPayload := s.buildInboundForNodePush
	if updatedSnapshot.NodeID == nil {
		buildPayload = s.buildInboundForLocalRuntime
	}
	oldPayload, buildErr := buildPayload(database.GetDB(), &oldSnapshot)
	if buildErr != nil {
		if updatedSnapshot.NodeID == nil {
			transition.NeedRestart = true
		}
		return transition, fmt.Errorf("prepare old REALITY rotation payload: %w", buildErr)
	}
	updatedPayload, buildErr := buildPayload(database.GetDB(), &updatedSnapshot)
	if buildErr != nil {
		if updatedSnapshot.NodeID == nil {
			transition.NeedRestart = true
		}
		return transition, fmt.Errorf("prepare updated REALITY rotation payload: %w", buildErr)
	}
	if err := rt.UpdateInbound(context.Background(), oldPayload, updatedPayload); err != nil {
		if updatedSnapshot.NodeID == nil {
			transition.NeedRestart = true
		}
		return transition, fmt.Errorf("apply REALITY short ID update on %s: %w", rt.Name(), err)
	}
	return transition, nil
}

func normalizeStoredRealityShortIDRotation(inbound *model.Inbound) error {
	if inbound.RealityShortIdsRotationDays <= 0 {
		inbound.RealityShortIdsRotationDays = defaultRealityShortIDRotationDays
	}
	if inbound.RealityShortIdsGraceHours < 0 {
		return errors.New("REALITY short ID grace period cannot be negative")
	}
	if inbound.RealityShortIdsGraceHours >= inbound.RealityShortIdsRotationDays*24 {
		return errors.New("REALITY short ID grace period must be shorter than the rotation interval")
	}
	_, _, shortIDs, err := parseRealityShortIDs(inbound.StreamSettings)
	if err != nil {
		return err
	}
	if len(shortIDs) == 0 {
		return errors.New("REALITY shortIds must contain at least one entry")
	}
	if inbound.RealityShortIdsActiveCount <= 0 {
		inbound.RealityShortIdsActiveCount = len(shortIDs)
	}
	if inbound.RealityShortIdsActiveCount > len(shortIDs) {
		return fmt.Errorf("REALITY active short ID count %d exceeds list length %d", inbound.RealityShortIdsActiveCount, len(shortIDs))
	}
	if count := inbound.RealityShortIdsRotationCount; count < 0 || count > inbound.RealityShortIdsActiveCount {
		return fmt.Errorf("REALITY rotation count %d must be between 0 and %d", count, inbound.RealityShortIdsActiveCount)
	}
	return nil
}
