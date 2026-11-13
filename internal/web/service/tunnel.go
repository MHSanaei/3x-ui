package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/wattunnel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type TunnelMutation struct {
	Name        string           `json:"name" example:"Edge to exit"`
	NodeID      *int             `json:"nodeId"`
	Revision    int64            `json:"revision"`
	Enabled     bool             `json:"enabled"`
	Config      wattunnel.Config `json:"config"`
	Secret      string           `json:"secret"`
	Token       string           `json:"token"`
	GenerateKey bool             `json:"generateKey"`
}

type TunnelView struct {
	ID       int              `json:"id" example:"1"`
	GUID     string           `json:"guid"`
	Name     string           `json:"name"`
	NodeID   *int             `json:"nodeId"`
	Revision int64            `json:"revision"`
	Enabled  bool             `json:"enabled"`
	Managed  bool             `json:"managed"`
	Config   wattunnel.Config `json:"config"`
	Status   wattunnel.Status `json:"status"`
}

type TunnelToken struct {
	Token string `json:"token"`
}
type TunnelService struct{}

var (
	tunnelMu         sync.Mutex
	tunnelStatuses   = map[string]wattunnel.Status{}
	tunnelRuntimeFor = func(nodeID *int) (runtime.TunnelRuntime, error) {
		m := runtime.GetManager()
		if m == nil {
			return nil, errors.New("runtime unavailable")
		}
		return m.TunnelRuntimeFor(nodeID)
	}
)

func (s *TunnelService) codec(create bool) (interface {
	EncryptBound([]byte, string) (string, error)
	DecryptBound([]byte, string) (string, error)
}, error,
) {
	return wattunnel.SecretCodec(wattunnel.KeyDir(), create)
}
func tunnelAAD(guid string) []byte { return []byte("tunnels/secret/" + guid) }

func (s *TunnelService) spec(row model.ManagedTunnel) (wattunnel.Spec, error) {
	out := wattunnel.Spec{GUID: row.GUID, Owner: row.Owner, Revision: row.Revision, Restart: row.Restart, Name: row.Name, Enabled: row.Enabled, Deleted: row.Deleted}
	if row.Deleted {
		return out, nil
	}
	if err := json.Unmarshal([]byte(row.Config), &out.Config); err != nil {
		return out, errors.New("invalid stored tunnel configuration")
	}
	if !strings.HasPrefix(row.Secret, "enc:v1:") {
		return out, errors.New("unencrypted tunnel secret refused")
	}
	codec, err := s.codec(false)
	if err != nil {
		return out, errors.New("tunnel keyring unavailable; restore the original keyring")
	}
	out.Secret, err = codec.DecryptBound(tunnelAAD(row.GUID), row.Secret)
	if err != nil {
		return out, errors.New("cannot decrypt tunnel secret")
	}
	return out, nil
}

func (s *TunnelService) apply(ctx context.Context, row model.ManagedTunnel) wattunnel.Status {
	spec, err := s.spec(row)
	if err != nil && !row.Enabled {
		spec = wattunnel.Spec{GUID: row.GUID, Owner: row.Owner, Revision: row.Revision, Restart: row.Restart, Name: row.Name, Deleted: row.Deleted}
		_ = json.Unmarshal([]byte(row.Config), &spec.Config)
		err = nil
	}
	status := wattunnel.Status{State: "error", Revision: row.Revision}
	if err == nil {
		var rt runtime.TunnelRuntime
		rt, err = tunnelRuntimeFor(row.NodeID)
		if err == nil && spec.Enabled && !spec.Deleted {
			var caps wattunnel.Capabilities
			caps, err = rt.TunnelCapabilities(ctx)
			if err == nil {
				err = checkTunnelCapabilities(spec.Config, caps)
			}
		}
		if err == nil {
			status, err = rt.ApplyTunnel(ctx, spec)
		}
	}
	if err != nil {
		status.State = "error"
		if row.NodeID != nil {
			status.State = "unavailable"
		}
		status.Reason = err.Error()
		status.Revision = row.Revision
	}
	tunnelStatuses[row.GUID] = status
	return status
}

func tunnelView(row model.ManagedTunnel) TunnelView {
	var cfg wattunnel.Config
	_ = json.Unmarshal([]byte(row.Config), &cfg)
	st, ok := tunnelStatuses[row.GUID]
	if !ok {
		st = wattunnel.Status{State: "pending", Revision: row.Revision}
	}
	return TunnelView{ID: row.ID, GUID: row.GUID, Name: row.Name, NodeID: row.NodeID, Revision: row.Revision, Enabled: row.Enabled, Managed: row.AuthOwner != "", Config: cfg, Status: st}
}

func (s *TunnelService) List() ([]TunnelView, error) {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	var rows []model.ManagedTunnel
	if err := database.GetDB().Where("deleted = ?", false).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]TunnelView, 0, len(rows))
	for _, row := range rows {
		out = append(out, tunnelView(row))
	}
	return out, nil
}

func (s *TunnelService) Save(ctx context.Context, id int, req TunnelMutation) (TunnelView, error) {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	db := database.GetDB()
	var row model.ManagedTunnel
	if id > 0 {
		if err := db.First(&row, id).Error; err != nil {
			return TunnelView{}, err
		}
		if row.Deleted || row.AuthOwner != "" {
			return TunnelView{}, errors.New("tunnel is deleted or managed by another panel")
		}
		if req.Revision != row.Revision {
			return TunnelView{}, errors.New("tunnel changed; reload before saving")
		}
		if !sameNode(row.NodeID, req.NodeID) {
			return TunnelView{}, errors.New("delete and recreate the tunnel to move it to another node")
		}
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 128 {
		return TunnelView{}, errors.New("tunnel name must contain 1..128 bytes")
	}
	if req.NodeID != nil {
		var node model.Node
		if err := db.First(&node, *req.NodeID).Error; err != nil {
			return TunnelView{}, errors.New("target node not found")
		}
	}
	key := req.Secret
	if req.Token != "" {
		if req.Secret != "" || req.GenerateKey {
			return TunnelView{}, errors.New("choose either a token, a key, or key generation")
		}
		cfg, k, err := wattunnel.DecodeToken(req.Token)
		if err != nil {
			return TunnelView{}, err
		}
		if len(req.Config.Ports) > 0 {
			cfg.Ports = req.Config.Ports
		}
		cfg.Bind = req.Config.Bind
		cfg.Listen = req.Config.Listen
		req.Config = cfg
		key = k
	}
	if req.GenerateKey {
		if key != "" {
			return TunnelView{}, errors.New("cannot generate and import a key together")
		}
		var err error
		key, err = wattunnel.NewKey()
		if err != nil {
			return TunnelView{}, err
		}
	}
	if key == "" && id > 0 {
		old, err := s.spec(row)
		if err != nil {
			return TunnelView{}, err
		}
		key = old.Secret
	}
	if err := wattunnel.ValidateKey(key); err != nil {
		return TunnelView{}, err
	}
	cfg := req.Config.Normalized()
	if err := cfg.Validate(); err != nil {
		return TunnelView{}, err
	}
	var count int64
	if err := db.Model(&model.ManagedTunnel{}).Where("secret <> ?", "").Count(&count).Error; err != nil {
		return TunnelView{}, err
	}
	codec, err := s.codec(count == 0)
	if err != nil {
		return TunnelView{}, errors.New("tunnel keyring unavailable")
	}
	if id == 0 {
		owner, err := (&SettingService{}).GetPanelGuid()
		if err != nil {
			return TunnelView{}, err
		}
		row.GUID = uuid.NewString()
		row.Owner = owner
	}
	cipher, err := codec.EncryptBound(tunnelAAD(row.GUID), key)
	if err != nil {
		return TunnelView{}, errors.New("cannot encrypt tunnel secret")
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return TunnelView{}, err
	}
	oldRevision := row.Revision
	row.Name = req.Name
	row.NodeID = req.NodeID
	row.Enabled = req.Enabled
	row.Config = string(raw)
	row.Secret = cipher
	row.Revision++
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := checkTunnelPorts(tx, row, cfg); err != nil {
			return err
		}
		if id == 0 {
			return tx.Create(&row).Error
		}
		result := tx.Model(&model.ManagedTunnel{}).Where("id = ? AND revision = ?", id, oldRevision).Select("*").Updates(&row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("concurrent tunnel update")
		}
		return nil
	})
	if err != nil {
		return TunnelView{}, err
	}
	s.apply(ctx, row)
	return tunnelView(row), nil
}

func checkTunnelCapabilities(c wattunnel.Config, caps wattunnel.Capabilities) error {
	if !caps.Supported {
		return errors.New("target platform does not support WatTunnel")
	}
	if caps.Error != "" {
		return errors.New(caps.Error)
	}
	if caps.Version != wattunnel.Version {
		return errors.New("target WatTunnel version mismatch")
	}
	if c.Carrier == "icmp" && !caps.RawSocket {
		return errors.New("ICMP requires Linux and CAP_NET_RAW")
	}
	return nil
}

func (s *TunnelService) Action(ctx context.Context, id int, action string) (TunnelView, error) {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	var row model.ManagedTunnel
	db := database.GetDB()
	if err := db.First(&row, id).Error; err != nil {
		return TunnelView{}, err
	}
	if row.AuthOwner != "" {
		return TunnelView{}, errors.New("tunnel is managed by another panel")
	}
	if row.Deleted && action != "delete" {
		return TunnelView{}, errors.New("tunnel was deleted")
	}
	switch action {
	case "start":
		row.Enabled = true
	case "stop":
		row.Enabled = false
	case "restart":
		row.Enabled = true
		row.Restart++
	case "delete":
		row.Enabled = false
		row.Deleted = true
	default:
		return TunnelView{}, errors.New("invalid tunnel action")
	}
	row.Revision++
	if err := db.Save(&row).Error; err != nil {
		return TunnelView{}, err
	}
	s.apply(ctx, row)
	return tunnelView(row), nil
}

func (s *TunnelService) Export(id int) (TunnelToken, error) {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	var row model.ManagedTunnel
	if err := database.GetDB().Where("deleted = ?", false).First(&row, id).Error; err != nil {
		return TunnelToken{}, err
	}
	if row.AuthOwner != "" {
		return TunnelToken{}, errors.New("export tokens from the owning panel")
	}
	spec, err := s.spec(row)
	if err != nil {
		return TunnelToken{}, err
	}
	token, err := wattunnel.EncodeToken(spec.Config, spec.Secret)
	return TunnelToken{Token: token}, err
}

func (s *TunnelService) Capabilities(ctx context.Context, nodeID *int) (wattunnel.Capabilities, error) {
	rt, err := tunnelRuntimeFor(nodeID)
	if err != nil {
		return wattunnel.Capabilities{}, err
	}
	return rt.TunnelCapabilities(ctx)
}

func (s *TunnelService) Sync(ctx context.Context, spec wattunnel.Spec, identity string) (wattunnel.Status, error) {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	if identity == "" {
		return wattunnel.Status{}, errors.New("node synchronization requires an API identity")
	}
	if guid, err := uuid.Parse(spec.GUID); err != nil || guid.String() != spec.GUID {
		return wattunnel.Status{}, errors.New("invalid tunnel GUID")
	}
	if len(spec.Owner) == 0 || len(spec.Owner) > 128 || spec.Revision < 1 {
		return wattunnel.Status{}, errors.New("invalid tunnel synchronization metadata")
	}
	owner, err := (&SettingService{}).GetPanelGuid()
	if err != nil {
		return wattunnel.Status{}, err
	}
	if owner == spec.Owner {
		return wattunnel.Status{}, errors.New("tunnel synchronization loop refused")
	}
	db := database.GetDB()
	var row model.ManagedTunnel
	err = db.Where("guid = ?", spec.GUID).First(&row).Error
	fresh := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !fresh {
		return wattunnel.Status{}, err
	}
	if !fresh {
		if row.AuthOwner != identity || row.Owner != spec.Owner || row.NodeID != nil {
			return wattunnel.Status{}, errors.New("tunnel belongs to a different owner")
		}
		if spec.Revision < row.Revision {
			return wattunnel.Status{}, errors.New("stale tunnel revision")
		}
		if spec.Revision == row.Revision {
			return s.apply(ctx, row), nil
		}
	}
	if !spec.Deleted {
		if err = spec.Config.Validate(); err != nil {
			return wattunnel.Status{}, err
		}
		if err = wattunnel.ValidateKey(spec.Secret); err != nil && (spec.Enabled || spec.Secret != "") {
			return wattunnel.Status{}, err
		}
	}
	row.GUID = spec.GUID
	row.Owner = spec.Owner
	row.AuthOwner = identity
	row.NodeID = nil
	row.Name = spec.Name
	row.Revision = spec.Revision
	row.Restart = spec.Restart
	row.Enabled = spec.Enabled
	row.Deleted = spec.Deleted
	if spec.Deleted {
		row.Secret = ""
		row.Config = "{}"
	} else {
		if spec.Secret != "" {
			var count int64
			if err = db.Model(&model.ManagedTunnel{}).Where("secret <> ?", "").Count(&count).Error; err != nil {
				return wattunnel.Status{}, err
			}
			codec, e := s.codec(count == 0)
			if e != nil && spec.Enabled {
				return wattunnel.Status{}, errors.New("node tunnel keyring unavailable")
			}
			if e == nil {
				row.Secret, err = codec.EncryptBound(tunnelAAD(row.GUID), spec.Secret)
				if err != nil {
					return wattunnel.Status{}, errors.New("cannot encrypt tunnel secret")
				}
			}
		}

		raw, e := json.Marshal(spec.Config.Normalized())
		if e != nil {
			return wattunnel.Status{}, e
		}
		row.Config = string(raw)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if !row.Deleted && row.Enabled {
			if e := checkTunnelPorts(tx, row, spec.Config); e != nil {
				return e
			}
		}
		return tx.Save(&row).Error
	})
	if err != nil {
		return wattunnel.Status{}, err
	}
	return s.apply(ctx, row), nil
}

func (s *TunnelService) Reconcile(ctx context.Context) error {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	var rows []model.ManagedTunnel
	if err := database.GetDB().Order("id").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		s.apply(ctx, row)
	}
	return nil
}

func checkTunnelPorts(db *gorm.DB, row model.ManagedTunnel, c wattunnel.Config) error {
	bindings := c.Bindings()
	if row.NodeID == nil {
		raw, err := (&SettingService{}).GetXrayConfigTemplate()
		if err != nil {
			return err
		}
		inbounds, err := tunnelTemplateInbounds(raw)
		if err != nil {
			return err
		}
		for _, b := range bindings {
			bits := transportTCP
			if b.Protocol == "udp" {
				bits = transportUDP
			}
			for _, inbound := range inbounds {
				if inbound.Port == b.Port && inboundTransports(inbound.Protocol, inbound.StreamSettings, inbound.Settings)&bits != 0 && listenOverlaps(inboundBindAddr(inbound), bindAddr{listen: b.Address}) {
					return fmt.Errorf("tunnel port %d conflicts with Xray template listener %s", b.Port, inbound.Tag)
				}
			}
		}
	}
	for i, b := range bindings {
		for _, other := range bindings[:i] {
			if b.Port == other.Port && b.Protocol == other.Protocol && listenOverlaps(bindAddr{listen: b.Address}, bindAddr{listen: other.Address}) {
				return errors.New("tunnel carrier and forwarding listeners overlap")
			}
		}
		ib := &model.Inbound{Port: b.Port, Listen: b.Address, NodeID: row.NodeID, Protocol: model.Tunnel, Settings: fmt.Sprintf(`{"allowedNetwork":%q}`, b.Protocol)}
		// The existing checker covers inbounds and derived relay listeners; tunnel rows are checked below.
		conflict, err := checkPortConflictWithoutTunnels(db, ib, 0)
		if err != nil {
			return err
		}
		if conflict != nil {
			return fmt.Errorf("tunnel port %d/%s conflicts with %s", b.Port, b.Protocol, conflict.Tag)
		}
		if row.NodeID == nil && b.Protocol == "tcp" {
			webPort, _ := (&SettingService{}).GetPort()
			webListen, _ := (&SettingService{}).GetListen()
			subPort, _ := (&SettingService{}).GetSubPort()
			subListen, _ := (&SettingService{}).GetSubListen()
			if b.Port == webPort && listenOverlaps(bindAddr{listen: b.Address}, bindAddr{listen: webListen}) || b.Port == subPort && listenOverlaps(bindAddr{listen: b.Address}, bindAddr{listen: subListen}) {
				return fmt.Errorf("port %d is reserved by the panel", b.Port)
			}
		}
	}
	var others []model.ManagedTunnel
	if err := db.Where("deleted = ? AND guid <> ?", false, row.GUID).Find(&others).Error; err != nil {
		return err
	}
	for _, other := range others {
		if !sameNode(row.NodeID, other.NodeID) {
			continue
		}
		var cfg wattunnel.Config
		if err := json.Unmarshal([]byte(other.Config), &cfg); err != nil {
			return err
		}
		for _, a := range bindings {
			for _, b := range cfg.Bindings() {
				if a.Port == b.Port && a.Protocol == b.Protocol && listenOverlaps(bindAddr{listen: a.Address}, bindAddr{listen: b.Address}) {
					return fmt.Errorf("port %d/%s conflicts with tunnel %s", a.Port, a.Protocol, other.Name)
				}
			}
		}
	}
	return nil
}

func tunnelPortConflict(db *gorm.DB, ib *model.Inbound) (*portConflictDetail, error) {
	var rows []model.ManagedTunnel
	if err := db.Where("deleted = ?", false).Find(&rows).Error; err != nil {
		return nil, err
	}
	bits := inboundTransports(ib.Protocol, ib.StreamSettings, ib.Settings)
	for _, row := range rows {
		if !sameNode(row.NodeID, ib.NodeID) {
			continue
		}
		var c wattunnel.Config
		if err := json.Unmarshal([]byte(row.Config), &c); err != nil {
			return nil, err
		}
		for _, b := range c.Bindings() {
			want := transportTCP
			if b.Protocol == "udp" {
				want = transportUDP
			}
			if bits&want != 0 && ib.Port == b.Port && listenOverlaps(inboundBindAddr(ib), bindAddr{listen: b.Address}) {
				return &portConflictDetail{Tag: "tunnel:" + row.Name, Listen: b.Address, Port: b.Port, Transports: want}, nil
			}
		}
	}
	return nil, nil
}

func checkPanelTunnelPort(db *gorm.DB, listen string, port int) error {
	conflict, err := tunnelPortConflict(db, &model.Inbound{Listen: listen, Port: port, Protocol: model.Tunnel, Settings: `{"allowedNetwork":"tcp"}`})
	if err != nil {
		return err
	}
	if conflict != nil {
		return fmt.Errorf("panel listener conflicts with %s", conflict.Tag)
	}
	return nil
}

func tunnelTemplateInbounds(raw string) ([]*model.Inbound, error) {
	var cfg xray.Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	inbounds := make([]*model.Inbound, 0, len(cfg.InboundConfigs))
	for _, inbound := range cfg.InboundConfigs {
		if inbound.Port <= 0 {
			continue
		}
		var listen string
		_ = json.Unmarshal(inbound.Listen, &listen)
		inbounds = append(inbounds, &model.Inbound{Tag: inbound.Tag, Port: inbound.Port, Listen: listen, Protocol: model.Protocol(inbound.Protocol), Settings: string(inbound.Settings), StreamSettings: string(inbound.StreamSettings)})
	}
	return inbounds, nil
}

func checkTemplateTunnelPorts(raw string) error {
	inbounds, err := tunnelTemplateInbounds(raw)
	if err != nil {
		return err
	}
	for _, inbound := range inbounds {
		conflict, err := tunnelPortConflict(database.GetDB(), inbound)
		if err != nil {
			return err
		}
		if conflict != nil {
			return fmt.Errorf("Xray template listener conflicts with %s", conflict.Tag)
		}
	}
	return nil
}
