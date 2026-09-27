package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// IPWithTimestamp tracks an IP address with its last seen timestamp
type IPWithTimestamp struct {
	IP        string `json:"ip"`
	Timestamp int64  `json:"timestamp"`
}

// CheckClientIpJob monitors client IP addresses and manages IP blocking based
// on configured limits. The per-client IPs come from the core's online-stats
// API; no access log is involved. On a core too old to expose that API the job
// simply skips the run (the bundled core always supports it).
type CheckClientIpJob struct {
	bannedSeen  map[string]int64
	xrayService service.XrayService
	allowlist   ipLimitAllowlist
	lastIpPrune int64
}

var job *CheckClientIpJob

const defaultXrayAPIPort = 62789

const ipStaleAfterSeconds = int64(30 * 60)

// pruneStaleIpRows cadence; the scan itself cannot prune offline clients' rows.
const ipPruneIntervalSeconds = int64(5 * 60)

// NewCheckClientIpJob creates a new client IP monitoring job instance.
func NewCheckClientIpJob() *CheckClientIpJob {
	job = new(CheckClientIpJob)
	return job
}

func (j *CheckClientIpJob) Run() {
	j.pruneStaleIpRows()
	observed, apiMode := j.collectFromOnlineAPI()
	if !apiMode {
		// xray is down or predates the online-stats API. There is no access-log
		// fallback anymore, so there is nothing to do this run.
		logger.Debug("[LimitIP] online-stats API unavailable this run; skipping")
		return
	}

	if !isFail2BanEnabled() {
		return
	}

	hasLimit := j.hasLimitIp()
	f2bInstalled := false
	if hasLimit {
		f2bInstalled = j.checkFail2BanInstalled()
	}
	// Read only when the limit is actually applied: this runs every 10s and
	// most panels carry no IP limit at all.
	enforce := j.resolveEnforce(hasLimit, f2bInstalled)
	if enforce {
		j.allowlist = j.loadAllowlist()
	}
	j.processObserved(observed, enforce, true)
}

// resolveEnforce decides whether limits can actually be enforced this run.
// Without fail2ban on a platform that needs it the limit can't be applied, so
// enforcement is skipped (the panel resets these limits to 0 on upgrade and
// disables the field, so this is normally a no-op).
func (j *CheckClientIpJob) resolveEnforce(hasLimit, f2bInstalled bool) bool {
	if hasLimit && runtime.GOOS != "windows" && !f2bInstalled {
		return false
	}
	return hasLimit
}

// collectFromOnlineAPI builds per-email IP observations (email -> ip ->
// last-seen unix seconds) from the core's online-stats API. ok=false means the
// API is unavailable — xray not running, an older core, or a transient gRPC
// failure — and the caller skips the run (there is no access-log fallback).
func (j *CheckClientIpJob) collectFromOnlineAPI() (map[string]map[string]int64, bool) {
	onlineUsers, ok, err := j.xrayService.GetOnlineUsers()
	if err != nil {
		logger.Debug("[LimitIP] online-stats API unavailable this run:", err)
		return nil, false
	}
	if !ok {
		return nil, false
	}
	now := time.Now().Unix()
	observed := make(map[string]map[string]int64, len(onlineUsers))
	for _, user := range onlineUsers {
		for _, entry := range user.IPs {
			// No localhost guard needed here: the core's OnlineMap.AddIP drops
			// 127.0.0.1/[::1] itself, so they never reach this list.
			ts := entry.LastSeen
			if ts <= 0 {
				ts = now
			}
			if _, exists := observed[user.Email]; !exists {
				observed[user.Email] = make(map[string]int64)
			}
			if existing, seen := observed[user.Email][entry.IP]; !seen || ts > existing {
				observed[user.Email][entry.IP] = ts
			}
		}
	}
	return observed, true
}

// hasLimitIp reports whether any client carries an IP limit. It probes the
// normalized clients table (limit_ip is synced there by SyncInbound and the
// legacy seeder), replacing the old `settings LIKE '%limitIp%'` scan that
// loaded and JSON-parsed every inbound's settings blob on each 10s run.
func (j *CheckClientIpJob) hasLimitIp() bool {
	db := database.GetDB()
	var probe int64
	err := db.Model(&model.ClientRecord{}).Where("limit_ip > 0").Limit(1).Count(&probe).Error
	return err == nil && probe > 0
}

// loadAllowlist reads the operator's trusted addresses once per scan; a bad
// read leaves the list empty, which enforces the limit as before rather than
// silently exempting everyone.
func (j *CheckClientIpJob) loadAllowlist() ipLimitAllowlist {
	raw, err := (&service.SettingService{}).GetIpLimitAllowlist()
	if err != nil {
		logger.Warning("[LimitIP] could not read the allowlist, enforcing without it:", err)
		return ipLimitAllowlist{}
	}
	return parseIpLimitAllowlist(raw)
}

const ipScanChunk = 400

func chunkEmails(s []string, size int) [][]string {
	if len(s) == 0 {
		return nil
	}
	chunks := make([][]string, 0, (len(s)+size-1)/size)
	for size < len(s) {
		s, chunks = s[size:], append(chunks, s[:size])
	}
	return append(chunks, s)
}

// loadClientLimits maps each observed email to its clients.limit_ip in a few
// chunked queries, replacing the per-email settings-JSON parse that previously
// resolved the limit.
func (j *CheckClientIpJob) loadClientLimits(emails []string) map[string]int {
	db := database.GetDB()
	out := make(map[string]int, len(emails))
	for _, batch := range chunkEmails(emails, ipScanChunk) {
		var rows []struct {
			Email   string
			LimitIp int
		}
		if err := db.Model(&model.ClientRecord{}).
			Select("email, limit_ip").
			Where("email IN ?", batch).
			Scan(&rows).Error; err != nil {
			j.checkError(err)
			continue
		}
		for _, r := range rows {
			out[r.Email] = r.LimitIp
		}
	}
	return out
}

// loadInboundsByEmails resolves each email's owning inbound through the
// clients/client_inbounds relation in chunked queries. Like the old per-email
// First() it keeps the lowest inbound id when a client spans several inbounds.
func (j *CheckClientIpJob) loadInboundsByEmails(emails []string) map[string]*model.Inbound {
	db := database.GetDB()
	minInboundByEmail := make(map[string]int, len(emails))
	for _, batch := range chunkEmails(emails, ipScanChunk) {
		var pairs []struct {
			Email     string
			InboundId int
		}
		if err := db.Table("client_inbounds").
			Select("clients.email AS email, client_inbounds.inbound_id AS inbound_id").
			Joins("JOIN clients ON clients.id = client_inbounds.client_id").
			Where("clients.email IN ?", batch).
			Scan(&pairs).Error; err != nil {
			j.checkError(err)
			return nil
		}
		for _, p := range pairs {
			if cur, ok := minInboundByEmail[p.Email]; !ok || p.InboundId < cur {
				minInboundByEmail[p.Email] = p.InboundId
			}
		}
	}
	if len(minInboundByEmail) == 0 {
		return nil
	}

	idSet := make(map[int]struct{}, len(minInboundByEmail))
	ids := make([]int, 0, len(minInboundByEmail))
	for _, id := range minInboundByEmail {
		if _, seen := idSet[id]; !seen {
			idSet[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	inboundsById := make(map[int]*model.Inbound, len(ids))
	for lo := 0; lo < len(ids); lo += ipScanChunk {
		hi := min(lo+ipScanChunk, len(ids))
		var page []*model.Inbound
		if err := db.Model(&model.Inbound{}).Where("id IN ?", ids[lo:hi]).Find(&page).Error; err != nil {
			j.checkError(err)
			return nil
		}
		for _, ib := range page {
			inboundsById[ib.Id] = ib
		}
	}

	out := make(map[string]*model.Inbound, len(minInboundByEmail))
	for email, id := range minInboundByEmail {
		if ib, ok := inboundsById[id]; ok {
			out[email] = ib
		}
	}
	return out
}

func (j *CheckClientIpJob) loadClientIpRows(emails []string) map[string]*model.InboundClientIps {
	db := database.GetDB()
	out := make(map[string]*model.InboundClientIps, len(emails))
	for _, batch := range chunkEmails(emails, ipScanChunk) {
		var rows []model.InboundClientIps
		if err := db.Where("client_email IN ?", batch).Find(&rows).Error; err != nil {
			j.checkError(err)
			continue
		}
		for i := range rows {
			out[rows[i].ClientEmail] = &rows[i]
		}
	}
	return out
}

// processObserved runs collection + enforcement for one scan's observations
// (email -> ip -> last-seen unix seconds). observedAreLive marks the
// observations as live connections, which bypass the stale cutoff: a connection
// that opened hours ago is still live even though its timestamp is old. The
// online-stats API always reports live connections, so the job passes true.
// Lookups are batched up front and all inbound_client_ips writes share one
// transaction, so a scan costs a handful of queries and one fsync instead of
// several per observed email.
func (j *CheckClientIpJob) processObserved(observed map[string]map[string]int64, enforce, observedAreLive bool) bool {
	shouldCleanLog := false
	now := time.Now().Unix()

	emails := make([]string, 0, len(observed))
	for email := range observed {
		emails = append(emails, email)
	}
	sort.Strings(emails)

	limitByEmail := j.loadClientLimits(emails)
	inboundByEmail := j.loadInboundsByEmails(emails)
	ipRowByEmail := j.loadClientIpRows(emails)

	// attribution accumulates this scan's local observations per email so they can
	// be recorded under this panel's own guid for cross-node IP attribution.
	attribution := make(map[string][]model.ClientIpEntry, len(observed))

	var bans []pendingBan

	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		j.checkError(tx.Error)
		return false
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	for _, email := range emails {
		ipTimestamps := observed[email]

		// The observations can still reference a client that was just renamed
		// or deleted; its email no longer matches any inbound. Skip it (and
		// drop any orphaned tracking row) instead of recreating a row and
		// logging an ERROR every run (#4963). The batch map resolves through
		// the clients relation; the per-email fallback keeps its settings LIKE
		// net for clients not yet present there.
		inbound, ok := inboundByEmail[email]
		if !ok {
			var err error
			inbound, err = j.getInboundByEmail(email)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					logger.Debugf("[LimitIP] skipping stale observed email %q (renamed or deleted)", email)
					j.delInboundClientIps(tx, email)
				} else {
					j.checkError(err)
				}
				continue
			}
		}

		// Convert to IPWithTimestamp slice
		ipsWithTime := make([]IPWithTimestamp, 0, len(ipTimestamps))
		attrEntries := make([]model.ClientIpEntry, 0, len(ipTimestamps))
		for ip, timestamp := range ipTimestamps {
			ipsWithTime = append(ipsWithTime, IPWithTimestamp{IP: ip, Timestamp: timestamp})
			// Live API observations may carry an old lastSeen (connection start),
			// so stamp attribution with now; otherwise the stale cutoff would evict
			// an IP that is connected right now.
			attrTs := timestamp
			if observedAreLive {
				attrTs = now
			}
			attrEntries = append(attrEntries, model.ClientIpEntry{IP: ip, Timestamp: attrTs})
		}
		if len(attrEntries) > 0 {
			attribution[email] = attrEntries
		}

		clientIpsRecord, ok := ipRowByEmail[email]
		if !ok {
			jsonIps, err := json.Marshal(ipsWithTime)
			if err != nil {
				j.checkError(err)
				continue
			}
			if err := tx.Save(&model.InboundClientIps{ClientEmail: email, Ips: string(jsonIps)}).Error; err != nil {
				j.checkError(err)
			}
			continue
		}

		candidates, keptLive := j.updateInboundClientIps(tx, clientIpsRecord, inbound, email, limitByEmail[email], ipsWithTime, enforce, observedAreLive)
		bans = append(bans, pendingBan{inbound: inbound, email: email, candidates: candidates, keptLive: keptLive})
	}

	if err := tx.Commit().Error; err != nil {
		j.checkError(err)
		return false
	}
	committed = true

	published := j.publishBans(bans)

	// Xray disconnects run after the commit so their network round-trips never
	// extend the scan's write transaction (node syncs upsert the same table).
	shouldCleanLog = shouldCleanLog || len(published) > 0
	clientsCache := make(map[int][]model.Client)
	for _, d := range published {
		clients, cached := clientsCache[d.inbound.Id]
		if !cached {
			clients, _ = service.ParseInboundSettingsClients(d.inbound.Settings)
			clientsCache[d.inbound.Id] = clients
		}
		j.disconnectClientTemporarily(d.inbound, d.email, clients)
	}

	j.recordLocalAttribution(attribution)

	return shouldCleanLog
}

// recordLocalAttribution stores this scan's local observations under this panel's
// own guid so a parent panel can attribute each IP to the node it is on.
// Best-effort: attribution is advisory and must never block IP-limit enforcement.
func (j *CheckClientIpJob) recordLocalAttribution(attribution map[string][]model.ClientIpEntry) {
	if len(attribution) == 0 {
		return
	}
	guid, err := (&service.SettingService{}).GetPanelGuid()
	if err != nil || guid == "" {
		return
	}
	if err := (&service.InboundService{}).RecordLocalClientIps(guid, attribution); err != nil {
		logger.Debug("[LimitIP] record local ip attribution failed:", err)
	}
}

// mergeClientIps folds this scan's observations into the persisted set,
// dropping entries older than staleCutoff. newAlwaysLive exempts the new
// entries from that cutoff: an API-observed IP is a live connection by
// definition, even when its lastSeen (set at dispatch time) is hours old.
func mergeClientIps(old, new []IPWithTimestamp, staleCutoff int64, newAlwaysLive bool) map[string]int64 {
	ipMap := make(map[string]int64, len(old)+len(new))
	for _, ipTime := range old {
		if ipTime.Timestamp < staleCutoff {
			continue
		}
		ipMap[ipTime.IP] = ipTime.Timestamp
	}
	for _, ipTime := range new {
		if !newAlwaysLive && ipTime.Timestamp < staleCutoff {
			continue
		}
		if existingTime, ok := ipMap[ipTime.IP]; !ok || ipTime.Timestamp > existingTime {
			ipMap[ipTime.IP] = ipTime.Timestamp
		}
	}
	return ipMap
}

// selectIpsToBan splits the live IPs (sorted oldest-first by partitionLiveIps)
// into the newest `limit` entries to keep and the older remainder to ban.
func selectIpsToBan(live []IPWithTimestamp, limit int) (kept, banned []IPWithTimestamp) {
	if limit <= 0 || len(live) <= limit {
		return live, nil
	}
	cutoff := len(live) - limit
	return live[cutoff:], live[:cutoff]
}

func partitionLiveIps(ipMap map[string]int64, observedThisScan map[string]bool) (live, historical []IPWithTimestamp) {
	live = make([]IPWithTimestamp, 0, len(observedThisScan))
	historical = make([]IPWithTimestamp, 0, len(ipMap))
	now := time.Now().Unix()
	for ip, ts := range ipMap {
		entry := IPWithTimestamp{IP: ip, Timestamp: ts}
		// Consider an IP "live" if it was seen locally in this scan, OR if its
		// timestamp from the synced database is very recent (e.g. within 2 minutes).
		// This ensures cluster-wide limits work even if the IP was seen on another node.
		if observedThisScan[ip] || now-ts < 120 {
			live = append(live, entry)
		} else {
			historical = append(historical, entry)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].Timestamp < live[j].Timestamp })
	sort.Slice(historical, func(i, j int) bool { return historical[i].Timestamp < historical[j].Timestamp })
	return live, historical
}

func (j *CheckClientIpJob) checkFail2BanInstalled() bool {
	if !isFail2BanEnabled() {
		return false
	}

	cmd := "fail2ban-client"
	args := []string{"-h"}
	err := exec.CommandContext(context.Background(), cmd, args...).Run()
	return err == nil
}

func isFail2BanEnabled() bool {
	value, ok := os.LookupEnv("XUI_ENABLE_FAIL2BAN")
	return !ok || value == "true"
}

func (j *CheckClientIpJob) checkError(e error) {
	if e != nil {
		logger.Warning("client ip job err:", e)
	}
}

// delInboundClientIps drops the inbound_client_ips tracking row for an email
// that no longer maps to any inbound (a renamed or deleted client), so stale
// access-log entries don't keep a ghost row alive (#4963).
func (j *CheckClientIpJob) delInboundClientIps(tx *gorm.DB, clientEmail string) {
	if err := tx.Where("client_email = ?", clientEmail).Delete(&model.InboundClientIps{}).Error; err != nil {
		j.checkError(err)
	}
}

// updateInboundClientIps merges one email's observed IPs into its tracking row
// and applies the IP limit. Ban candidates are returned, not written: the
// fail2ban log is the point of no return and must wait for the commit.
func (j *CheckClientIpJob) updateInboundClientIps(tx *gorm.DB, inboundClientIps *model.InboundClientIps, inbound *model.Inbound, clientEmail string, limitIp int, newIpsWithTime []IPWithTimestamp, enforce, observedAreLive bool) (banCandidates []IPWithTimestamp, keptLiveCount int) {
	if inbound.Settings == "" {
		logger.Debug("wrong data:", inbound)
		return nil, 0
	}

	if !enforce || limitIp <= 0 || !inbound.Enable {
		// Nothing to enforce (collection-only run, no limit on the clients row,
		// or inbound disabled): record the observed IPs for the panel and return.
		jsonIps, _ := json.Marshal(newIpsWithTime)
		inboundClientIps.Ips = string(jsonIps)
		if err := tx.Save(inboundClientIps).Error; err != nil {
			logger.Error("failed to save inboundClientIps:", err)
		}
		return nil, 0
	}

	// only ips seen in this scan count toward the limit. see
	// partitionLiveIps.
	observedThisScan := make(map[string]bool, len(newIpsWithTime))
	for _, ipTime := range newIpsWithTime {
		observedThisScan[ipTime.IP] = true
	}
	staleCutoff := time.Now().Unix() - ipStaleAfterSeconds

	// Node sync merges into the same blob (#6587): compare-and-set it, and on a
	// miss re-read and re-merge so neither writer drops the other's IPs.
	for range service.ClientIpCasRetries {
		var oldIpsWithTime []IPWithTimestamp
		if inboundClientIps.Ips != "" {
			_ = json.Unmarshal([]byte(inboundClientIps.Ips), &oldIpsWithTime)
		}
		ipMap := mergeClientIps(oldIpsWithTime, newIpsWithTime, staleCutoff, observedAreLive)
		liveIps, historicalIps := partitionLiveIps(ipMap, observedThisScan)

		// historical db-only ips are excluded from this count on purpose.
		limitedIps, allowedIps := j.allowlist.split(liveIps)
		keptLive, bannedLive := selectIpsToBan(limitedIps, limitIp)
		// Allowlisted addresses stay connected and out of the count: charging them
		// against the limit would still cut the shared network the entry protects.
		keptLive = append(keptLive, allowedIps...)

		// keep kept-live + historical in the blob so the panel keeps showing recently
		// seen ips; banned live ips reappear in the next scan if they reconnect.
		dbIps := make([]IPWithTimestamp, 0, len(keptLive)+len(historicalIps))
		dbIps = append(dbIps, keptLive...)
		dbIps = append(dbIps, historicalIps...)
		jsonIps, _ := json.Marshal(dbIps)
		newIps := string(jsonIps)
		if newIps == inboundClientIps.Ips {
			return bannedLive, len(keptLive)
		}

		updated, err := service.CasUpdateInboundClientIps(tx, inboundClientIps.Id, inboundClientIps.Ips, newIps)
		if err != nil {
			logger.Error("failed to save inboundClientIps:", err)
			return nil, 0
		}
		if updated {
			inboundClientIps.Ips = newIps
			return bannedLive, len(keptLive)
		}
		if err := tx.Where("id = ?", inboundClientIps.Id).First(inboundClientIps).Error; err != nil {
			logger.Error("failed to re-read inboundClientIps after a concurrent write:", err)
			return nil, 0
		}
	}
	logger.Error("failed to save inboundClientIps: exhausted CAS retries")
	return nil, 0
}

// pendingBan carries one client's enforcement outcome from inside the scan's
// transaction to the publication that may only follow a successful commit.
type pendingBan struct {
	inbound    *model.Inbound
	email      string
	candidates []IPWithTimestamp
	keptLive   int
}

// publishBans returns the clients whose lines reached the log. bannedSeen
// advances only for those, so a failed write leaves the address retryable.
func (j *CheckClientIpJob) publishBans(bans []pendingBan) []pendingBan {
	published := make([]pendingBan, 0, len(bans))
	var logIpFile *os.File
	defer func() {
		if logIpFile == nil {
			return
		}
		if err := logIpFile.Close(); err != nil {
			logger.Errorf("failed to close IP limit log file: %s", err)
		}
	}()
	for _, b := range bans {
		actionable := j.selectAdvancedSinceLastBan(b.email, b.candidates)
		if len(actionable) == 0 {
			j.recordBannedSeen(b.email, b.candidates, nil)
			continue
		}
		if logIpFile == nil {
			f, err := os.OpenFile(xray.GetIPLimitLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				logger.Errorf("failed to open IP limit log file: %s", err)
				return published
			}
			logIpFile = f
		}
		if err := writeBanLines(logIpFile, b.email, actionable); err != nil {
			logger.Errorf("failed to write IP limit bans for %s: %s", b.email, err)
			continue
		}
		j.recordBannedSeen(b.email, b.candidates, actionable)
		logger.Infof("[LIMIT_IP] Client %s: Kept %d live IPs, queued %d old IPs for fail2ban", b.email, b.keptLive, len(actionable))
		published = append(published, b)
	}
	return published
}

// writeBanLines emits one line per address; the wording is load-bearing, since
// x-ui.sh create_iplimit_jails builds filter.d/3x-ipl.conf failregex from it.
func writeBanLines(w io.Writer, clientEmail string, actionable []IPWithTimestamp) error {
	stamp := time.Now().Format("2006/01/02 15:04:05")
	for _, ipTime := range actionable {
		if _, err := fmt.Fprintf(w, "%s [LIMIT_IP] Email = %s || Disconnecting OLD IP = %s || Timestamp = %d\n",
			stamp, clientEmail, ipTime.IP, ipTime.Timestamp); err != nil {
			return err
		}
	}
	return nil
}

// selectAdvancedSinceLastBan drops pairs with a frozen lastSeen: the core
// refreshes it only on a new dispatch, so those are unreaped dead connections.
func (j *CheckClientIpJob) selectAdvancedSinceLastBan(email string, banned []IPWithTimestamp) []IPWithTimestamp {
	actionable := make([]IPWithTimestamp, 0, len(banned))
	for _, ipTime := range banned {
		if last, ok := j.bannedSeen[email+"|"+ipTime.IP]; ok && ipTime.Timestamp <= last {
			continue
		}
		actionable = append(actionable, ipTime)
	}
	return actionable
}

// recordBannedSeen marks published pairs and forgets addresses this scan no
// longer bans; it runs for every enforced client, which is what prunes the map.
func (j *CheckClientIpJob) recordBannedSeen(email string, banned, published []IPWithTimestamp) {
	if j.bannedSeen == nil {
		j.bannedSeen = make(map[string]int64)
	}
	for _, ipTime := range published {
		j.bannedSeen[email+"|"+ipTime.IP] = ipTime.Timestamp
	}
	current := make(map[string]struct{}, len(banned))
	for _, ipTime := range banned {
		current[email+"|"+ipTime.IP] = struct{}{}
	}
	prefix := email + "|"
	for key := range j.bannedSeen {
		if strings.HasPrefix(key, prefix) {
			if _, still := current[key]; !still {
				delete(j.bannedSeen, key)
			}
		}
	}
}

// disconnectClientTemporarily drops a client's credential for a moment, so new
// handshakes are refused; the fail2ban ban is what ends live traffic.
func (j *CheckClientIpJob) disconnectClientTemporarily(inbound *model.Inbound, clientEmail string, clients []model.Client) {
	var xrayAPI xray.XrayAPI
	apiPort := j.resolveXrayAPIPort()

	err := xrayAPI.Init(apiPort)
	if err != nil {
		logger.Warningf("[LIMIT_IP] Failed to init Xray API for disconnection: %v", err)
		return
	}
	defer xrayAPI.Close()

	// Find the client config
	var clientConfig map[string]any
	var reverseClient bool
	for _, client := range clients {
		if client.Email == clientEmail {
			// Convert client to map for API
			clientBytes, _ := json.Marshal(client)
			_ = json.Unmarshal(clientBytes, &clientConfig)
			reverseClient = client.Reverse != nil
			break
		}
	}

	if clientConfig == nil {
		return
	}

	// Protocols XrayAPI can remove and re-add from a marshaled model.Client.
	// wireguard stays out: keepAlive marshals as a number, AddUser wants a string.
	protocol := string(inbound.Protocol)
	switch protocol {
	case "vmess", "vless", "trojan", "shadowsocks", "hysteria":
		// supported protocols, continue
	default:
		logger.Warningf("[LIMIT_IP] Temporary disconnect is not supported for protocol %s on inbound %s", protocol, inbound.Tag)
		return
	}

	// RemoveUser drops a reverse client's outbound handler and the re-add below
	// cannot restore it, so its tunnel would stay down until Xray restarts.
	if reverseClient {
		logger.Warningf("[LIMIT_IP] Not disconnecting %s: its reverse proxy config does not survive a temporary removal", clientEmail)
		return
	}

	// For Shadowsocks, ensure the required "cipher" field is present by
	// reading it from the inbound settings (e.g., settings["method"]).
	if string(inbound.Protocol) == "shadowsocks" {
		var inboundSettings map[string]any
		if err := json.Unmarshal([]byte(inbound.Settings), &inboundSettings); err != nil {
			logger.Warningf("[LIMIT_IP] Failed to parse inbound settings for shadowsocks cipher: %v", err)
		} else {
			if method, ok := inboundSettings["method"].(string); ok && method != "" {
				clientConfig["cipher"] = method
			}
		}
	}

	// The core's RemoveUser clears its validator: a session already up keeps
	// running, except a reverse vless client, which is skipped above.
	err = xrayAPI.RemoveUser(inbound.Tag, clientEmail)
	if err != nil {
		logger.Warningf("[LIMIT_IP] Failed to remove user %s: %v", clientEmail, err)
		return
	}

	// Nothing is pending here: AlterInbound applies the removal inline, so this
	// only widens the window in which new handshakes fail.
	time.Sleep(100 * time.Millisecond)

	// Re-add user to allow new connections
	err = xrayAPI.AddUser(protocol, inbound.Tag, clientConfig)
	if err != nil {
		logger.Warningf("[LIMIT_IP] Failed to re-add user %s: %v", clientEmail, err)
	}
}

// resolveXrayAPIPort returns the API inbound port from running config, then template config, then default.
func (j *CheckClientIpJob) resolveXrayAPIPort() int {
	var configErr error
	var templateErr error

	if port, err := getAPIPortFromConfigPath(xray.GetConfigPath()); err == nil {
		return port
	} else {
		configErr = err
	}

	db := database.GetDB()
	var template model.Setting
	if err := db.Where("key = ?", "xrayTemplateConfig").First(&template).Error; err == nil {
		if port, parseErr := getAPIPortFromConfigData([]byte(template.Value)); parseErr == nil {
			return port
		} else {
			templateErr = parseErr
		}
	} else {
		templateErr = err
	}

	logger.Warningf(
		"[LIMIT_IP] Could not determine Xray API port from config or template; falling back to default port %d (config error: %v, template error: %v)",
		defaultXrayAPIPort,
		configErr,
		templateErr,
	)

	return defaultXrayAPIPort
}

func getAPIPortFromConfigPath(configPath string) (int, error) {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return 0, err
	}

	return getAPIPortFromConfigData(configData)
}

func getAPIPortFromConfigData(configData []byte) (int, error) {
	xrayConfig := &xray.Config{}
	if err := json.Unmarshal(configData, xrayConfig); err != nil {
		return 0, err
	}

	for _, inboundConfig := range xrayConfig.InboundConfigs {
		if inboundConfig.Tag == "api" && inboundConfig.Port > 0 {
			return inboundConfig.Port, nil
		}
	}

	return 0, errors.New("api inbound port not found")
}

// getInboundByEmail resolves the inbound that owns a client email. It prefers
// the exact clients/client_inbounds relation; a substring "settings LIKE
// %email%" can match the wrong inbound (an email that is a substring of another,
// or text that merely appears elsewhere in the settings JSON). The LIKE + JSON
// scan stays only as a fallback for clients not yet present in the relation, so
// nothing regresses when the join finds no row.
func (j *CheckClientIpJob) getInboundByEmail(clientEmail string) (*model.Inbound, error) {
	db := database.GetDB()
	inbound := &model.Inbound{}

	err := db.Model(&model.Inbound{}).
		Joins("JOIN client_inbounds ON client_inbounds.inbound_id = inbounds.id").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email = ?", clientEmail).
		First(inbound).Error
	if err == nil {
		return inbound, nil
	}

	var candidates []model.Inbound
	if listErr := db.Model(&model.Inbound{}).Where("settings LIKE ?", "%"+clientEmail+"%").Find(&candidates).Error; listErr != nil {
		return nil, listErr
	}
	for i := range candidates {
		clients, jsonErr := service.ParseInboundSettingsClients(candidates[i].Settings)
		if jsonErr != nil {
			continue
		}
		for _, client := range clients {
			if client.Email == clientEmail {
				return &candidates[i], nil
			}
		}
	}

	return nil, err
}

// Runs before the fail2ban/apiMode gates: retention must hold for stored rows
// even while nothing is being collected.
func (j *CheckClientIpJob) pruneStaleIpRows() {
	now := time.Now().Unix()
	if now-j.lastIpPrune < ipPruneIntervalSeconds {
		return
	}
	j.lastIpPrune = now
	if err := (&service.InboundService{}).PruneStaleClientIps(); err != nil {
		logger.Warning("prune stale client ip rows failed:", err)
	}
}
