package job

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"time"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/xray"
)

type CheckClientIpJob struct {
	lastClear     int64
	disAllowedIps []string
}

var job *CheckClientIpJob

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

	if iplimitActive {
		if f2bInstalled && isAccessLogAvailable {
			shouldClearAccessLog = j.processLogFile()
		} else {
			if !f2bInstalled {
				logger.Warning("[LimitIP] Fail2Ban is not installed, Please install Fail2Ban from the x-ui bash menu.")
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

	accessLogPath, err := xray.GetAccessLogPath()
	j.checkError(err)

	file, err := os.Open(accessLogPath)
	j.checkError(err)

	_, err = io.Copy(logAccessP, file)
	j.checkError(err)

	logAccessP.Close()
	file.Close()

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

	ipRegex := regexp.MustCompile(`from \[?([0-9a-fA-F:.]+)\]?:\d+ accepted`)
	emailRegex := regexp.MustCompile(`email: (.+)$`)

	accessLogPath, _ := xray.GetAccessLogPath()
	file, _ := os.Open(accessLogPath)
	defer file.Close()

	inboundClientIps := make(map[string]map[string]struct{}, 100)

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

		if _, exists := inboundClientIps[email]; !exists {
			inboundClientIps[email] = make(map[string]struct{})
		}
		inboundClientIps[email][ip] = struct{}{}
	}

	shouldCleanLog := false
	for email, uniqueIps := range inboundClientIps {

		ips := make([]string, 0, len(uniqueIps))
		for ip := range uniqueIps {
			ips = append(ips, ip)
		}
		sort.Strings(ips)

		inboundClientIps, err := j.getInboundClientIps(email)
		if err != nil {
			j.addInboundClientIps(email, ips)
			continue
		}

		shouldCleanLog = j.updateInboundClientIps(inboundClientIps, email, ips) || shouldCleanLog
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

func (j *CheckClientIpJob) contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
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

func (j *CheckClientIpJob) addInboundClientIps(clientEmail string, ips []string) error {
	inboundClientIps := &model.InboundClientIps{}
	jsonIps, err := json.Marshal(ips)
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

func (j *CheckClientIpJob) updateInboundClientIps(inboundClientIps *model.InboundClientIps, clientEmail string, ips []string) bool {
	jsonIps, err := json.Marshal(ips)
	if err != nil {
		logger.Error("failed to marshal IPs to JSON:", err)
		return false
	}

	inboundClientIps.ClientEmail = clientEmail
	inboundClientIps.Ips = string(jsonIps)

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
	shouldCleanLog := false
	j.disAllowedIps = []string{}

	logIpFile, err := os.OpenFile(xray.GetIPLimitLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("failed to open IP limit log file: %s", err)
		return false
	}
	defer logIpFile.Close()
	log.SetOutput(logIpFile)
	log.SetFlags(log.LstdFlags)

	for _, client := range clients {
		if client.Email == clientEmail {
			limitIp := client.LimitIP

			if limitIp > 0 && inbound.Enable {
				shouldCleanLog = true

				if limitIp < len(ips) {
					j.disAllowedIps = append(j.disAllowedIps, ips[limitIp:]...)
					for i := limitIp; i < len(ips); i++ {
						log.Printf("[LIMIT_IP] Email = %s || SRC = %s", clientEmail, ips[i])
					}
				}
			}
		}
	}

	sort.Strings(j.disAllowedIps)

	if len(j.disAllowedIps) > 0 {
		logger.Debug("disAllowedIps:", j.disAllowedIps)
	}

	db := database.GetDB()
	err = db.Save(inboundClientIps).Error
	if err != nil {
		logger.Error("failed to save inboundClientIps:", err)
		return false
	}

	return shouldCleanLog
}

func (j *CheckClientIpJob) getInboundByEmail(clientEmail string) (*model.Inbound, error) {
	db := database.GetDB()
	var inbounds *model.Inbound

	err := db.Model(model.Inbound{}).Where("settings LIKE ?", "%"+clientEmail+"%").Find(&inbounds).Error
	if err != nil {
		return nil, err
	}

	return inbounds, nil
}
