package service

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	xtls "github.com/xtls/xray-core/transport/internet/tls"
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
	case "", "none":
	case "tls":
		t, _ := down["tlsSettings"].(map[string]any)
		if err := checkXhttpDownloadFingerprint(t, "tlsSettings", true); err != nil {
			return err
		}
	case "reality":
		r, _ := down["realitySettings"].(map[string]any)
		if err := checkXhttpDownloadFingerprint(r, "realitySettings", false); err != nil {
			return err
		}
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

// checkXhttpDownloadFingerprint applies the core's client-side rule: an unknown uTLS name,
// or one REALITY refuses, fails the build of every client that imports the link.
func checkXhttpDownloadFingerprint(settings map[string]any, block string, allowUnsafe bool) error {
	raw, has := settings["fingerprint"]
	if !has || raw == nil {
		return nil
	}
	name, ok := raw.(string)
	name = strings.ToLower(name)
	insecure := name == "unsafe" || name == "hellogolang"
	switch {
	case !ok, insecure && !allowUnsafe, name != "unsafe" && xtls.GetFingerprint(name) == nil:
		return errors.New("downloadSettings." + block + ".fingerprint is not a uTLS fingerprint this security accepts")
	}
	return nil
}

// What the listener reads from every request with its own config, the download GET included.
var xhttpListenerCheckedKeys = []string{
	"sessionIDPlacement", "sessionIDKey", "sessionIDTable", "sessionIDLength",
	"seqPlacement", "seqKey",
	"xPaddingBytes", "xPaddingObfsMode", "xPaddingKey", "xPaddingHeader", "xPaddingPlacement", "xPaddingMethod",
}

// syncXhttpDownloadRequestShape makes the download leg carry the inbound's own session and
// padding rules, so an edit of the inbound cannot leave a saved leg the listener rejects.
func syncXhttpDownloadRequestShape(streamSettings string) string {
	var stream map[string]any
	if json.Unmarshal([]byte(streamSettings), &stream) != nil || stream["network"] != "xhttp" {
		return streamSettings
	}
	upload, _ := stream["xhttpSettings"].(map[string]any)
	if extra, ok := upload["extra"].(map[string]any); ok {
		upload = extra
	}
	down, ok := upload["downloadSettings"].(map[string]any)
	if !ok {
		return streamSettings
	}
	leg, _ := down["xhttpSettings"].(map[string]any)
	if extra, ok := leg["extra"].(map[string]any); ok {
		leg = extra
	}
	if leg == nil {
		leg = map[string]any{}
		down["xhttpSettings"] = leg
	}
	changed := false
	for _, key := range xhttpListenerCheckedKeys {
		want, has := upload[key]
		got, had := leg[key]
		if has == had && reflect.DeepEqual(want, got) {
			continue
		}
		if has {
			leg[key] = want
		} else {
			delete(leg, key)
		}
		changed = true
	}
	if !changed {
		return streamSettings
	}
	out, err := json.MarshalIndent(stream, "", "  ")
	if err != nil {
		return streamSettings
	}
	return string(out)
}
