package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/mhsanaei/3x-ui/v3/util/common"
)

// WarpService provides business logic for Cloudflare WARP integration.
// It manages WARP configuration and connectivity settings.
type WarpService struct {
	SettingService
}

const (
	warpAPIBase   = "https://api.cloudflareclient.com/v0a4005"
	warpClientVer = "a-6.30-3596"
)

var warpHTTPClient = &http.Client{Timeout: 15 * time.Second}

func (s *WarpService) GetWarpData() (string, error) {
	return s.SettingService.GetWarp()
}

func (s *WarpService) DelWarpData() error {
	return s.SettingService.SetWarp("")
}

func (s *WarpService) GetWarpConfig() (string, error) {
	warpData, err := s.loadWarpCreds()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/reg/%s", warpAPIBase, warpData["device_id"])
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+warpData["access_token"])

	body, err := doWarpRequest(req)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (s *WarpService) RegWarp(secretKey string, publicKey string) (string, error) {
	hostName, _ := os.Hostname()
	reqBody, err := json.Marshal(map[string]any{
		"key":   publicKey,
		"tos":   time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"type":  "PC",
		"model": "x-ui",
		"name":  hostName,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, warpAPIBase+"/reg", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("CF-Client-Version", warpClientVer)
	req.Header.Set("Content-Type", "application/json")

	body, err := doWarpRequest(req)
	if err != nil {
		return "", err
	}

	var rsp map[string]any
	if err := json.Unmarshal(body, &rsp); err != nil {
		return "", err
	}

	deviceID, ok := rsp["id"].(string)
	if !ok {
		return "", common.NewError("warp register: missing 'id' in response")
	}
	token, ok := rsp["token"].(string)
	if !ok {
		return "", common.NewError("warp register: missing 'token' in response")
	}
	account, ok := rsp["account"].(map[string]any)
	if !ok {
		return "", common.NewError("warp register: missing 'account' in response")
	}
	license, ok := account["license"].(string)
	if !ok {
		return "", common.NewError("warp register: missing 'account.license' in response")
	}

	warpData := map[string]string{
		"access_token": token,
		"device_id":    deviceID,
		"license_key":  license,
		"private_key":  secretKey,
	}
	warpJSON, err := json.MarshalIndent(warpData, "", "  ")
	if err != nil {
		return "", err
	}
	if err := s.SettingService.SetWarp(string(warpJSON)); err != nil {
		return "", err
	}

	result, err := json.MarshalIndent(map[string]any{
		"data":   warpData,
		"config": json.RawMessage(body),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func (s *WarpService) SetWarpLicense(license string) (string, error) {
	warpData, err := s.loadWarpCreds()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/reg/%s/account", warpAPIBase, warpData["device_id"])
	reqBody, err := json.Marshal(map[string]string{"license": license})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+warpData["access_token"])
	req.Header.Set("Content-Type", "application/json")

	body, err := doWarpRequest(req)
	if err != nil {
		return "", err
	}

	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if _, ok := response["id"].(string); !ok {
		return "", common.NewErrorf("warp set license failed: unexpected response: %s", string(body))
	}

	warpData["license_key"] = license
	newWarpData, err := json.MarshalIndent(warpData, "", "  ")
	if err != nil {
		return "", err
	}
	if err := s.SettingService.SetWarp(string(newWarpData)); err != nil {
		return "", err
	}
	return string(newWarpData), nil
}

// loadWarpCreds reads the stored warp JSON and ensures access_token + device_id are set.
func (s *WarpService) loadWarpCreds() (map[string]string, error) {
	warp, err := s.SettingService.GetWarp()
	if err != nil {
		return nil, err
	}
	var data map[string]string
	if err := json.Unmarshal([]byte(warp), &data); err != nil {
		return nil, err
	}
	if data["access_token"] == "" || data["device_id"] == "" {
		return nil, common.NewError("warp not registered: missing access_token or device_id")
	}
	return data, nil
}

// doWarpRequest sends the request and returns the response body on 2xx.
// Non-2xx responses are returned as errors including the status code and body.
func doWarpRequest(req *http.Request) ([]byte, error) {
	resp, err := warpHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if msg := parseWarpError(body); msg != "" {
			return nil, common.NewError(msg)
		}
		return nil, common.NewErrorf("warp api %s %s returned status %d: %s",
			req.Method, req.URL.Path, resp.StatusCode, string(body))
	}
	return body, nil
}

func parseWarpError(body []byte) string {
	var env struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return ""
	}
	if len(env.Errors) == 0 || env.Errors[0].Message == "" {
		return ""
	}
	return env.Errors[0].Message
}
