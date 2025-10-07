package job

import (
	"time"

	"strings"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	ldaputil "github.com/mhsanaei/3x-ui/v2/util/ldap"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"strconv"

	"github.com/google/uuid"
)

var DefaultTruthyValues = []string{"true", "1", "yes", "on"}

type LdapSyncJob struct {
	settingService service.SettingService
	inboundService service.InboundService
	xrayService    service.XrayService
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
		Host:       mustGetString(j.settingService.GetLdapHost),
		Port:       mustGetInt(j.settingService.GetLdapPort),
		UseTLS:     mustGetBool(j.settingService.GetLdapUseTLS),
		BindDN:     mustGetString(j.settingService.GetLdapBindDN),
		Password:   mustGetString(j.settingService.GetLdapPassword),
		BaseDN:     mustGetString(j.settingService.GetLdapBaseDN),
		UserFilter: mustGetString(j.settingService.GetLdapUserFilter),
		UserAttr:   mustGetString(j.settingService.GetLdapUserAttr),
		FlagField:  mustGetStringOr(j.settingService.GetLdapFlagField, mustGetString(j.settingService.GetLdapVlessField)),
		TruthyVals: splitCsv(mustGetString(j.settingService.GetLdapTruthyValues)),
		Invert:     mustGetBool(j.settingService.GetLdapInvertFlag),
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

	clientsToCreate := map[string][]model.Client{} // tag -> []new clients
	clientsToEnable := map[string][]string{}       // tag -> []email
	clientsToDisable := map[string][]string{}      // tag -> []email

	for email, allowed := range flags {
		exists := allClients[email] != nil
		for _, tag := range inboundTags {
			if !exists && allowed && autoCreate {
				newClient := j.buildClient(inboundMap[tag], email, defGB, defExpiryDays, defLimitIP)
				clientsToCreate[tag] = append(clientsToCreate[tag], newClient)
			} else if exists {
				if allowed && !allClients[email].Enable {
					clientsToEnable[tag] = append(clientsToEnable[tag], email)
				} else if !allowed && allClients[email].Enable {
					clientsToDisable[tag] = append(clientsToDisable[tag], email)
				}
			}
		}
	}

	// --- Execute batch create ---
	for tag, newClients := range clientsToCreate {
		if len(newClients) == 0 {
			continue
		}
		payload := &model.Inbound{Id: inboundMap[tag].Id}
		payload.Settings = j.clientsToJSON(newClients)
		if _, err := j.inboundService.AddInboundClient(payload); err != nil {
			logger.Warningf("Failed to add clients for tag %s: %v", tag, err)
		} else {
			logger.Infof("LDAP auto-create: %d clients for %s", len(newClients), tag)
			j.xrayService.SetToNeedRestart()
		}
	}

	// --- Execute enable/disable batch ---
	for tag, emails := range clientsToEnable {
		j.batchSetEnable(inboundMap[tag], emails, true)
	}
	for tag, emails := range clientsToDisable {
		j.batchSetEnable(inboundMap[tag], emails, false)
	}

	// --- Auto delete clients not in LDAP ---
	autoDelete := mustGetBool(j.settingService.GetLdapAutoDelete)
	if autoDelete {
		ldapEmailSet := map[string]struct{}{}
		for e := range flags {
			ldapEmailSet[e] = struct{}{}
		}
		for _, tag := range inboundTags {
			j.deleteClientsNotInLDAP(tag, ldapEmailSet)
		}
	}
}

func splitCsv(s string) []string {
	if s == "" {
		return DefaultTruthyValues
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

// buildClient creates a new client for auto-create
func (j *LdapSyncJob) buildClient(ib *model.Inbound, email string, defGB, defExpiryDays, defLimitIP int) model.Client {
	c := model.Client{
		Email:   email,
		Enable:  true,
		LimitIP: defLimitIP,
		TotalGB: int64(defGB),
	}
	if defExpiryDays > 0 {
		c.ExpiryTime = time.Now().Add(time.Duration(defExpiryDays) * 24 * time.Hour).UnixMilli()
	}
	switch ib.Protocol {
	case model.Trojan, model.Shadowsocks:
		c.Password = uuid.NewString()
	default:
		c.ID = uuid.NewString()
	}
	return c
}

// batchSetEnable enables/disables clients in batch through a single call
func (j *LdapSyncJob) batchSetEnable(ib *model.Inbound, emails []string, enable bool) {
	if len(emails) == 0 {
		return
	}

	// Prepare JSON for mass update
	clients := make([]model.Client, 0, len(emails))
	for _, email := range emails {
		clients = append(clients, model.Client{
			Email:  email,
			Enable: enable,
		})
	}

	payload := &model.Inbound{
		Id:       ib.Id,
		Settings: j.clientsToJSON(clients),
	}

	// Use a single AddInboundClient call to update enable
	if _, err := j.inboundService.AddInboundClient(payload); err != nil {
		logger.Warningf("Batch set enable failed for inbound %s: %v", ib.Tag, err)
		return
	}

	logger.Infof("Batch set enable=%v for %d clients in inbound %s", enable, len(emails), ib.Tag)
	j.xrayService.SetToNeedRestart()
}

// deleteClientsNotInLDAP deletes clients not in LDAP using batches and a single restart
func (j *LdapSyncJob) deleteClientsNotInLDAP(inboundTag string, ldapEmails map[string]struct{}) {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Failed to get inbounds for deletion:", err)
		return
	}

	batchSize := 50 //  clients in 1 batch
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

		// Delete in batches
		for i := 0; i < len(toDelete); i += batchSize {
			end := i + batchSize
			if end > len(toDelete) {
				end = len(toDelete)
			}
			batch := toDelete[i:end]

			for _, c := range batch {
				var clientKey string
				switch ib.Protocol {
				case model.Trojan:
					clientKey = c.Password
				case model.Shadowsocks:
					clientKey = c.Email
				default: // vless/vmess
					clientKey = c.ID
				}

				if _, err := j.inboundService.DelInboundClient(ib.Id, clientKey); err != nil {
					logger.Warningf("Failed to delete client %s from inbound id=%d(tag=%s): %v",
						c.Email, ib.Id, ib.Tag, err)
				} else {
					logger.Infof("Deleted client %s from inbound id=%d(tag=%s)",
						c.Email, ib.Id, ib.Tag)
					// do not restart here
					restartNeeded = true
				}
			}
		}
	}

	// One time after all batches
	if restartNeeded {
		j.xrayService.SetToNeedRestart()
		logger.Info("Xray restart scheduled after batch deletion")
	}
}

// clientsToJSON serializes an array of clients to JSON
func (j *LdapSyncJob) clientsToJSON(clients []model.Client) string {
	b := strings.Builder{}
	b.WriteString("{\"clients\":[")
	for i, c := range clients {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(j.clientToJSON(c))
	}
	b.WriteString("]}")
	return b.String()
}

// ensureClientExists adds client with defaults to inbound tag if not present
func (j *LdapSyncJob) ensureClientExists(inboundTag string, email string, defGB int, defExpiryDays int, defLimitIP int) {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("ensureClientExists: get inbounds failed:", err)
		return
	}
	var target *model.Inbound
	for _, ib := range inbounds {
		if ib.Tag == inboundTag {
			target = ib
			break
		}
	}
	if target == nil {
		logger.Debugf("ensureClientExists: inbound tag %s not found", inboundTag)
		return
	}
	// check if email already exists in this inbound
	clients, err := j.inboundService.GetClients(target)
	if err == nil {
		for _, c := range clients {
			if c.Email == email {
				return
			}
		}
	}

	// build new client according to protocol
	newClient := model.Client{
		Email:   email,
		Enable:  true,
		LimitIP: defLimitIP,
		TotalGB: int64(defGB),
	}
	if defExpiryDays > 0 {
		newClient.ExpiryTime = time.Now().Add(time.Duration(defExpiryDays) * 24 * time.Hour).UnixMilli()
	}

	switch target.Protocol {
	case model.Trojan:
		newClient.Password = uuid.NewString()
	case model.Shadowsocks:
		newClient.Password = uuid.NewString()
	default: // VMESS/VLESS and others using ID
		newClient.ID = uuid.NewString()
	}

	// prepare inbound payload with only the new client
	payload := &model.Inbound{Id: target.Id}
	payload.Settings = `{"clients":[` + j.clientToJSON(newClient) + `]}`

	if _, err := j.inboundService.AddInboundClient(payload); err != nil {
		logger.Warning("ensureClientExists: add client failed:", err)
	} else {
		j.xrayService.SetToNeedRestart()
		logger.Infof("LDAP auto-create: %s in %s", email, inboundTag)
	}
}

// clientToJSON serializes minimal client fields to JSON object string without extra deps
func (j *LdapSyncJob) clientToJSON(c model.Client) string {
	// construct minimal JSON manually to avoid importing json for simple case
	b := strings.Builder{}
	b.WriteString("{")
	if c.ID != "" {
		b.WriteString("\"id\":\"")
		b.WriteString(c.ID)
		b.WriteString("\",")
	}
	if c.Password != "" {
		b.WriteString("\"password\":\"")
		b.WriteString(c.Password)
		b.WriteString("\",")
	}
	b.WriteString("\"email\":\"")
	b.WriteString(c.Email)
	b.WriteString("\",")
	b.WriteString("\"enable\":")
	if c.Enable {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteString(",")
	b.WriteString("\"limitIp\":")
	b.WriteString(strconv.Itoa(c.LimitIP))
	b.WriteString(",")
	b.WriteString("\"totalGB\":")
	b.WriteString(strconv.FormatInt(c.TotalGB, 10))
	if c.ExpiryTime > 0 {
		b.WriteString(",\"expiryTime\":")
		b.WriteString(strconv.FormatInt(c.ExpiryTime, 10))
	}
	b.WriteString("}")
	return b.String()
}
