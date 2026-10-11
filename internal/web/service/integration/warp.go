package integration

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// WarpService provides business logic for Cloudflare WARP integration.
// It manages WARP configuration and connectivity settings.
type WarpService struct {
	service.SettingService
}

const (
	warpClientVer = "a-6.30-3596"
)

// warpAPIBase is the Cloudflare WARP registration API base URL. It is a var
// (not a const) so integration tests can point it at a mock server.
var warpAPIBase = "https://api.cloudflareclient.com/v0a4005"

// warpMasqueAPIBase and warpMasqueClientVer are the API revision and client the WARP
// apps enroll secp256r1 MASQUE keys with; a var so tests can point it at a mock.
var warpMasqueAPIBase = "https://api.cloudflareclient.com/v0a4471"

const warpMasqueClientVer = "a-6.35-4471"

func (s *WarpService) GetWarpData() (string, error) {
	return s.GetWarp()
}

func (s *WarpService) DelWarpData() error {
	return s.SetWarp("")
}

func (s *WarpService) GetWarpConfig() (string, error) {
	warpData, err := s.loadWarpCreds()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/reg/%s", warpAPIBase, warpData["device_id"])
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+warpData["access_token"])

	body, err := s.doWarpRequest(req)
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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, warpAPIBase+"/reg", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("CF-Client-Version", warpClientVer)
	req.Header.Set("Content-Type", "application/json")

	body, err := s.doWarpRequest(req)
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
	if config, ok := rsp["config"].(map[string]any); ok {
		if clientID, ok := config["client_id"].(string); ok {
			warpData["client_id"] = clientID
		}
	}
	warpJSON, err := json.MarshalIndent(warpData, "", "  ")
	if err != nil {
		return "", err
	}
	if err := s.SetWarp(string(warpJSON)); err != nil {
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

// RegWarpMasque registers a WARP device of its own and enrolls an ECDSA P-256 key for
// MASQUE: enrolling replaces a device's WireGuard key, so the stored one stays untouched.
func (s *WarpService) RegWarpMasque() (string, error) {
	_, wgPublicKey, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		return "", err
	}
	hostName, _ := os.Hostname()
	regBody, err := json.Marshal(map[string]any{
		"key":   wgPublicKey,
		"tos":   time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"type":  "PC",
		"model": "x-ui",
		"name":  hostName,
	})
	if err != nil {
		return "", err
	}
	var registration struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if err := s.doWarpMasqueRequest(http.MethodPost, warpMasqueAPIBase+"/reg", "", regBody, &registration); err != nil {
		return "", err
	}
	if registration.ID == "" || registration.Token == "" {
		return "", common.NewError("warp masque register: missing 'id' or 'token' in response")
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", err
	}
	enrollBody, err := json.Marshal(map[string]string{
		"key":         base64.StdEncoding.EncodeToString(publicDER),
		"key_type":    "secp256r1",
		"tunnel_type": "masque",
		"name":        hostName,
	})
	if err != nil {
		return "", err
	}
	var enrolled struct {
		Config struct {
			Interface struct {
				Addresses struct {
					V4 string `json:"v4"`
					V6 string `json:"v6"`
				} `json:"addresses"`
			} `json:"interface"`
			Peers []struct {
				PublicKey string `json:"public_key"`
				Endpoint  struct {
					V4 string `json:"v4"`
				} `json:"endpoint"`
			} `json:"peers"`
		} `json:"config"`
	}
	if err := s.doWarpMasqueRequest(http.MethodPatch, warpMasqueAPIBase+"/reg/"+registration.ID, registration.Token, enrollBody, &enrolled); err != nil {
		return "", err
	}
	if len(enrolled.Config.Peers) == 0 || enrolled.Config.Peers[0].PublicKey == "" {
		return "", common.NewError("warp masque enroll: missing endpoint public key in response")
	}
	peer := enrolled.Config.Peers[0]
	endpoint, _, err := net.SplitHostPort(peer.Endpoint.V4)
	if err != nil {
		endpoint = peer.Endpoint.V4
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	var address []string
	for _, a := range []string{enrolled.Config.Interface.Addresses.V4, enrolled.Config.Interface.Addresses.V6} {
		if a != "" {
			address = append(address, a)
		}
	}
	result, err := json.MarshalIndent(map[string]any{
		"privateKey": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
		"publicKey":  peer.PublicKey,
		"address":    address,
		"endpoint":   endpoint,
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func (s *WarpService) doWarpMasqueRequest(method, url, token string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(context.Background(), method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("CF-Client-Version", warpMasqueClientVer)
	req.Header.Set("User-Agent", "WARP for Android")
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	respBody, err := s.doWarpRequest(req)
	if err != nil {
		return err
	}
	return json.Unmarshal(respBody, out)
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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+warpData["access_token"])
	req.Header.Set("Content-Type", "application/json")

	body, err := s.doWarpRequest(req)
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
	if err := s.SetWarp(string(newWarpData)); err != nil {
		return "", err
	}
	return string(newWarpData), nil
}

func (s *WarpService) ChangeWarpIP() (string, error) {
	warpDataMap, err := s.loadWarpCreds()
	if err != nil {
		return "", err
	}

	privKey, pubKey, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		return "", err
	}

	result, err := s.RegWarp(privKey, pubKey)
	if err != nil {
		return "", err
	}

	var parsed struct {
		Data   map[string]string `json:"data"`
		Config map[string]any    `json:"config"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		return "", err
	}

	// RegWarp stores the new device's data, which for a fresh registration
	// carries an empty license_key. Re-apply the old license key to the stored
	// data BEFORE the remote upgrade attempt, so a failed re-apply can never
	// delete the saved key.
	var reapplyWarn error
	if license, ok := warpDataMap["license_key"]; ok && len(license) >= 26 {
		if parsed.Data == nil {
			parsed.Data = make(map[string]string)
		}
		parsed.Data["license_key"] = license
		if stored, err := json.MarshalIndent(parsed.Data, "", "  "); err != nil {
			return "", err
		} else if err := s.SetWarp(string(stored)); err != nil {
			return "", err
		}
	}

	xraySvc := service.XraySettingService{}
	if err := xraySvc.UpdateWarpXraySetting(parsed.Data, parsed.Config); err != nil {
		return "", err
	}

	if license, ok := warpDataMap["license_key"]; ok && len(license) >= 26 {
		if _, licErr := s.SetWarpLicense(license); licErr != nil {
			// The key is already preserved in storage above; surface the
			// remote failure instead of silently downgrading to a free account.
			reapplyWarn = licErr
			logger.Warning("ChangeWarpIP: failed to re-apply WARP license (key preserved in storage): ", licErr)
		}
	}

	// Return the final stored data (with the preserved license key) instead of
	// RegWarp's snapshot, which always carries an empty license.
	response := map[string]any{
		"data":   parsed.Data,
		"config": parsed.Config,
	}
	if reapplyWarn != nil {
		response["warning"] = fmt.Sprintf("failed to re-apply WARP license: %v", reapplyWarn)
	}
	resultJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", err
	}
	return string(resultJSON), nil
}

// loadWarpCreds reads the stored warp JSON and ensures access_token + device_id are set.
func (s *WarpService) loadWarpCreds() (map[string]string, error) {
	warp, err := s.GetWarp()
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
func (s *WarpService) doWarpRequest(req *http.Request) ([]byte, error) {
	client := s.NewProxiedHTTPClient(15 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
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
