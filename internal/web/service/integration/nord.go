package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type NordService struct {
	service.SettingService
}

var nordHTTPClient = &http.Client{Timeout: 15 * time.Second}

// maxResponseSize limits the maximum size of NordVPN API responses (10 MB).
const maxResponseSize = 10 << 20

func (s *NordService) GetCountries() (string, error) {
	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.nordvpn.com/v1/servers/countries?filters[servers_technologies][identifier]=wireguard_udp", nil)
	if reqErr != nil {
		return "", reqErr
	}
	resp, err := nordHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", common.NewErrorf("NordVPN API error: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *NordService) GetServers(countryId string) (string, error) {
	// Validate countryId is numeric to prevent URL injection
	for _, c := range countryId {
		if c < '0' || c > '9' {
			return "", common.NewError("invalid country ID")
		}
	}
	url := fmt.Sprintf("https://api.nordvpn.com/v2/servers?limit=0&filters[servers_technologies][identifier]=wireguard_udp&filters[country_id]=%s", countryId)
	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if reqErr != nil {
		return "", reqErr
	}
	resp, err := nordHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", common.NewErrorf("NordVPN API error: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *NordService) SetKey(privateKey string) (string, error) {
	if privateKey == "" {
		return "", common.NewError("private key cannot be empty")
	}
	nordData := map[string]string{
		"private_key": privateKey,
		"token":       "",
	}
	data, _ := json.Marshal(nordData)
	err := s.SetNord(string(data))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *NordService) GetCredentials(token string) (string, error) {
	url := "https://api.nordvpn.com/v1/users/services/credentials"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth("token", token)

	resp, err := nordHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", common.NewErrorf("NordVPN API error: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
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
	err = s.SetNord(string(data))
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (s *NordService) GetNordData() (string, error) {
	return s.GetNord()
}

func (s *NordService) DelNordData() error {
	return s.SetNord("")
}
