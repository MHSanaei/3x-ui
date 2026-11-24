package xray

import (
	"bytes"
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func splitWireguardPeers(raw json_util.RawMessage) (map[string]clientEntry, []byte, bool) {
	var settings map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&settings); err != nil {
		return nil, nil, false
	}
	peers, ok := settings["peers"].([]any)
	if !ok {
		return nil, nil, false
	}
	byKey := make(map[string]clientEntry, len(peers))
	for _, peer := range peers {
		user, ok := peer.(map[string]any)
		if !ok {
			return nil, nil, false
		}
		email, _ := user["email"].(string)
		publicKey, _ := user["publicKey"].(string)
		key, err := wgutil.KeyToHex(publicKey)
		if err != nil || key == "" || email == "" {
			return nil, nil, false
		}
		if _, duplicate := byKey[key]; duplicate {
			return nil, nil, false
		}

		// The inbound user API can apply panel-managed peers with default level.
		level, endpoint := user["level"], user["endpoint"]
		if (level != nil && level != json.Number("0")) || (endpoint != nil && endpoint != "") ||
			len(collectStringSlice(user["allowedIPs"])) == 0 {
			return nil, nil, false
		}
		norm, err := json.Marshal(user)
		if err != nil {
			return nil, nil, false
		}
		if keepAlive, ok := user["keepAlive"].(json.Number); ok {
			user["keepAlive"] = keepAlive.String()
		}
		byKey[key] = clientEntry{user: user, norm: norm}
	}
	delete(settings, "peers")
	rest, err := json.Marshal(settings)
	return byKey, rest, err == nil
}

func diffWireguardPeers(oldIb, newIb *InboundConfig, diff *HotDiff) bool {
	oldPeers, oldRest, oldOK := splitWireguardPeers(oldIb.Settings)
	newPeers, newRest, newOK := splitWireguardPeers(newIb.Settings)
	if !oldOK || !newOK || !bytes.Equal(oldRest, newRest) {
		return false
	}
	replaced := make(map[string]bool)
	for key, peer := range oldPeers {
		email := peer.user["email"].(string)
		if !replaced[email] && !bytes.Equal(peer.norm, newPeers[key].norm) {
			diff.RemovedUsers = append(diff.RemovedUsers, UserOp{Tag: oldIb.Tag, Protocol: oldIb.Protocol, Email: email})
			replaced[email] = true
		}
	}
	// Xray removes by email, so restore the surviving peers of this client.
	for key, peer := range newPeers {
		email := peer.user["email"].(string)
		if replaced[email] || !bytes.Equal(peer.norm, oldPeers[key].norm) {
			diff.AddedUsers = append(diff.AddedUsers, UserOp{Tag: newIb.Tag, Protocol: newIb.Protocol, Email: email, User: peer.user})
		}
	}
	return true
}
