package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/fnv"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/mtproto"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const clientOperationSettingPrefix = "tgbot.clientOperation.v1."

var clientOperationLocks [128]sync.Mutex

var (
	ErrClientOperationChanged = errors.New("client or plan changed; prepare a new preview")
	ErrClientOperationPending = errors.New("client operation synchronization pending")
)

type ClientRenewalSpec struct {
	Mode         string `json:"mode"`
	QuotaBytes   int64  `json:"quotaBytes"`
	Days         int    `json:"days"`
	FirstUse     bool   `json:"firstUse"`
	KeepDisabled bool   `json:"keepDisabled,omitempty"`
	PlanID       string `json:"planId,omitempty"`
	PlanRevision int64  `json:"planRevision,omitempty"`
}

type ClientAdminPreview struct {
	OldQuotaBytes int64 `json:"oldQuotaBytes"`
	UsedBytes     int64 `json:"usedBytes"`
	NewQuotaBytes int64 `json:"newQuotaBytes"`
	OldExpiry     int64 `json:"oldExpiry"`
	NewExpiry     int64 `json:"newExpiry"`
	ResetUsage    bool  `json:"resetUsage"`
	EnabledBefore bool  `json:"enabledBefore"`
	EnabledAfter  bool  `json:"enabledAfter"`
	InboundIDs    []int `json:"inboundIds"`
	LimitIP       int   `json:"limitIp"`
}

type ClientAdminOperation struct {
	Token       string             `json:"token"`
	Owner       int64              `json:"owner"`
	Email       string             `json:"email"`
	RecordID    int                `json:"recordId,omitempty"`
	Kind        string             `json:"kind"`
	State       string             `json:"state"`
	ExpiresAt   int64              `json:"expiresAt"`
	ConfirmedAt int64              `json:"confirmedAt,omitempty"`
	Spec        ClientRenewalSpec  `json:"spec"`
	Preview     ClientAdminPreview `json:"preview"`
	Baseline    string             `json:"baseline,omitempty"`
	SubID       string             `json:"subId,omitempty"`
	Plan        *ClientPlan        `json:"plan,omitempty"`
	NeedsEnable bool               `json:"needsEnable"`
}

type ClientAdminWorkflowService struct{}

type clientLifecycleSnapshot struct {
	record      *model.ClientRecord
	traffic     xray.ClientTraffic
	inbounds    []*model.Inbound
	settings    map[int]map[string]any
	clients     map[int]map[string]any
	fingerprint string
}

func clientOperationLock(email string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(email)))
	return &clientOperationLocks[h.Sum32()%uint32(len(clientOperationLocks))]
}

func loadClientLifecycle(tx *gorm.DB, email string) (*clientLifecycleSnapshot, error) {
	svc := &ClientService{}
	record, err := svc.GetRecordByEmail(tx, email)
	if err != nil {
		return nil, err
	}
	snap := &clientLifecycleSnapshot{record: record, settings: map[int]map[string]any{}, clients: map[int]map[string]any{}}
	if err := tx.Where("email = ?", email).First(&snap.traffic).Error; err != nil {
		return nil, err
	}
	if err := overlayGlobalTrafficChecked(tx, []*xray.ClientTraffic{&snap.traffic}); err != nil {
		return nil, err
	}
	snap.inbounds, err = svc.GetAttachedInbounds(tx, email)
	if err != nil {
		return nil, err
	}
	type attachment struct {
		ID       int
		Protocol model.Protocol
		NodeID   *int
		Enabled  bool
		Client   map[string]any
	}
	attached := make([]attachment, 0, len(snap.inbounds))
	for _, inbound := range snap.inbounds {
		settings, err := decodeInboundSettingsObject(inbound.Settings)
		if err != nil {
			return nil, err
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			return nil, errors.New("inbound clients are missing")
		}
		for _, item := range clients {
			client, ok := item.(map[string]any)
			if !ok {
				return nil, errors.New("invalid inbound client")
			}
			if client["email"] != email {
				continue
			}
			if snap.clients[inbound.Id] != nil {
				return nil, errors.New("duplicate client in inbound settings")
			}
			snap.clients[inbound.Id] = client
		}
		if snap.clients[inbound.Id] == nil {
			return nil, errors.New("client attachment and inbound settings disagree")
		}
		snap.settings[inbound.Id] = settings
		attached = append(attached, attachment{inbound.Id, inbound.Protocol, inbound.NodeID, inbound.Enable, snap.clients[inbound.Id]})
	}
	data, err := json.Marshal(struct {
		Record        *model.ClientRecord
		Attached      []attachment
		Total, Expiry int64
		Enabled       bool
	}{record, attached, snap.traffic.Total, snap.traffic.ExpiryTime, snap.traffic.Enable})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	snap.fingerprint = hex.EncodeToString(digest[:])
	return snap, nil
}

func validateRenewalSpec(spec ClientRenewalSpec) error {
	if spec.Mode != "replace" && spec.Mode != "extend" {
		return errors.New("choose replace or extend renewal")
	}
	if spec.QuotaBytes < 0 || spec.Days < 0 || spec.Days > 36500 {
		return errors.New("invalid renewal quota or duration")
	}
	return nil
}

func renewalPreview(snap *clientLifecycleSnapshot, spec ClientRenewalSpec, now int64) (ClientAdminPreview, error) {
	if err := validateRenewalSpec(spec); err != nil {
		return ClientAdminPreview{}, err
	}
	traffic := snap.traffic
	if traffic.Up < 0 || traffic.Down < 0 || traffic.Up > math.MaxInt64-traffic.Down {
		return ClientAdminPreview{}, errors.New("invalid client traffic counters")
	}
	p := ClientAdminPreview{OldQuotaBytes: snap.record.TotalGB, UsedBytes: traffic.Up + traffic.Down, OldExpiry: snap.record.ExpiryTime, EnabledBefore: snap.record.Enable, LimitIP: snap.record.LimitIP}
	for _, inbound := range snap.inbounds {
		p.InboundIDs = append(p.InboundIDs, inbound.Id)
	}
	p.ResetUsage = spec.Mode == "replace"
	duration := int64(spec.Days) * 86400000
	if p.ResetUsage {
		p.NewQuotaBytes = spec.QuotaBytes
		p.NewExpiry = duration
		if spec.FirstUse {
			p.NewExpiry = -duration
		} else if duration > 0 {
			p.NewExpiry += now
		}
	} else {
		p.NewQuotaBytes = p.OldQuotaBytes
		if p.NewQuotaBytes > 0 {
			if spec.QuotaBytes > math.MaxInt64-p.NewQuotaBytes {
				return p, errors.New("renewal quota overflows")
			}
			p.NewQuotaBytes += spec.QuotaBytes
		}
		p.NewExpiry = p.OldExpiry
		if p.NewExpiry < 0 {
			if p.NewExpiry < math.MinInt64+duration {
				return p, errors.New("renewal expiry overflows")
			}
			p.NewExpiry -= duration
		} else if p.NewExpiry > 0 && duration > 0 {
			if p.NewExpiry < now {
				p.NewExpiry = now
			}
			if p.NewExpiry > math.MaxInt64-duration {
				return p, errors.New("renewal expiry overflows")
			}
			p.NewExpiry += duration
		}
	}
	wasDepleted := (traffic.Total > 0 && p.UsedBytes >= traffic.Total) || (traffic.ExpiryTime > 0 && traffic.ExpiryTime <= now)
	used := p.UsedBytes
	if p.ResetUsage {
		used = 0
	}
	stillDepleted := (p.NewQuotaBytes > 0 && used >= p.NewQuotaBytes) || (p.NewExpiry > 0 && p.NewExpiry <= now)
	p.EnabledAfter = (p.EnabledBefore || wasDepleted) && !stillDepleted && (!spec.KeepDisabled || p.EnabledBefore)
	return p, nil
}

func newClientOperation(owner int64, email, kind string) (*ClientAdminOperation, error) {
	if owner <= 0 || email == "" {
		return nil, errors.New("invalid operation owner or client")
	}
	if err := validateClientEmail(email); err != nil {
		return nil, err
	}
	now := time.Now()
	return &ClientAdminOperation{Token: now.UTC().Format("20060102") + "-" + uuid.NewString(), Owner: owner, Email: email, Kind: kind, State: "pending", ExpiresAt: now.Add(20 * time.Minute).UnixMilli()}, nil
}

func clientOperationKey(token string) (string, error) {
	if len(token) != 45 || token[8] != '-' {
		return "", errors.New("invalid operation token")
	}
	if _, err := time.Parse("20060102", token[:8]); err != nil {
		return "", err
	}
	if _, err := uuid.Parse(token[9:]); err != nil {
		return "", err
	}
	return clientOperationSettingPrefix + token, nil
}

func loadClientOperation(tx *gorm.DB, token string, owner int64) (*ClientAdminOperation, error) {
	key, err := clientOperationKey(token)
	if err != nil {
		return nil, err
	}
	op := &ClientAdminOperation{}
	if err := readBotSetting(tx, key, op); err != nil {
		return nil, err
	}
	if op.Token != token || op.Owner != owner || owner <= 0 {
		return nil, errors.New("operation belongs to another administrator")
	}
	if (op.Kind != "renew" && op.Kind != "create") || (op.Kind == "create" && op.Plan == nil) {
		return nil, errors.New("invalid stored client operation")
	}
	return op, nil
}

func saveClientOperation(tx *gorm.DB, op *ClientAdminOperation) error {
	key, err := clientOperationKey(op.Token)
	if err != nil {
		return err
	}
	return writeBotSetting(tx, key, op)
}

func pruneClientOperations(tx *gorm.DB) error {
	cutoff := clientOperationSettingPrefix + time.Now().UTC().AddDate(0, 0, -31).Format("20060102")
	var ids []int
	if err := tx.Model(&model.Setting{}).Where("key >= ? AND key < ?", clientOperationSettingPrefix, cutoff).Limit(sqlInChunk).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	return tx.Where("id IN ?", ids).Delete(&model.Setting{}).Error
}

func operationPlan(tx *gorm.DB, spec ClientRenewalSpec) (*ClientPlan, error) {
	if spec.PlanID == "" {
		return nil, nil
	}
	plans, err := loadClientPlans(tx)
	if err != nil {
		return nil, err
	}
	for _, plan := range plans {
		if plan.ID == spec.PlanID && plan.Revision == spec.PlanRevision {
			return &plan, nil
		}
	}
	return nil, ErrClientOperationChanged
}

func (s *ClientAdminWorkflowService) PrepareRenewPlan(owner int64, email, name string) (*ClientAdminOperation, error) {
	plan, err := (&ClientPlanService{}).Resolve(name)
	if err != nil {
		return nil, err
	}
	return s.PrepareRenew(owner, email, ClientRenewalSpec{Mode: plan.RenewalMode, QuotaBytes: plan.QuotaBytes, Days: plan.Days, FirstUse: plan.FirstUse, PlanID: plan.ID, PlanRevision: plan.Revision})
}

func (s *ClientAdminWorkflowService) PrepareCreate(owner int64, email, name string) (*ClientAdminOperation, error) {
	op, err := newClientOperation(owner, email, "create")
	if err != nil {
		return nil, err
	}
	botSettingsMu.Lock()
	defer botSettingsMu.Unlock()
	err = runSerializedTx(func(tx *gorm.DB) error {
		plans, err := loadClientPlans(tx)
		if err != nil {
			return err
		}
		for _, plan := range plans {
			if strings.EqualFold(plan.Name, name) || plan.ID == name {
				op.Plan = &plan
				break
			}
		}
		if op.Plan == nil {
			return errors.New("plan not found")
		}
		if err := validateClientPlan(tx, *op.Plan, email); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.ClientRecord{}).Where("email = ?", email).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return errors.New("client already exists")
		}
		op.SubID = uuid.NewString()
		op.Spec = ClientRenewalSpec{PlanID: op.Plan.ID, PlanRevision: op.Plan.Revision}
		payload := op.Plan.Payload(email, op.SubID, time.Now().UnixMilli())
		op.Preview = ClientAdminPreview{NewQuotaBytes: payload.Client.TotalGB, NewExpiry: payload.Client.ExpiryTime, EnabledAfter: payload.Client.Enable, InboundIDs: payload.InboundIds, LimitIP: payload.Client.LimitIP}
		if err := pruneClientOperations(tx); err != nil {
			return err
		}
		return saveClientOperation(tx, op)
	})
	if err != nil {
		return nil, err
	}
	return op, nil
}

func (s *ClientAdminWorkflowService) PrepareRenew(owner int64, email string, spec ClientRenewalSpec) (*ClientAdminOperation, error) {
	op, err := newClientOperation(owner, email, "renew")
	if err != nil {
		return nil, err
	}
	op.Spec = spec
	botSettingsMu.Lock()
	defer botSettingsMu.Unlock()
	err = runSerializedTx(func(tx *gorm.DB) error {
		plan, err := operationPlan(tx, spec)
		if err != nil {
			return err
		}
		if plan != nil {
			if err := validateClientPlan(tx, *plan, email); err != nil {
				return err
			}
			if spec.Mode != plan.RenewalMode || spec.QuotaBytes != plan.QuotaBytes || spec.Days != plan.Days || spec.FirstUse != plan.FirstUse {
				return ErrClientOperationChanged
			}
		}
		snap, err := loadClientLifecycle(tx, email)
		if err != nil {
			return err
		}
		op.Preview, err = renewalPreview(snap, spec, time.Now().UnixMilli())
		if err != nil {
			return err
		}
		op.Baseline = snap.fingerprint
		op.RecordID = snap.record.Id
		if err := pruneClientOperations(tx); err != nil {
			return err
		}
		return saveClientOperation(tx, op)
	})
	if err != nil {
		return nil, err
	}
	return op, nil
}

func (s *ClientAdminWorkflowService) Load(token string, owner int64) (*ClientAdminOperation, error) {
	return loadClientOperation(database.GetDB(), token, owner)
}

func (s *ClientAdminWorkflowService) KeepDisabled(token string, owner int64) (*ClientAdminOperation, error) {
	botSettingsMu.Lock()
	defer botSettingsMu.Unlock()
	var op *ClientAdminOperation
	err := runSerializedTx(func(tx *gorm.DB) error {
		var err error
		op, err = loadClientOperation(tx, token, owner)
		if err != nil {
			return err
		}
		if op.Kind != "renew" || op.State != "pending" || op.ExpiresAt <= tx.NowFunc().UnixMilli() {
			return errors.New("operation is no longer pending")
		}
		snap, err := loadClientLifecycle(tx, op.Email)
		if err != nil {
			return err
		}
		if snap.fingerprint != op.Baseline || snap.record.Enable {
			return ErrClientOperationChanged
		}
		op.Spec.KeepDisabled = true
		op.Preview, err = renewalPreview(snap, op.Spec, tx.NowFunc().UnixMilli())
		if err != nil {
			return err
		}
		return saveClientOperation(tx, op)
	})
	if err != nil {
		return nil, err
	}
	return op, nil
}

func (s *ClientAdminWorkflowService) Cancel(token string, owner int64) error {
	botSettingsMu.Lock()
	defer botSettingsMu.Unlock()
	return runSerializedTx(func(tx *gorm.DB) error {
		op, err := loadClientOperation(tx, token, owner)
		if err != nil {
			return err
		}
		if op.State != "pending" {
			return errors.New("operation is no longer pending")
		}
		op.State = "cancelled"
		return saveClientOperation(tx, op)
	})
}

func (s *ClientService) commitRenewalTx(tx *gorm.DB, op *ClientAdminOperation) error {
	snap, err := loadClientLifecycle(tx, op.Email)
	if err != nil {
		return err
	}
	if snap.fingerprint != op.Baseline {
		return ErrClientOperationChanged
	}
	now := tx.NowFunc().UnixMilli()
	op.Preview, err = renewalPreview(snap, op.Spec, now)
	if err != nil {
		return err
	}
	p := op.Preview
	updatedAt := now
	if updatedAt <= snap.record.UpdatedAt {
		updatedAt = snap.record.UpdatedAt + 1
	}
	if p.ResetUsage {
		if err := resetClientTrafficAccounting(tx, op.Email, p.EnabledAfter); err != nil {
			return err
		}
		if _, err := queueNodeResets(tx, []string{op.Email}); err != nil {
			return err
		}
	}
	if err := tx.Model(&model.ClientRecord{}).Where("id = ?", snap.record.Id).Updates(map[string]any{"total_gb": p.NewQuotaBytes, "expiry_time": p.NewExpiry, "enable": p.EnabledAfter, "updated_at": updatedAt}).Error; err != nil {
		return err
	}
	if err := tx.Model(&xray.ClientTraffic{}).Where("email = ?", op.Email).Updates(map[string]any{"total": p.NewQuotaBytes, "expiry_time": p.NewExpiry, "enable": p.EnabledAfter}).Error; err != nil {
		return err
	}
	for _, inbound := range snap.inbounds {
		client := snap.clients[inbound.Id]
		client["totalGB"], client["expiryTime"], client["enable"], client["updated_at"] = p.NewQuotaBytes, p.NewExpiry, p.EnabledAfter, updatedAt
		base := inbound.Settings
		settings, err := json.Marshal(snap.settings[inbound.Id])
		if err != nil {
			return err
		}
		inbound.Settings = string(settings)
		if err := commitInboundClientSettings(tx, inbound, base); err != nil {
			return err
		}
		if p.ResetUsage {
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("last_traffic_reset_time", now).Error; err != nil {
				return err
			}
		}
		if inbound.NodeID != nil {
			if err := (&NodeService{}).MarkNodeDirtyTx(tx, *inbound.NodeID); err != nil {
				return err
			}
		}
	}
	after, err := loadClientLifecycle(tx, op.Email)
	if err != nil {
		return err
	}
	op.Baseline = after.fingerprint
	op.NeedsEnable = p.EnabledAfter && (!p.EnabledBefore || !snap.traffic.Enable)
	op.State = "committed"
	return saveClientOperation(tx, op)
}

func lockClientWorkflowInbounds(ctx context.Context, email string) (func(), error) {
	ids, err := (&ClientService{}).GetInboundIdsForEmail(nil, email)
	if err != nil {
		return nil, err
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	var locks []*sync.Mutex
	unlock := func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
		}
	}
	for _, id := range ids {
		m := inboundMutex(id)
		if err := lockMutexContext(ctx, m); err != nil {
			unlock()
			return nil, err
		}
		locks = append(locks, m)
	}
	return unlock, nil
}

func captureInterruptedClientCreation(op *ClientAdminOperation) error {
	botSettingsMu.Lock()
	defer botSettingsMu.Unlock()
	return runSerializedTx(func(tx *gorm.DB) error {
		snap, err := loadClientLifecycle(tx, op.Email)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		expected := op.Plan.Payload(op.Email, op.SubID, op.ConfirmedAt).Client
		r := snap.record
		if r.SubID != op.SubID || r.TotalGB != expected.TotalGB || r.ExpiryTime != expected.ExpiryTime || r.LimitIP != expected.LimitIP || r.Enable != expected.Enable || r.TgID != 0 || r.Comment != "" || r.Group != "" || r.Reset != 0 || r.ResetDay != 0 || r.ResetWeekday != 0 {
			return ErrClientOperationChanged
		}
		op.Baseline = snap.fingerprint
		op.RecordID = r.Id
		return saveClientOperation(tx, op)
	})
}

func (s *ClientAdminWorkflowService) Confirm(ctx context.Context, token string, owner int64) (*ClientAdminOperation, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	op, err := s.Load(token, owner)
	if err != nil {
		return nil, err
	}
	lock := clientOperationLock(op.Email)
	if err := lockMutexContext(ctx, lock); err != nil {
		return nil, err
	}
	defer lock.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if op.Kind == "renew" {
		unlock, err := lockClientWorkflowInbounds(ctx, op.Email)
		if err != nil {
			return nil, err
		}
		defer unlock()
	}
	resumeCreation := false
	err = func() error {
		botSettingsMu.Lock()
		defer botSettingsMu.Unlock()
		return runSerializedTx(func(tx *gorm.DB) error {
			var err error
			op, err = loadClientOperation(tx, token, owner)
			if err != nil {
				return err
			}
			resumeCreation = op.State == "creating"
			if op.State == "done" {
				record, err := (&ClientService{}).GetRecordByEmail(tx, op.Email)
				if err != nil || record.Id != op.RecordID {
					return ErrClientOperationChanged
				}
				return nil
			}
			if op.State == "committed" || op.State == "creating" {
				return nil
			}
			if op.State != "pending" || op.ExpiresAt <= time.Now().UnixMilli() {
				return errors.New("operation expired or was cancelled")
			}
			plan, err := operationPlan(tx, op.Spec)
			if err != nil {
				return err
			}
			if op.Kind == "renew" {
				if plan != nil {
					if err := validateClientPlan(tx, *plan, op.Email); err != nil {
						return err
					}
				}
				return (&ClientService{}).commitRenewalTx(tx, op)
			}
			if op.Kind != "create" || plan == nil {
				return errors.New("invalid client operation")
			}
			if err := validateClientPlan(tx, *plan, op.Email); err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&model.ClientRecord{}).Where("email = ?", op.Email).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				return errors.New("client already exists")
			}
			op.State = "creating"
			op.ConfirmedAt = tx.NowFunc().UnixMilli()
			return saveClientOperation(tx, op)
		})
	}()
	if err != nil {
		return nil, err
	}
	if op.State == "done" {
		return op, nil
	}
	if op.Kind == "create" && op.State == "committed" {
		unlock, err := lockClientWorkflowInbounds(ctx, op.Email)
		if err != nil {
			return op, errors.Join(ErrClientOperationPending, err)
		}
		defer unlock()
	}
	if op.State == "creating" {
		if resumeCreation {
			record, err := (&ClientService{}).GetRecordByEmail(nil, op.Email)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
			if record != nil {
				if op.Baseline == "" {
					return nil, ErrClientOperationChanged
				}
				snap, err := loadClientLifecycle(database.GetDB(), op.Email)
				if err != nil {
					return nil, err
				}
				if snap.fingerprint != op.Baseline {
					return nil, ErrClientOperationChanged
				}
			} else if op.Baseline != "" {
				return nil, ErrClientOperationChanged
			}
		}
		payload := op.Plan.Payload(op.Email, op.SubID, op.ConfirmedAt)
		if err := (&ClientPlanService{}).Validate(*op.Plan, op.Email); err != nil {
			return nil, err
		}
		if _, err := (&ClientService{}).Create(&InboundService{}, payload); err != nil {
			if captureErr := captureInterruptedClientCreation(op); captureErr != nil {
				return op, captureErr
			}
			return op, errors.Join(ErrClientOperationPending, err)
		}
		unlock, lockErr := lockClientWorkflowInbounds(ctx, op.Email)
		if lockErr != nil {
			if captureErr := captureInterruptedClientCreation(op); captureErr != nil {
				return op, captureErr
			}
			return op, errors.Join(ErrClientOperationPending, lockErr)
		}
		defer unlock()
		err := func() error {
			botSettingsMu.Lock()
			defer botSettingsMu.Unlock()
			return runSerializedTx(func(tx *gorm.DB) error {
				snap, err := loadClientLifecycle(tx, op.Email)
				if err != nil {
					return err
				}
				if snap.record.SubID != op.SubID {
					return errors.New("client identity changed")
				}
				op.Baseline = snap.fingerprint
				op.RecordID = snap.record.Id
				op.NeedsEnable = snap.record.Enable
				op.State = "committed"
				return saveClientOperation(tx, op)
			})
		}()
		if err != nil {
			return nil, err
		}
	}
	InvalidateSubData()
	if err := applyClientAdminOperation(ctx, op); err != nil {
		return op, errors.Join(ErrClientOperationPending, err)
	}
	err = func() error {
		botSettingsMu.Lock()
		defer botSettingsMu.Unlock()
		return runSerializedTx(func(tx *gorm.DB) error { op.State = "done"; return saveClientOperation(tx, op) })
	}()
	if err != nil {
		return op, err
	}
	return op, nil
}

func applyClientAdminOperation(ctx context.Context, op *ClientAdminOperation) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	snap, err := loadClientLifecycle(database.GetDB(), op.Email)
	if err != nil {
		return err
	}
	if snap.fingerprint != op.Baseline {
		return ErrClientOperationChanged
	}
	inboundSvc := &InboundService{}
	nodes := map[int]bool{}
	var mtprotoIDs []int
	for _, inbound := range snap.inbounds {
		if inbound.NodeID != nil {
			nodes[*inbound.NodeID] = true
		}
		if !inbound.Enable && inbound.NodeID == nil {
			continue
		}
		needsConfig := inbound.NodeID != nil || servedOutsideXray(inbound.Protocol)
		if !needsConfig && !op.NeedsEnable {
			continue
		}
		rt, err := inboundSvc.runtimeFor(inbound)
		if err != nil {
			return err
		}
		if needsConfig {
			desired := inbound
			if inbound.NodeID == nil {
				desired, err = inboundSvc.buildInboundForLocalRuntime(database.GetDB(), inbound)
				if err != nil {
					return err
				}
			}
			if err := rt.UpdateInbound(ctx, inbound, desired); err != nil {
				return err
			}
		} else {
			var client model.Client
			data, err := json.Marshal(snap.clients[inbound.Id])
			if err != nil {
				return err
			}
			if err := json.Unmarshal(data, &client); err != nil {
				return err
			}
			if err := rt.UpdateUser(ctx, inbound, op.Email, client); err != nil {
				return err
			}
		}
		if inbound.NodeID != nil {
			nodes[*inbound.NodeID] = true
		}
		if inbound.NodeID == nil && inbound.Protocol == model.MTProto {
			mtprotoIDs = append(mtprotoIDs, inbound.Id)
		}
	}
	if op.Preview.ResetUsage {
		for id := range nodes {
			rt, err := inboundSvc.runtimeFor(&model.Inbound{NodeID: &id})
			if err != nil {
				return err
			}
			if err := inboundSvc.DeliverNodeResets(ctx, id, rt); err != nil {
				return err
			}
		}
		if len(mtprotoIDs) > 0 && snap.record.Enable {
			return mtproto.GetManager().ResetQuotaContext(ctx, op.Email, mtprotoIDs...)
		}
	}
	return nil
}
