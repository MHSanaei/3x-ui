package job

import (
    "time"

    "x-ui/database/model"
    "x-ui/logger"
    ldaputil "x-ui/util/ldap"
    "x-ui/web/service"
    "strings"

    "github.com/google/uuid"
    "strconv"
)

type LdapSyncJob struct {
    settingService service.SettingService
    inboundService service.InboundService
    xrayService    service.XrayService
}

func NewLdapSyncJob() *LdapSyncJob {
    return new(LdapSyncJob)
}

func (j *LdapSyncJob) Run() {
    enabled, err := j.settingService.GetLdapEnable()
    if err != nil || !enabled {
        return
    }
    host, _ := j.settingService.GetLdapHost()
    port, _ := j.settingService.GetLdapPort()
    useTLS, _ := j.settingService.GetLdapUseTLS()
    bindDN, _ := j.settingService.GetLdapBindDN()
    password, _ := j.settingService.GetLdapPassword()
    baseDN, _ := j.settingService.GetLdapBaseDN()
    userFilter, _ := j.settingService.GetLdapUserFilter()
    userAttr, _ := j.settingService.GetLdapUserAttr()
    // Generic flag settings
    flagField, _ := j.settingService.GetLdapFlagField()
    if flagField == "" {
        flagField, _ = j.settingService.GetLdapVlessField()
    }
    truthyCSV, _ := j.settingService.GetLdapTruthyValues()
    invert, _ := j.settingService.GetLdapInvertFlag()

    cfg := ldaputil.Config{
        Host: host,
        Port: port,
        UseTLS: useTLS,
        BindDN: bindDN,
        Password: password,
        BaseDN: baseDN,
        UserFilter: userFilter,
        UserAttr: userAttr,
        FlagField: flagField,
        TruthyVals: splitCsv(truthyCSV),
        Invert: invert,
    }

    flags, err := ldaputil.FetchVlessFlags(cfg)
    if err != nil {
        logger.Warning("LDAP sync failed:", err)
        return
    }

    // Settings for auto create/delete
    inboundTagsCSV, _ := j.settingService.GetLdapInboundTags()
    autoCreate, _ := j.settingService.GetLdapAutoCreate()
    autoDelete, _ := j.settingService.GetLdapAutoDelete()
    defGB, _ := j.settingService.GetLdapDefaultTotalGB()
    defExpiryDays, _ := j.settingService.GetLdapDefaultExpiryDays()
    defLimitIP, _ := j.settingService.GetLdapDefaultLimitIP()
    inboundTags := splitCsv(inboundTagsCSV)

    // Build a set of LDAP emails for delete checks
    ldapEmails := map[string]struct{}{}
    for email := range flags { ldapEmails[email] = struct{}{} }

    // Create/enable according to LDAP flags
    for email, allowed := range flags {
        // ensure exists if allowed and autoCreate
        if allowed && autoCreate && len(inboundTags) > 0 {
            for _, tag := range inboundTags {
                j.ensureClientExists(tag, email, defGB, defExpiryDays, defLimitIP)
            }
        }
        changed, needRestart, err := j.inboundService.SetClientEnableByEmail(email, allowed)
        if err != nil {
            logger.Debugf("LDAP sync skip email=%s err=%v", email, err)
            continue
        }
        if changed {
            logger.Infof("LDAP sync: %s -> enable=%v", email, allowed)
        }
        if needRestart {
            j.xrayService.SetToNeedRestart()
        }
    }

    // Auto delete: find clients in targeted inbounds that are not in LDAP
    if autoDelete && len(inboundTags) > 0 {
        for _, tag := range inboundTags {
            j.deleteClientsNotInLDAP(tag, ldapEmails)
        }
    }
}

func splitCsv(s string) []string {
    if s == "" {
        return []string{"true", "1", "yes", "on"}
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

// deleteClientsNotInLDAP removes clients from inbound tag that are not in ldapEmails
func (j *LdapSyncJob) deleteClientsNotInLDAP(inboundTag string, ldapEmails map[string]struct{}) {
    inbounds, err := j.inboundService.GetAllInbounds()
    if err != nil {
        return
    }
    for _, ib := range inbounds {
        if ib.Tag != inboundTag {
            continue
        }
        clients, err := j.inboundService.GetClients(ib)
        if err != nil {
            continue
        }
        for _, c := range clients {
            if _, ok := ldapEmails[c.Email]; !ok {
                // determine clientId per protocol
                clientId := c.ID
                if ib.Protocol == model.Trojan {
                    clientId = c.Password
                } else if ib.Protocol == model.Shadowsocks {
                    clientId = c.Email
                }
                needRestart, err := j.inboundService.DelInboundClient(ib.Id, clientId)
                if err == nil {
                    if needRestart {
                        j.xrayService.SetToNeedRestart()
                    }
                    logger.Infof("LDAP auto-delete: %s from %s", c.Email, inboundTag)
                }
            }
        }
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
    if c.Enable { b.WriteString("true") } else { b.WriteString("false") }
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


