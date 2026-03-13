package job

import (
	"bufio"
	"encoding/json"
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

	// Merge old and new IPs, keeping the latest timestamp for each IP
	ipMap := make(map[string]int64)
	for _, ipTime := range oldIpsWithTime {
		ipMap[ipTime.IP] = ipTime.Timestamp
	}
	for _, ipTime := range newIpsWithTime {
		if existingTime, ok := ipMap[ipTime.IP]; !ok || ipTime.Timestamp > existingTime {
			ipMap[ipTime.IP] = ipTime.Timestamp
		}
	}

	// Convert back to slice and sort by timestamp (oldest first)
	// This ensures we always protect the original/current connections and ban new excess ones.
	allIps := make([]IPWithTimestamp, 0, len(ipMap))
	for ip, timestamp := range ipMap {
		allIps = append(allIps, IPWithTimestamp{IP: ip, Timestamp: timestamp})
	}
	sort.Slice(allIps, func(i, j int) bool {
		return allIps[i].Timestamp < allIps[j].Timestamp // Ascending order (oldest first)
	})

	shouldCleanLog := false
	j.disAllowedIps = []string{}

	// Open log file
	logIpFile, err := os.OpenFile(xray.GetIPLimitLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("failed to open IP limit log file: %s", err)
		return false
	}
	defer logIpFile.Close()
	log.SetOutput(logIpFile)
	log.SetFlags(log.LstdFlags)

	// Check if we exceed the limit
	if len(allIps) > limitIp {
		shouldCleanLog = true

		// Keep the oldest IPs (currently active connections) and ban the new excess ones.
		keptIps := allIps[:limitIp]
		bannedIps := allIps[limitIp:]

		// Log banned IPs in the format fail2ban filters expect: [LIMIT_IP] Email = X || Disconnecting OLD IP = Y || Timestamp = Z
		for _, ipTime := range bannedIps {
			j.disAllowedIps = append(j.disAllowedIps, ipTime.IP)
			log.Printf("[LIMIT_IP] Email = %s || Disconnecting OLD IP = %s || Timestamp = %d", clientEmail, ipTime.IP, ipTime.Timestamp)
		}

		// Update database with only the currently active (kept) IPs
		jsonIps, _ := json.Marshal(keptIps)
		inboundClientIps.Ips = string(jsonIps)
	} else {
		// Under limit, save all IPs
		jsonIps, _ := json.Marshal(allIps)
		inboundClientIps.Ips = string(jsonIps)
	}

	db := database.GetDB()
	err = db.Save(inboundClientIps).Error
	if err != nil {
		logger.Error("failed to save inboundClientIps:", err)
		return false
	}

	if len(j.disAllowedIps) > 0 {
		logger.Infof("[LIMIT_IP] Client %s: Kept %d current IPs, queued %d new IPs for fail2ban", clientEmail, limitIp, len(j.disAllowedIps))
	}

	return shouldCleanLog
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
