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
	"strings"
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
	f2bInstalled := j.checkFail2BanInstalled()
	isAccessLogAvailable := j.checkAccessLogAvailable(f2bInstalled)

	if j.hasLimitIp() {
		if f2bInstalled && isAccessLogAvailable {
			shouldClearAccessLog = j.processLogFile()
		} else {
			if !f2bInstalled {
				logger.Warning("[iplimit] fail2ban is not installed. IP limiting may not work properly.")
			}
		}
	}

	if shouldClearAccessLog || isAccessLogAvailable && time.Now().Unix()-j.lastClear > 3600 {
		j.clearAccessLog()
	}
}

func (j *CheckClientIpJob) clearAccessLog() {
	logAccessP, err := os.OpenFile(xray.GetAccessPersistentLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	j.checkError(err)

	// get access log path to open it
	accessLogPath, err := xray.GetAccessLogPath()
	j.checkError(err)

	// reopen the access log file for reading
	file, err := os.Open(accessLogPath)
	j.checkError(err)

	// copy access log content to persistent file
	_, err = io.Copy(logAccessP, file)
	j.checkError(err)

	// close the file after copying content
	logAccessP.Close()
	file.Close()

	// clean access log
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
	accessLogPath, err := xray.GetAccessLogPath()
	j.checkError(err)

	file, err := os.Open(accessLogPath)
	j.checkError(err)

	InboundClientIps := make(map[string][]string)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		ipRegx, _ := regexp.Compile(`(\d+\.\d+\.\d+\.\d+).* accepted`)
		emailRegx, _ := regexp.Compile(`email:.+`)

		matches := ipRegx.FindStringSubmatch(line)
		if len(matches) > 1 {
			ip := matches[1]
			if ip == "127.0.0.1" {
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

	j.checkError(scanner.Err())
	file.Close()

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

	return shouldCleanLog
}

func (j *CheckClientIpJob) checkFail2BanInstalled() bool {
	cmd := "fail2ban-client"
	args := []string{"-h"}
	err := exec.Command(cmd, args...).Run()
	return err == nil
}

func (j *CheckClientIpJob) checkAccessLogAvailable(handleWarning bool) bool {
	isAvailable := true
	warningMsg := ""
	accessLogPath, err := xray.GetAccessLogPath()
	if err != nil {
		return false
	}

	// access log is not available if it is set to 'none' or an empty string
	switch accessLogPath {
	case "none":
		warningMsg = "Access log is set to 'none', check your Xray Configs"
		isAvailable = false
	case "":
		warningMsg = "Access log doesn't exist in your Xray Configs"
		isAvailable = false
	}

	if handleWarning && warningMsg != "" {
		logger.Warning(warningMsg)
	}
	return isAvailable
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
	j.disAllowedIps = []string{}

	// create iplimit log file channel
	logIpFile, err := os.OpenFile(xray.GetIPLimitLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
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
		logger.Debug("disAllowedIps ", j.disAllowedIps)
	}

	db := database.GetDB()
	err = db.Save(inboundClientIps).Error
	j.checkError(err)

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
