package job

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	ldaputil "github.com/mhsanaei/3x-ui/v3/internal/util/ldap"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var DefaultTruthyValues = []string{"true", "1", "yes", "on"}

// Share of the previous successful fetch a new one must still return to be
// trusted: a sudden collapse means a broken directory far more often than churn.
const ldapAutoDeleteMinRetainPercent = 50

type LdapSyncJob struct {
	settingService service.SettingService
	inboundService service.InboundService
	clientService  service.ClientService
	xrayService    service.XrayService
	lastFlagCount  atomic.Int64
}

// --- Helper functions for mustGet ---
func mustGetString(fn func() (string, error)) string {
	v, err := fn()
	if err != nil {
		panic(err)
	}
	return v
}

func mustGetInt(fn func() (int, error)) int {
	v, err := fn()
	if err != nil {
		panic(err)
	}
	return v
}

func mustGetBool(fn func() (bool, error)) bool {
	v, err := fn()
	if err != nil {
		panic(err)
	}
	return v
}

func mustGetStringOr(fn func() (string, error), fallback string) string {
	v, err := fn()
	if err != nil || v == "" {
		return fallback
	}
	return v
}

func NewLdapSyncJob() *LdapSyncJob {
	return new(LdapSyncJob)
}

func (j *LdapSyncJob) Run() {
	logger.Info("LDAP sync job started")

	enabled, err := j.settingService.GetLdapEnable()
	if err != nil || !enabled {
		logger.Warning("LDAP disabled or failed to fetch flag")
		return
	}

	// --- LDAP fetch ---
	cfg := ldaputil.Config{
		Host:               mustGetString(j.settingService.GetLdapHost),
		Port:               mustGetInt(j.settingService.GetLdapPort),
		UseTLS:             mustGetBool(j.settingService.GetLdapUseTLS),
		InsecureSkipVerify: mustGetBool(j.settingService.GetLdapInsecureSkipVerify),
		BindDN:             mustGetString(j.settingService.GetLdapBindDN),
		Password:           mustGetString(j.settingService.GetLdapPassword),
		BaseDN:             mustGetString(j.settingService.GetLdapBaseDN),
		UserFilter:         mustGetString(j.settingService.GetLdapUserFilter),
		UserAttr:           mustGetString(j.settingService.GetLdapUserAttr),
		FlagField:          mustGetStringOr(j.settingService.GetLdapFlagField, mustGetString(j.settingService.GetLdapVlessField)),
		TruthyVals:         truthyValuesOrDefault(mustGetString(j.settingService.GetLdapTruthyValues)),
		Invert:             mustGetBool(j.settingService.GetLdapInvertFlag),
	}

	flags, err := ldaputil.FetchVlessFlags(cfg)
	if err != nil {
		logger.Warning("LDAP fetch failed:", err)
		return
	}
	logger.Infof("Fetched %d LDAP flags", len(flags))

	// --- Load all inbounds and all clients once ---
	inboundTags := splitCsv(mustGetString(j.settingService.GetLdapInboundTags))
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Failed to get inbounds:", err)
		return
	}

	allClients := map[string]*model.Client{}  // email -> client
	inboundMap := map[string]*model.Inbound{} // tag -> inbound
	for _, ib := range inbounds {
		inboundMap[ib.Tag] = ib
		clients, _ := j.inboundService.GetClients(ib)
		for i := range clients {
			allClients[clients[i].Email] = &clients[i]
		}
	}

	// --- Prepare batch operations ---
	autoCreate := mustGetBool(j.settingService.GetLdapAutoCreate)
	defGB := mustGetInt(j.settingService.GetLdapDefaultTotalGB)
	defExpiryDays := mustGetInt(j.settingService.GetLdapDefaultExpiryDays)
	defLimitIP := mustGetInt(j.settingService.GetLdapDefaultLimitIP)

	resolvedInboundIds := make([]int, 0, len(inboundTags))
	resolvedTags := make([]string, 0, len(inboundTags))
	for _, tag := range inboundTags {
		ib := inboundMap[tag]
		if ib == nil {
			logger.Warningf("LDAP inbound tag %s does not match any inbound", tag)
			continue
		}
		resolvedInboundIds = append(resolvedInboundIds, ib.Id)
		resolvedTags = append(resolvedTags, tag)
	}

	clientsToCreate := []model.Client{}
	var clientsToEnable, clientsToDisable []string

	for email, allowed := range flags {
		existing := allClients[email]
		if existing == nil {
			if allowed && autoCreate {
				clientsToCreate = append(clientsToCreate, j.buildClient(email, defGB, defExpiryDays, defLimitIP))
			}
			continue
		}
		if len(resolvedTags) == 0 {
			continue
		}
		if allowed && !existing.Enable {
			clientsToEnable = append(clientsToEnable, email)
		} else if !allowed && existing.Enable {
			clientsToDisable = append(clientsToDisable, email)
		}
	}

	j.createClients(clientsToCreate, resolvedInboundIds, resolvedTags)

	// --- Execute enable/disable batch ---
	j.batchSetEnable(clientsToEnable, true)
	j.batchSetEnable(clientsToDisable, false)

	// --- Auto delete clients not in LDAP ---
	autoDelete := mustGetBool(j.settingService.GetLdapAutoDelete)
	if autoDelete && j.autoDeleteSafeForFetch(len(flags)) {
		ldapEmailSet := map[string]struct{}{}
		for e := range flags {
			ldapEmailSet[e] = struct{}{}
		}
		for _, tag := range inboundTags {
			j.deleteClientsNotInLDAP(tag, ldapEmailSet)
		}
	}
	j.lastFlagCount.Store(int64(len(flags)))
}

// FetchVlessFlags returns (empty, nil) when the bind succeeds but the search
// yields nothing — a renamed OU, a lost read grant — which is not "all gone".
func (j *LdapSyncJob) autoDeleteSafeForFetch(fetched int) bool {
	if fetched == 0 {
		logger.Warning("LDAP auto-delete skipped: directory returned no usable users")
		return false
	}
	previous := j.lastFlagCount.Load()
	if previous > 0 && int64(fetched)*100 < previous*ldapAutoDeleteMinRetainPercent {
		logger.Warningf("LDAP auto-delete skipped: fetched %d users, previous successful sync saw %d (below %d%% retention)",
			fetched, previous, ldapAutoDeleteMinRetainPercent)
		return false
	}
	return true
}

func truthyValuesOrDefault(s string) []string {
	if vals := splitCsv(s); len(vals) > 0 {
		return vals
	}
	return DefaultTruthyValues
}

func splitCsv(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// buildClient creates a new client for auto-create; ClientService.Create fills per-protocol credentials
func (j *LdapSyncJob) buildClient(email string, defGB, defExpiryDays, defLimitIP int) model.Client {
	c := model.Client{
		Email:   email,
		Enable:  true,
		LimitIP: defLimitIP,
		TotalGB: int64(defGB) * 1024 * 1024 * 1024,
	}
	if defExpiryDays > 0 {
		c.ExpiryTime = time.Now().Add(time.Duration(defExpiryDays) * 24 * time.Hour).UnixMilli()
	}
	return c
}

// createClients adds each new LDAP client once, attached to every configured inbound
func (j *LdapSyncJob) createClients(newClients []model.Client, inboundIds []int, tags []string) {
	if len(newClients) == 0 || len(inboundIds) == 0 {
		return
	}
	tagList := strings.Join(tags, ",")
	created := 0
	restartNeeded := false
	for _, c := range newClients {
		nr, err := j.clientService.Create(&j.inboundService, &service.ClientCreatePayload{Client: c, InboundIds: inboundIds})
		// Read before the error check: a partly-applied create still committed
		// clients on the inbounds that succeeded, and those need the restart.
		if nr {
			restartNeeded = true
		}
		if err != nil {
			logger.Warningf("Failed to add client %s for tags %s: %v", c.Email, tagList, err)
			continue
		}
		created++
	}
	if restartNeeded {
		j.xrayService.SetToNeedRestart()
	}
	if created == 0 {
		return
	}
	logger.Infof("LDAP auto-create: %d clients for %s", created, tagList)
}

// batchSetEnable takes the bulk path: per-user calls held each inbound's lock through
// its node push, so users sharing a hung node inbound queued one push timeout apiece.
func (j *LdapSyncJob) batchSetEnable(emails []string, enable bool) {
	if len(emails) == 0 {
		return
	}
	result, needRestart, err := j.clientService.BulkSetEnable(&j.inboundService, emails, enable)
	if err != nil {
		logger.Warningf("Batch set enable=%v failed: %v", enable, err)
	}
	for _, skipped := range result.Skipped {
		logger.Warningf("Batch set enable failed for %s: %s", skipped.Email, skipped.Reason)
	}
	if result.Changed > 0 {
		logger.Infof("Batch set enable=%v for %d clients", enable, result.Changed)
	}
	if needRestart {
		j.xrayService.SetToNeedRestart()
	}
}

// deleteClientsNotInLDAP detaches clients not in LDAP, one bulk detach per inbound
func (j *LdapSyncJob) deleteClientsNotInLDAP(inboundTag string, ldapEmails map[string]struct{}) {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Failed to get inbounds for deletion:", err)
		return
	}

	restartNeeded := false

	for _, ib := range inbounds {
		if ib.Tag != inboundTag {
			continue
		}
		clients, err := j.inboundService.GetClients(ib)
		if err != nil {
			logger.Warningf("Failed to get clients for inbound %s: %v", ib.Tag, err)
			continue
		}

		// Collect clients for deletion
		toDelete := []model.Client{}
		for _, c := range clients {
			if _, ok := ldapEmails[c.Email]; !ok {
				toDelete = append(toDelete, c)
			}
		}

		if len(toDelete) == 0 {
			continue
		}

		emails := make([]string, len(toDelete))
		for i, c := range toDelete {
			emails[i] = c.Email
		}
		result, nr, err := j.clientService.BulkDetach(&j.inboundService, emails, []int{ib.Id})
		if err != nil {
			logger.Warningf("Failed to delete clients from inbound id=%d(tag=%s): %v", ib.Id, ib.Tag, err)
			continue
		}
		for _, msg := range result.Errors {
			logger.Warningf("Failed to delete client from inbound id=%d(tag=%s): %s", ib.Id, ib.Tag, msg)
		}
		for _, email := range result.Detached {
			logger.Infof("Deleted client %s from inbound id=%d(tag=%s)", email, ib.Id, ib.Tag)
		}
		if nr {
			restartNeeded = true
		}
	}

	if restartNeeded {
		j.xrayService.SetToNeedRestart()
		logger.Info("Xray restart scheduled after batch deletion")
	}
}
