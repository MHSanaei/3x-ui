package job

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// IPWithTimestamp tracks an IP address with its last seen timestamp
type IPWithTimestamp struct {
	IP        string `json:"ip"`
	Timestamp int64  `json:"timestamp"`
}

// CheckClientIpJob monitors client IP addresses from access logs and manages IP blocking based on configured limits.
type CheckClientIpJob struct {
	lastClear     int64
	disAllowedIps []string
}

var job *CheckClientIpJob

const defaultXrayAPIPort = 62789

// ipStaleAfterSeconds controls how long a client IP kept in the
// per-client tracking table (model.InboundClientIps.Ips) is considered
// still "active" before it's evicted during the next scan.
//
// Without this eviction, an IP that connected once and then went away
// keeps sitting in the table with its old timestamp. Because the
// excess-IP selector sorts ascending ("oldest wins, newest loses") to
// protect the original/current connections, that stale entry keeps
// occupying a slot and the IP that is *actually* currently using the
// config gets classified as "new excess" and banned by fail2ban on
// every single run — producing the continuous ban loop from #4077.
//
// 30 minutes is chosen so an actively-streaming client (where xray
// emits a fresh `accepted` log line whenever it opens a new TCP) will
// always refresh its timestamp well within the window, but a client
// that has really stopped using the config will drop out of the table
// in a bounded time and free its slot.
const ipStaleAfterSeconds = int64(30 * 60)

// NewCheckClientIpJob creates a new client IP monitoring job instance.
func NewCheckClientIpJob() *CheckClientIpJob {
	job = new(CheckClientIpJob)
	return job
}

func (j *CheckClientIpJob) Run() {
	if j.lastClear == 0 {
		j.lastClear = time.Now().Unix()
	}

	shouldClearAccessLog := false
	iplimitActive := j.hasLimitIp()
	f2bInstalled := j.checkFail2BanInstalled()
	isAccessLogAvailable := j.checkAccessLogAvailable(iplimitActive)

	if isAccessLogAvailable {
		if runtime.GOOS == "windows" {
			if iplimitActive {
				shouldClearAccessLog = j.processLogFile()
			}
		} else {
			if iplimitActive {
				if f2bInstalled {
					shouldClearAccessLog = j.processLogFile()
				} else {
					if !f2bInstalled {
						logger.Warning("[LimitIP] Fail2Ban is not installed, Please install Fail2Ban from the x-ui bash menu.")
					}
				}
			}
		}
	}

	if shouldClearAccessLog || (isAccessLogAvailable && time.Now().Unix()-j.lastClear > 3600) {
		j.clearAccessLog()
	}
}

func (j *CheckClientIpJob) clearAccessLog() {
	logAccessP, err := os.OpenFile(xray.GetAccessPersistentLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	j.checkError(err)
	defer logAccessP.Close()

	accessLogPath, err := xray.GetAccessLogPath()
	j.checkError(err)

	file, err := os.Open(accessLogPath)
	j.checkError(err)
	defer file.Close()

	_, err = io.Copy(logAccessP, file)
	j.checkError(err)

	err = os.Truncate(accessLogPath, 0)
	j.checkError(err)

	j.lastClear = time.Now().Unix()
}

func (j *CheckClientIpJob) hasLimitIp() bool {
	db := database.GetDB()
	var inbounds []*model.Inbound

	err := db.Model(model.Inbound{}).Find(&inbounds).Error
	if err != nil {
		return false
	}

	for _, inbound := range inbounds {
		if inbound.Settings == "" {
			continue
		}

		settings := map[string][]model.Client{}
		json.Unmarshal([]byte(inbound.Settings), &settings)
		clients := settings["clients"]

		for _, client := range clients {
			limitIp := client.LimitIP
			if limitIp > 0 {
				return true
			}
		}
	}

	return false
}

func (j *CheckClientIpJob) processLogFile() bool {

	ipRegex := regexp.MustCompile(`from (?:tcp:|udp:)?\[?([0-9a-fA-F\.:]+)\]?:\d+ accepted`)
	emailRegex := regexp.MustCompile(`email: (.+)$`)
	timestampRegex := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`)

	accessLogPath, _ := xray.GetAccessLogPath()
	file, _ := os.Open(accessLogPath)
	defer file.Close()

	// Track IPs with their last seen timestamp
	inboundClientIps := make(map[string]map[string]int64, 100)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		ipMatches := ipRegex.FindStringSubmatch(line)
		if len(ipMatches) < 2 {
			continue
		}

		ip := ipMatches[1]

		if ip == "127.0.0.1" || ip == "::1" {
			continue
		}

		emailMatches := emailRegex.FindStringSubmatch(line)
		if len(emailMatches) < 2 {
			continue
		}
		email := emailMatches[1]

		// Extract timestamp from log line
		var timestamp int64
		timestampMatches := timestampRegex.FindStringSubmatch(line)
		if len(timestampMatches) >= 2 {
			t, err := time.Parse("2006/01/02 15:04:05", timestampMatches[1])
			if err == nil {
				timestamp = t.Unix()
			} else {
				timestamp = time.Now().Unix()
			}
		} else {
			timestamp = time.Now().Unix()
		}

		if _, exists := inboundClientIps[email]; !exists {
			inboundClientIps[email] = make(map[string]int64)
		}
		// Update timestamp - keep the latest
		if existingTime, ok := inboundClientIps[email][ip]; !ok || timestamp > existingTime {
			inboundClientIps[email][ip] = timestamp
		}
	}
	if err := scanner.Err(); err != nil {
		j.checkError(err)
	}

	shouldCleanLog := false
	for email, ipTimestamps := range inboundClientIps {

		// Convert to IPWithTimestamp slice
		ipsWithTime := make([]IPWithTimestamp, 0, len(ipTimestamps))
		for ip, timestamp := range ipTimestamps {
			ipsWithTime = append(ipsWithTime, IPWithTimestamp{IP: ip, Timestamp: timestamp})
		}

		clientIpsRecord, err := j.getInboundClientIps(email)
		if err != nil {
			j.addInboundClientIps(email, ipsWithTime)
			continue
		}

		shouldCleanLog = j.updateInboundClientIps(clientIpsRecord, email, ipsWithTime) || shouldCleanLog
	}

	return shouldCleanLog
}

// mergeClientIps combines the persisted (old) and freshly observed (new)
// IP-with-timestamp lists for a single client into a map. An entry is
// dropped if its last-seen timestamp is older than staleCutoff.
//
// Extracted as a helper so updateInboundClientIps can stay DB-oriented
// and the merge policy can be exercised by a unit test.
func mergeClientIps(old, new []IPWithTimestamp, staleCutoff int64) map[string]int64 {
	ipMap := make(map[string]int64, len(old)+len(new))
	for _, ipTime := range old {
		if ipTime.Timestamp < staleCutoff {
			continue
		}
		ipMap[ipTime.IP] = ipTime.Timestamp
	}
	for _, ipTime := range new {
		if ipTime.Timestamp < staleCutoff {
			continue
		}
		if existingTime, ok := ipMap[ipTime.IP]; !ok || ipTime.Timestamp > existingTime {
			ipMap[ipTime.IP] = ipTime.Timestamp
		}
	}
	return ipMap
}

// partitionLiveIps splits the merged ip map into live (seen in the
// current scan) and historical (only in the db blob, still inside the
// staleness window).
//
// only live ips count toward the per-client limit. historical ones stay
// in the db so the panel keeps showing them, but they must not take a
// protected slot. the 30min cutoff alone isn't tight enough: an ip that
// stopped connecting a few minutes ago still looks fresh to
// mergeClientIps, and since the over-limit picker sorts ascending and
// keeps the oldest, those idle entries used to win the slot while the
// ip actually connecting got classified as excess and sent to fail2ban
// every tick. see #4077 / #4091.
//
// live is sorted ascending so the "protect original, ban newcomer"
// rule still holds when several ips are really connecting at once.
func partitionLiveIps(ipMap map[string]int64, observedThisScan map[string]bool) (live, historical []IPWithTimestamp) {
	live = make([]IPWithTimestamp, 0, len(observedThisScan))
	historical = make([]IPWithTimestamp, 0, len(ipMap))
	for ip, ts := range ipMap {
		entry := IPWithTimestamp{IP: ip, Timestamp: ts}
		if observedThisScan[ip] {
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
	cmd := "fail2ban-client"
	args := []string{"-h"}
	err := exec.Command(cmd, args...).Run()
	return err == nil
}

func (j *CheckClientIpJob) checkAccessLogAvailable(iplimitActive bool) bool {
	accessLogPath, err := xray.GetAccessLogPath()
	if err != nil {
		return false
	}

	if accessLogPath == "none" || accessLogPath == "" {
		if iplimitActive {
			logger.Warning("[LimitIP] Access log path is not set, Please configure the access log path in Xray configs.")
		}
		return false
	}

	return true
}

func (j *CheckClientIpJob) checkError(e error) {
	if e != nil {
		logger.Warning("client ip job err:", e)
	}
}

func (j *CheckClientIpJob) getInboundClientIps(clientEmail string) (*model.InboundClientIps, error) {
	db := database.GetDB()
	InboundClientIps := &model.InboundClientIps{}
	err := db.Model(model.InboundClientIps{}).Where("client_email = ?", clientEmail).First(InboundClientIps).Error
	if err != nil {
		return nil, err
	}
	return InboundClientIps, nil
}

func (j *CheckClientIpJob) addInboundClientIps(clientEmail string, ipsWithTime []IPWithTimestamp) error {
	inboundClientIps := &model.InboundClientIps{}
	jsonIps, err := json.Marshal(ipsWithTime)
	j.checkError(err)

	inboundClientIps.ClientEmail = clientEmail
	inboundClientIps.Ips = string(jsonIps)

	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	err = tx.Save(inboundClientIps).Error
	if err != nil {
		return err
	}
	return nil
}

func (j *CheckClientIpJob) updateInboundClientIps(inboundClientIps *model.InboundClientIps, clientEmail string, newIpsWithTime []IPWithTimestamp) bool {
	// Get the inbound configuration
	inbound, err := j.getInboundByEmail(clientEmail)
	if err != nil {
		logger.Errorf("failed to fetch inbound settings for email %s: %s", clientEmail, err)
		return false
	}

	if inbound.Settings == "" {
		logger.Debug("wrong data:", inbound)
		return false
	}

	settings := map[string][]model.Client{}
	json.Unmarshal([]byte(inbound.Settings), &settings)
	clients := settings["clients"]

	// Find the client's IP limit
	var limitIp int
	var clientFound bool
	for _, client := range clients {
		if client.Email == clientEmail {
			limitIp = client.LimitIP
			clientFound = true
			break
		}
	}

	if !clientFound || limitIp <= 0 || !inbound.Enable {
		// No limit or inbound disabled, just update and return
		jsonIps, _ := json.Marshal(newIpsWithTime)
		inboundClientIps.Ips = string(jsonIps)
		db := database.GetDB()
		db.Save(inboundClientIps)
		return false
	}

	// Parse old IPs from database
	var oldIpsWithTime []IPWithTimestamp
	if inboundClientIps.Ips != "" {
		json.Unmarshal([]byte(inboundClientIps.Ips), &oldIpsWithTime)
	}

	// Merge old and new IPs, evicting entries that haven't been
	// re-observed in a while. See mergeClientIps / #4077 for why.
	ipMap := mergeClientIps(oldIpsWithTime, newIpsWithTime, time.Now().Unix()-ipStaleAfterSeconds)

	// only ips seen in this scan count toward the limit. see
	// partitionLiveIps.
	observedThisScan := make(map[string]bool, len(newIpsWithTime))
	for _, ipTime := range newIpsWithTime {
		observedThisScan[ipTime.IP] = true
	}
	liveIps, historicalIps := partitionLiveIps(ipMap, observedThisScan)

	shouldCleanLog := false
	j.disAllowedIps = []string{}

	// historical db-only ips are excluded from this count on purpose.
	var keptLive []IPWithTimestamp
	if len(liveIps) > limitIp {
		shouldCleanLog = true

		// protect the oldest live ip, ban newcomers.
		keptLive = liveIps[:limitIp]
		bannedLive := liveIps[limitIp:]

		// Open log file only when a ban entry needs to be written.
		// Use a local logger to avoid mutating the global log.* state,
		// which would redirect all standard-library logging to this file
		// and leave a dangling closed-file handle after the defer fires.
		logIpFile, err := os.OpenFile(xray.GetIPLimitLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			logger.Errorf("failed to open IP limit log file: %s", err)
			return false
		}
		defer logIpFile.Close()
		ipLogger := log.New(logIpFile, "", log.LstdFlags)

		// log format is load-bearing: x-ui.sh create_iplimit_jails builds
		// filter.d/3x-ipl.conf with
		//   failregex = \[LIMIT_IP\]\s*Email\s*=\s*<F-USER>.+</F-USER>\s*\|\|\s*Disconnecting OLD IP\s*=\s*<ADDR>\s*\|\|\s*Timestamp\s*=\s*\d+
		// don't change the wording.
		for _, ipTime := range bannedLive {
			j.disAllowedIps = append(j.disAllowedIps, ipTime.IP)
			ipLogger.Printf("[LIMIT_IP] Email = %s || Disconnecting OLD IP = %s || Timestamp = %d", clientEmail, ipTime.IP, ipTime.Timestamp)
		}

		// force xray to drop existing connections from banned ips
		j.disconnectClientTemporarily(inbound, clientEmail, clients)
	} else {
		keptLive = liveIps
	}

	// keep kept-live + historical in the blob so the panel keeps showing
	// recently seen ips. banned live ips are already in the fail2ban log
	// and will reappear in the next scan if they reconnect.
	dbIps := make([]IPWithTimestamp, 0, len(keptLive)+len(historicalIps))
	dbIps = append(dbIps, keptLive...)
	dbIps = append(dbIps, historicalIps...)
	jsonIps, _ := json.Marshal(dbIps)
	inboundClientIps.Ips = string(jsonIps)

	db := database.GetDB()
	err = db.Save(inboundClientIps).Error
	if err != nil {
		logger.Error("failed to save inboundClientIps:", err)
		return false
	}

	if len(j.disAllowedIps) > 0 {
		logger.Infof("[LIMIT_IP] Client %s: Kept %d live IPs, queued %d new IPs for fail2ban", clientEmail, len(keptLive), len(j.disAllowedIps))
	}

	return shouldCleanLog
}

// disconnectClientTemporarily removes and re-adds a client to force disconnect banned connections
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
	for _, client := range clients {
		if client.Email == clientEmail {
			// Convert client to map for API
			clientBytes, _ := json.Marshal(client)
			json.Unmarshal(clientBytes, &clientConfig)
			break
		}
	}

	if clientConfig == nil {
		return
	}

	// Only perform remove/re-add for protocols supported by XrayAPI.AddUser
	protocol := string(inbound.Protocol)
	switch protocol {
	case "vmess", "vless", "trojan", "shadowsocks":
		// supported protocols, continue
	default:
		logger.Warningf("[LIMIT_IP] Temporary disconnect is not supported for protocol %s on inbound %s", protocol, inbound.Tag)
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

	// Remove user to disconnect all connections
	err = xrayAPI.RemoveUser(inbound.Tag, clientEmail)
	if err != nil {
		logger.Warningf("[LIMIT_IP] Failed to remove user %s: %v", clientEmail, err)
		return
	}

	// Wait a moment for disconnection to take effect
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

func (j *CheckClientIpJob) getInboundByEmail(clientEmail string) (*model.Inbound, error) {
	db := database.GetDB()
	inbound := &model.Inbound{}

	err := db.Model(&model.Inbound{}).Where("settings LIKE ?", "%"+clientEmail+"%").First(inbound).Error
	if err != nil {
		return nil, err
	}

	return inbound, nil
}
