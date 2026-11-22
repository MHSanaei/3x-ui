package service

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The server config strips downloadSettings, so building the listener cannot
// detect an unusable client leg. Validate it before persisting the inbound.
func validateXhttpDownload(raw string) error {
	var stream map[string]any
	if json.Unmarshal([]byte(raw), &stream) != nil || stream["network"] != "xhttp" {
		return nil
	}
	model.NormalizeXhttpExtra(stream)
	xh, _ := stream["xhttpSettings"].(map[string]any)
	if xh["downloadSettings"] == nil {
		return nil
	}
	down, ok := xh["downloadSettings"].(map[string]any)
	if !ok {
		return errors.New("downloadSettings must be an object")
	}
	if xh["mode"] == "stream-one" {
		return errors.New("downloadSettings is incompatible with stream-one")
	}
	network, _ := down["network"].(string)
	if down["method"] != nil {
		network, _ = down["method"].(string)
	}
	if !strings.EqualFold(network, "xhttp") && !strings.EqualFold(network, "splithttp") {
		return errors.New("downloadSettings.network must be xhttp")
	}
	if address, _ := down["address"].(string); strings.TrimSpace(address) == "" {
		return errors.New("downloadSettings.address is required")
	}
	if port, ok := down["port"].(float64); !ok || port < 1 || port > 65535 || float64(int(port)) != port {
		return errors.New("downloadSettings.port must be an integer from 1 to 65535")
	}
	security, ok := down["security"].(string)
	if !ok && down["security"] != nil {
		return errors.New("downloadSettings.security must be a string")
	}
	switch strings.ToLower(security) {
	case "", "none", "tls":
	case "reality":
		r, _ := down["realitySettings"].(map[string]any)
		key, _ := r["password"].(string)
		if key == "" {
			key, _ = r["publicKey"].(string)
		}
		if decoded, err := base64.RawURLEncoding.DecodeString(key); err != nil || len(decoded) != 32 {
			return errors.New("downloadSettings.realitySettings.password or publicKey must encode 32 bytes as unpadded base64url")
		}
		if v := r["shortId"]; v != nil {
			shortID, ok := v.(string)
			if !ok || len(shortID) > 16 {
				return errors.New("downloadSettings.realitySettings.shortId must encode at most 8 hexadecimal bytes")
			}
			if _, err := hex.DecodeString(shortID); err != nil {
				return errors.New("downloadSettings.realitySettings.shortId must encode at most 8 hexadecimal bytes")
			}
		}
	default:
		return errors.New("downloadSettings.security must be none, tls or reality")
	}
	return nil
}
