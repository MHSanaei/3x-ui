package job

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/xray"
)

type CheckClientIpJob struct{}

var job *CheckClientIpJob
var disAllowedIps []string
var ipFiles = []string{
	xray.GetIPLimitLogPath(),
	xray.GetIPLimitBannedLogPath(),
	xray.GetAccessPersistentLogPath(),
}

func NewCheckClientIpJob() *CheckClientIpJob {
	job = new(CheckClientIpJob)
	return job
}

func (j *CheckClientIpJob) Run() {

	// create files required for iplimit if not exists
	for i := 0; i < len(ipFiles); i++ {
		file, err := os.OpenFile(ipFiles[i], os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
		j.checkError(err)
		defer file.Close()
	}

	// check for limit ip
	if j.hasLimitIp() {
		j.checkFail2BanInstalled()
		j.processLogFile()
	}
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

func (j *CheckClientIpJob) checkFail2BanInstalled() {
	cmd := "fail2ban-client"
	args := []string{"-h"}

	err := exec.Command(cmd, args...).Run()
	if err != nil {
		logger.Warning("fail2ban is not installed. IP limiting may not work properly.")
	}
}

func (j *CheckClientIpJob) processLogFile() {
	accessLogPath := xray.GetAccessLogPath()
	if accessLogPath == "" {
		logger.Warning("access.log doesn't exist in your config.json")
		return
	}

	data, err := os.ReadFile(accessLogPath)
	InboundClientIps := make(map[string][]string)
	j.checkError(err)

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		ipRegx, _ := regexp.Compile(`[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+`)
		emailRegx, _ := regexp.Compile(`email:.+`)

		matchesIp := ipRegx.FindString(line)
		if len(matchesIp) > 0 {
			ip := string(matchesIp)
			if ip == "127.0.0.1" || ip == "1.1.1.1" {
				continue
			}

			matchesEmail := emailRegx.FindString(line)
			if matchesEmail == "" {
				continue
			}
			matchesEmail = strings.TrimSpace(strings.Split(matchesEmail, "email: ")[1])

			if InboundClientIps[matchesEmail] != nil {
				if j.contains(InboundClientIps[matchesEmail], ip) {
					continue
				}
				InboundClientIps[matchesEmail] = append(InboundClientIps[matchesEmail], ip)

			} else {
				InboundClientIps[matchesEmail] = append(InboundClientIps[matchesEmail], ip)
			}
		}
	}

	disAllowedIps = []string{}
	shouldCleanLog := false

	for clientEmail, ips := range InboundClientIps {
		inboundClientIps, err := j.getInboundClientIps(clientEmail)
		sort.Strings(ips)
		if err != nil {
			j.addInboundClientIps(clientEmail, ips)
		} else {
			shouldCleanLog = j.updateInboundClientIps(inboundClientIps, clientEmail, ips)
		}

	}

	// added delay before cleaning logs to reduce chance of logging IP that already has been banned
	time.Sleep(time.Second * 2)

	if shouldCleanLog {
		// copy access log to persistent file
		logAccessP, err := os.OpenFile(xray.GetAccessPersistentLogPath(), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
		j.checkError(err)
		input, err := os.ReadFile(accessLogPath)
		j.checkError(err)
		if _, err := logAccessP.Write(input); err != nil {
			j.checkError(err)
		}
		defer logAccessP.Close()

		// clean access log
		if err := os.Truncate(xray.GetAccessLogPath(), 0); err != nil {
			j.checkError(err)
		}
	}
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
	j.checkError(err)

	inboundClientIps.ClientEmail = clientEmail
	inboundClientIps.Ips = string(jsonIps)

	// check inbound limitation
	inbound, err := j.getInboundByEmail(clientEmail)
	j.checkError(err)

	if inbound.Settings == "" {
		logger.Debug("wrong data ", inbound)
		return false
	}

	settings := map[string][]model.Client{}
	json.Unmarshal([]byte(inbound.Settings), &settings)
	clients := settings["clients"]
	shouldCleanLog := false

	// create iplimit log file channel
	logIpFile, err := os.OpenFile(xray.GetIPLimitLogPath(), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		logger.Errorf("failed to create or open ip limit log file: %s", err)
	}
	defer logIpFile.Close()
	log.SetOutput(logIpFile)
	log.SetFlags(log.LstdFlags)

	for _, client := range clients {
		if client.Email == clientEmail {
			limitIp := client.LimitIP

			if limitIp != 0 {
				shouldCleanLog = true

				if limitIp < len(ips) && inbound.Enable {
					disAllowedIps = append(disAllowedIps, ips[limitIp:]...)
					for i := limitIp; i < len(ips); i++ {
						log.Printf("[LIMIT_IP] Email = %s || SRC = %s", clientEmail, ips[i])
					}
				}
			}
		}
	}
	logger.Debug("disAllowedIps ", disAllowedIps)
	sort.Strings(disAllowedIps)

	db := database.GetDB()
	err = db.Save(inboundClientIps).Error
	if err != nil {
		return shouldCleanLog
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
