package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mhsanaei/3x-ui/v2/util/common"
)

type NordService struct {
	SettingService
}

func (s *NordService) GetCountries() (string, error) {
	resp, err := http.Get("https://api.nordvpn.com/v1/countries")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *NordService) GetServers(countryId string) (string, error) {
	url := fmt.Sprintf("https://api.nordvpn.com/v2/servers?limit=0&filters[servers_technologies][id]=35&filters[country_id]=%s", countryId)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return string(body), nil
	}

	servers, ok := data["servers"].([]any)
	if !ok {
		return string(body), nil
	}

	var filtered []any
	for _, s := range servers {
		if server, ok := s.(map[string]any); ok {
			if load, ok := server["load"].(float64); ok && load > 7 {
				filtered = append(filtered, s)
			}
		}
	}
	data["servers"] = filtered

	result, _ := json.Marshal(data)
	return string(result), nil
}

func (s *NordService) SetKey(privateKey string) (string, error) {
	nordData := map[string]string{
		"private_key": privateKey,
		"token":       "", // No token for manual key
	}
	data, _ := json.Marshal(nordData)
	s.SettingService.SetNord(string(data))
	return string(data), nil
}

func (s *NordService) GetCredentials(token string) (string, error) {
	url := "https://api.nordvpn.com/v1/users/services/credentials"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth("token", token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", common.NewErrorf("NordVPN API error: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var creds map[string]any
	if err := json.Unmarshal(body, &creds); err != nil {
		return "", err
	}

	privateKey, ok := creds["nordlynx_private_key"].(string)
	if !ok || privateKey == "" {
		return "", common.NewError("failed to retrieve NordLynx private key")
	}

	nordData := map[string]string{
		"private_key": privateKey,
		"token":       token,
	}
	data, _ := json.Marshal(nordData)
	s.SettingService.SetNord(string(data))

	return string(data), nil
}

func (s *NordService) GetNordData() (string, error) {
	return s.SettingService.GetNord()
}

func (s *NordService) DelNordData() error {
	return s.SettingService.SetNord("")
}
