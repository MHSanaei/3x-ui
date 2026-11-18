package service

import (
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/crypto/nodetoken"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/ha"
)

// errHALocalInbound refuses inbounds the HA panel itself would serve: they
// would move to another IP with every failover (docs/master-failover.md).
var errHALocalInbound = fmt.Errorf("in HA mode the panel hosts no inbounds of its own: create the inbound on a node")

func haEnabled() bool { return ha.Active() != nil }

// CheckHAReady verifies, read-only, that this database can run under HA: no
// local inbounds, and every node token decrypts with this machine's key.
func CheckHAReady() error {
	db := database.GetDB()
	var local int64
	if err := db.Model(&model.Inbound{}).Where("node_id IS NULL").Count(&local).Error; err != nil {
		return fmt.Errorf("count local inbounds: %w", err)
	}
	if local > 0 {
		return fmt.Errorf("%d local inbound(s) found; move them to a node on the same machine first", local)
	}
	var nodes []model.Node
	if err := db.Select("id", "name", "api_token").Where("api_token <> ''").Find(&nodes).Error; err != nil {
		return fmt.Errorf("read node tokens: %w", err)
	}
	for _, n := range nodes {
		// Off mode passes ciphertext through as if it were the token itself.
		if nodetoken.IsEncrypted(n.ApiToken) && !nodetoken.Enabled() {
			return fmt.Errorf("node token of %q is encrypted but node-token encryption is off on this machine (all HA machines need the same node-token key)", n.Name)
		}
		if _, err := nodetoken.Decrypt(n.Id, n.ApiToken); err != nil {
			return fmt.Errorf("node token of %q does not decrypt with this machine's key (all HA machines need the same node-token key): %w", n.Name, err)
		}
	}
	return nil
}

// PrepareHAMaster runs once when this panel becomes master. Marking every node
// dirty forces a full reconcile instead of trusting the old master's memory.
func PrepareHAMaster() error {
	if err := CheckHAReady(); err != nil {
		return err
	}
	err := database.GetDB().Model(&model.Node{}).Where("1 = 1").Updates(map[string]any{
		"config_dirty":    true,
		"config_dirty_at": time.Now().UnixMilli(),
	}).Error
	if err != nil {
		return fmt.Errorf("mark nodes dirty: %w", err)
	}
	return nil
}
