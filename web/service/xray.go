package service

import (
	"encoding/json"
	"errors"
	"runtime"
	"sync"
	"strconv"

	"x-ui/logger"
	"x-ui/xray"
	json_util "x-ui/util/json_util"

	"go.uber.org/atomic"
)

var (
	p                 *xray.Process
	lock              sync.Mutex
	isNeedXrayRestart atomic.Bool // Indicates that restart was requested for Xray
	isManuallyStopped atomic.Bool // Indicates that Xray was stopped manually from the panel
	result            string
)

type XrayService struct {
	inboundService InboundService
	settingService SettingService
	xrayAPI        xray.XrayAPI
}

func (s *XrayService) IsXrayRunning() bool {
	return p != nil && p.IsRunning()
}

func (s *XrayService) GetApiPort() int {
	if p == nil {
		return 0
	}
	return p.GetAPIPort()
}

func (s *XrayService) GetXrayErr() error {
	if p == nil {
		return nil
	}

	err := p.GetErr()

	if runtime.GOOS == "windows" && err.Error() == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return nil
	}

	return err
}

func (s *XrayService) GetXrayResult() string {
	if result != "" {
		return result
	}
	if s.IsXrayRunning() {
		return ""
	}
	if p == nil {
		return ""
	}

	result = p.GetResult()

	if runtime.GOOS == "windows" && result == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return ""
	}

	return result
}

func (s *XrayService) GetXrayVersion() string {
	if p == nil {
		return "Unknown"
	}
	return p.GetVersion()
}

func RemoveIndex(s []any, index int) []any {
	return append(s[:index], s[index+1:]...)
}

func (s *XrayService) GetXrayConfig() (*xray.Config, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}

	xrayConfig := &xray.Config{}
	err = json.Unmarshal([]byte(templateConfig), xrayConfig)
	if err != nil {
		return nil, err
	}

	

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}

	
	uniqueSpeeds := make(map[int]bool)
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		
        
		dbClients, _ := s.inboundService.GetClients(inbound)
		for _, dbClient := range dbClients {
			if dbClient.SpeedLimit > 0 {
				uniqueSpeeds[dbClient.SpeedLimit] = true
			}
		}
	}

	
	var finalPolicy map[string]interface{}
	if xrayConfig.Policy != nil {
		if err := json.Unmarshal(xrayConfig.Policy, &finalPolicy); err != nil {
			logger.Warningf("Failed to parse the policy in the template: %v", err)
			finalPolicy = make(map[string]interface{})
		}
	} else {
		finalPolicy = make(map[string]interface{})
	}

	
	var policyLevels map[string]interface{}
	if levels, ok := finalPolicy["levels"].(map[string]interface{}); ok {
		policyLevels = levels
	} else {
		policyLevels = make(map[string]interface{})
	}

	// 3. [Important modification]: Ensure the integrity of the level 0 policy, which is key to enabling device restrictions and default user statistics.
	var level0 map[string]interface{}
	if l0, ok := policyLevels["0"].(map[string]interface{}); ok {
		// If level 0 already exists in the template, use it as the base for modifications.
		level0 = l0
	} else {
		//  If it does not exist in the template, create a brand new map.
		level0 = make(map[string]interface{})
	}
	// [Chinese comment]: Regardless of whether level 0 exists, supplement or override the following key parameters for it.
	// handshake and connIdle are prerequisites to activate Xray connection statistics,
	// uplinkOnly and downlinkOnly set to 0 mean no speed limit, which is the default behavior for level 0 users.
	// statsUserUplink and statsUserDownlink ensure that user traffic can be counted.
	level0["handshake"] = 4
	level0["connIdle"] = 300
	level0["uplinkOnly"] = 0
	level0["downlinkOnly"] = 0
	level0["statsUserUplink"] = true
	level0["statsUserDownlink"] = true 
	// [Added]: Add this key option to enable Xray-core's online IP statistics feature.
	// This is a prerequisite for the proper functioning of the "device restriction" feature.

	level0["statsUserOnline"] = true
	
	// Write the fully configured level 0 back to policyLevels to ensure the final generated config.json is correct.
	policyLevels["0"] = level0

	// 4. Iterate through all collected speed limits and create a corresponding level for each unique speed limit
	for speed := range uniqueSpeeds {
		// Create a level for each speed, where the level's name is the string representation of the speed
		// For example, the speed 1024 KB/s corresponds to the level "1024"
		policyLevels[strconv.Itoa(speed)] = map[string]interface{}{
			"downlinkOnly": speed,
			"uplinkOnly":   speed,
			"handshake":         4,
			"connIdle":          300,
			"statsUserUplink":   true,
			"statsUserDownlink": true,
			"statsUserOnline": true,
		}
	}
	// 5. Write the modified levels back to the policy object, serialize it back to xrayConfig.Policy, and apply the generated policy to the Xray configuration
	finalPolicy["levels"] = policyLevels
	policyJSON, err := json.Marshal(finalPolicy)
	if err != nil {
		return nil, err
	}
	xrayConfig.Policy = json_util.RawMessage(policyJSON)
	// =================================================================
	//  Add logs here to print the final generated speed limit policy
	// =================================================================

	if len(uniqueSpeeds) > 0 {
		finalPolicyLog, _ := json.Marshal(policyLevels)
		logger.Infof("已为Xray动态生成〔限速策略〕: %s", string(finalPolicyLog))
	}


	s.inboundService.AddTraffic(nil, nil)

	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		
		inboundConfig := inbound.GenXrayInboundConfig()

		
		speedByEmail := make(map[string]int)
		speedById := make(map[string]int)
		dbClients, _ := s.inboundService.GetClients(inbound)
		for _, dbc := range dbClients {
			if dbc.Email != "" {
				speedByEmail[dbc.Email] = dbc.SpeedLimit
			}
			
			if dbc.ID != "" {
				speedById[dbc.ID] = dbc.SpeedLimit
			}
		}

		
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			logger.Warningf("Failed to parse inbound.Settings (inbound %d): %v, skipping this inbound", inbound.Id, err)
			continue
		}

		originalClients, ok := settings["clients"].([]interface{})
		if ok {
			clientStats := inbound.ClientStats

			var xrayClients []interface{}
			for _, clientRaw := range originalClients {
				c, ok := clientRaw.(map[string]interface{})
				if !ok {
					continue
				}
			
				if en, ok := c["enable"].(bool); ok && !en {
					if em, _ := c["email"].(string); em != "" {
						logger.Infof("User marked as disabled in settings removed from Xray config: %s", em)
					}
					continue
				}

				
				email, _ := c["email"].(string)
				idStr, _ := c["id"].(string)
				disabledByStat := false
				for _, stat := range clientStats {
					if stat.Email == email && !stat.Enable {
						disabledByStat = true
						break
					}
				}
				if disabledByStat {
					logger.Infof("User disabled and removed from Xray config: %s", email)
					continue
				}


				
				xrayClient := make(map[string]interface{})
				if id, ok := c["id"]; ok { xrayClient["id"] = id }
				if email != "" { xrayClient["email"] = email }

				
				if flow, ok := c["flow"]; ok {
					if fs, ok2 := flow.(string); ok2 && fs == "xtls-rprx-vision-udp443" {
						xrayClient["flow"] = "xtls-rprx-vision"
					} else {
						xrayClient["flow"] = flow
					}
				}
				if password, ok := c["password"]; ok { xrayClient["password"] = password }
				if method, ok := c["method"]; ok { xrayClient["method"] = method }

				
				level := 0
				if email != "" {
					if v, ok := speedByEmail[email]; ok && v > 0 {
						level = v
					}
				}
				if level == 0 && idStr != "" {
					if v, ok := speedById[idStr]; ok && v > 0 {
						level = v
					}
				}
				if level == 0 {
					if sl, ok := c["speedLimit"]; ok {
						switch vv := sl.(type) {
						case float64:
							level = int(vv)
						case int:
							level = vv
						case int64:
							level = int(vv)
						case string:
							if n, err := strconv.Atoi(vv); err == nil {
								level = n
							}
						}
					}
				}
				
				if level > 0 && email != "" {
					logger.Infof("Applied independent speed limit for user %s: %d KB/s", email, level)
				}

				// =================================================================

				xrayClient["level"] = level

				xrayClients = append(xrayClients, xrayClient)
			}
			
			settings["clients"] = xrayClients
			finalSettingsForXray, err := json.Marshal(settings)
			if err != nil {
				logger.Warningf("Failed to serialize inbound settings for Xray in GetXrayConfig for inbound %d: %v, skipping this inbound", inbound.Id, err)
				continue
			}
			inboundConfig.Settings = json_util.RawMessage(finalSettingsForXray)
		}
		// get settings clients
		settings := map[string]any{}
		json.Unmarshal([]byte(inbound.Settings), &settings)
		clients, ok := settings["clients"].([]any)
		if ok {
			// check users active or not
			clientStats := inbound.ClientStats
			for _, clientTraffic := range clientStats {
				indexDecrease := 0
				for index, client := range clients {
					c := client.(map[string]any)
					if c["email"] == clientTraffic.Email {
						if !clientTraffic.Enable {
							clients = RemoveIndex(clients, index-indexDecrease)
							indexDecrease++
							logger.Infof("Remove Inbound User %s due to expiration or traffic limit", c["email"])
						}
					}
				}
			}

			// clear client config for additional parameters
			var final_clients []any
			for _, client := range clients {
				c := client.(map[string]any)
				if c["enable"] != nil {
					if enable, ok := c["enable"].(bool); ok && !enable {
						continue
					}
				}
				for key := range c {
					if key != "email" && key != "id" && key != "password" && key != "flow" && key != "method" {
						delete(c, key)
					}
					if c["flow"] == "xtls-rprx-vision-udp443" {
						c["flow"] = "xtls-rprx-vision"
					}
				}
				final_clients = append(final_clients, any(c))
			}

			settings["clients"] = final_clients
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return nil, err
			}

			inbound.Settings = string(modifiedSettings)
		}

		if len(inbound.StreamSettings) > 0 {
			// Unmarshal stream JSON
			var stream map[string]any
			json.Unmarshal([]byte(inbound.StreamSettings), &stream)

			// Remove the "settings" field under "tlsSettings" and "realitySettings"
			tlsSettings, ok1 := stream["tlsSettings"].(map[string]any)
			realitySettings, ok2 := stream["realitySettings"].(map[string]any)
			if ok1 || ok2 {
				if ok1 {
					delete(tlsSettings, "settings")
				} else if ok2 {
					delete(realitySettings, "settings")
				}
			}

			delete(stream, "externalProxy")

			newStream, err := json.MarshalIndent(stream, "", "  ")
			if err != nil {
				return nil, err
			}
			inbound.StreamSettings = string(newStream)
		}

		
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}
	return xrayConfig, nil
}

func (s *XrayService) GetXrayTraffic() ([]*xray.Traffic, []*xray.ClientTraffic, error) {
	if !s.IsXrayRunning() {
		err := errors.New("xray is not running")
		logger.Debug("Attempted to fetch Xray traffic, but Xray is not running:", err)
		return nil, nil, err
	}
	apiPort := p.GetAPIPort()
	s.xrayAPI.Init(apiPort)
	defer s.xrayAPI.Close()

	traffic, clientTraffic, err := s.xrayAPI.GetTraffic(true)
	if err != nil {
		logger.Debug("Failed to fetch Xray traffic:", err)
		return nil, nil, err
	}
	return traffic, clientTraffic, nil
}

func (s *XrayService) RestartXray(isForce bool) error {
	lock.Lock()
	defer lock.Unlock()
	logger.Debug("restart Xray, force:", isForce)
	isManuallyStopped.Store(false)

	xrayConfig, err := s.GetXrayConfig()
	if err != nil {
		return err
	}

	if s.IsXrayRunning() {
		if !isForce && p.GetConfig().Equals(xrayConfig) && !isNeedXrayRestart.Load() {
			logger.Debug("It does not need to restart Xray")
			return nil
		}
		p.Stop()
	}

	p = xray.NewProcess(xrayConfig)
	result = ""
	err = p.Start()
	if err != nil {
		return err
	}

	return nil
}

func (s *XrayService) StopXray() error {
	lock.Lock()
	defer lock.Unlock()
	isManuallyStopped.Store(true)
	logger.Debug("Attempting to stop Xray...")
	if s.IsXrayRunning() {
		return p.Stop()
	}
	return errors.New("xray is not running")
}

func (s *XrayService) SetToNeedRestart() {
	isNeedXrayRestart.Store(true)
}

func (s *XrayService) IsNeedRestartAndSetFalse() bool {
	return isNeedXrayRestart.CompareAndSwap(true, false)
}

// Check if Xray is not running and wasn't stopped manually, i.e. crashed
func (s *XrayService) DidXrayCrash() bool {
	return !s.IsXrayRunning() && !isManuallyStopped.Load()
}