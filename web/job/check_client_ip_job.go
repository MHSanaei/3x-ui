package job

import (
	"encoding/json"
	"os"
	"regexp"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/web/service"
	"x-ui/xray"

	"sort"
	"strings"
	"time"
)

type CheckClientIpJob struct {
	xrayService service.XrayService
}

var job *CheckClientIpJob
var disAllowedIps []string

func NewCheckClientIpJob() *CheckClientIpJob {
	job = new(CheckClientIpJob)
	return job
}

func (j *CheckClientIpJob) Run() {
	logger.Debug("Check Client IP Job...")

	if hasLimitIp() {
		processLogFile()
	}

	blockedIps := []byte(strings.Join(disAllowedIps, ","))

	// check if file exists, if not create one
	_, err := os.Stat(xray.GetBlockedIPsPath())
	if os.IsNotExist(err) {
		_, err = os.OpenFile(xray.GetBlockedIPsPath(), os.O_RDWR|os.O_CREATE, 0755)
		checkError(err)
	}
	err = os.WriteFile(xray.GetBlockedIPsPath(), blockedIps, 0755)
	checkError(err)
}

func hasLimitIp() bool {
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

func processLogFile() {
	accessLogPath := GetAccessLogPath()
	if accessLogPath == "" {
		logger.Warning("access.log doesn't exist in your config.json")
		return
	}

	data, err := os.ReadFile(accessLogPath)
	InboundClientIps := make(map[string][]string)
	checkError(err)

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
				if contains(InboundClientIps[matchesEmail], ip) {
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
		inboundClientIps, err := GetInboundClientIps(clientEmail)
		sort.Strings(ips)
		if err != nil {
			addInboundClientIps(clientEmail, ips)

		} else {
			shouldCleanLog = updateInboundClientIps(inboundClientIps, clientEmail, ips)
		}

	}

	time.Sleep(time.Second * 5)
	//added 5 seconds delay before cleaning logs to reduce chance of logging IP that already has been banned
	if shouldCleanLog {
		// clean log
		if err := os.Truncate(GetAccessLogPath(), 0); err != nil {
			checkError(err)
		}
	}

}
func GetAccessLogPath() string {

	config, err := os.ReadFile(xray.GetConfigPath())
	checkError(err)

	jsonConfig := map[string]interface{}{}
	err = json.Unmarshal([]byte(config), &jsonConfig)
	checkError(err)
	if jsonConfig["log"] != nil {
		jsonLog := jsonConfig["log"].(map[string]interface{})
		if jsonLog["access"] != nil {

			accessLogPath := jsonLog["access"].(string)

			return accessLogPath
		}
	}
	return ""

}
func checkError(e error) {
	if e != nil {
		logger.Warning("client ip job err:", e)
	}
}
func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
func GetInboundClientIps(clientEmail string) (*model.InboundClientIps, error) {
	db := database.GetDB()
	InboundClientIps := &model.InboundClientIps{}
	err := db.Model(model.InboundClientIps{}).Where("client_email = ?", clientEmail).First(InboundClientIps).Error
	if err != nil {
		return nil, err
	}
	return InboundClientIps, nil
}
func addInboundClientIps(clientEmail string, ips []string) error {
	inboundClientIps := &model.InboundClientIps{}
	jsonIps, err := json.Marshal(ips)
	checkError(err)

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
func updateInboundClientIps(inboundClientIps *model.InboundClientIps, clientEmail string, ips []string) bool {

	jsonIps, err := json.Marshal(ips)
	checkError(err)

	inboundClientIps.ClientEmail = clientEmail
	inboundClientIps.Ips = string(jsonIps)

	// check inbound limitation
	inbound, err := GetInboundByEmail(clientEmail)
	checkError(err)

	if inbound.Settings == "" {
		logger.Debug("wrong data ", inbound)
		return false
	}

	settings := map[string][]model.Client{}
	json.Unmarshal([]byte(inbound.Settings), &settings)
	clients := settings["clients"]
	shouldCleanLog := false

	for _, client := range clients {
		if client.Email == clientEmail {

			limitIp := client.LimitIP

			if limitIp != 0 {

				shouldCleanLog = true

				if limitIp < len(ips) && inbound.Enable {

					disAllowedIps = append(disAllowedIps, ips[limitIp:]...)
					for i := limitIp; i < len(ips); i++ {
						logger.Info("[LIMIT_IP] Email=", clientEmail, " SRC=", ips[i])
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

func DisableInbound(id int) error {
	db := database.GetDB()
	result := db.Model(model.Inbound{}).
		Where("id = ? and enable = ?", id, true).
		Update("enable", false)
	err := result.Error
	logger.Warning("disable inbound with id:", id)

	if err == nil {
		job.xrayService.SetToNeedRestart()
	}

	return err
}

func GetInboundByEmail(clientEmail string) (*model.Inbound, error) {
	db := database.GetDB()
	var inbounds *model.Inbound
	err := db.Model(model.Inbound{}).Where("settings LIKE ?", "%"+clientEmail+"%").Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	return inbounds, nil
}
