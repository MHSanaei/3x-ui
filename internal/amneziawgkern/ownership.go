package amneziawgkern

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func ownershipMarker(name string) string {
	return "# 3x-ui-owned-amneziawg: " + name
}

func parseOwnedInterfaceName(name string) (int, bool) {
	if !strings.HasPrefix(name, interfacePrefix) {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(name, interfacePrefix))
	if err != nil || id <= 0 || ownedInterfaceName(id) != name {
		return 0, false
	}
	return id, true
}

// ownedConfigIDs lists the inbounds whose config the panel left on disk, so a
// fresh manager can find interfaces that outlived a crash.
func ownedConfigIDs(dir string) ([]int, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []int
	for _, entry := range entries {
		id, ok := parseOwnedInterfaceName(strings.TrimSuffix(entry.Name(), ".conf"))
		if !ok || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}
		if _, found, err := readOwnedConfig(dir, id); found && err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// readOwnedConfig returns the config the panel wrote for id; found with an
// error means a file is there that the panel did not write.
func readOwnedConfig(dir string, id int) (string, bool, error) {
	name := ownedInterfaceName(id)
	path := ConfPathIn(dir, name)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !info.Mode().IsRegular() {
		return "", true, fmt.Errorf("amneziawgkern: refusing non-regular config %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", true, err
	}
	if !validOwnedConfig(string(data), name) {
		return "", true, fmt.Errorf("amneziawgkern: refusing untrusted config %s", path)
	}
	return string(data), true, nil
}

func configInterfacePublicKey(config string) (string, error) {
	var privateKey string
	for _, line := range strings.Split(config, "\n") {
		if !strings.HasPrefix(line, "PrivateKey = ") {
			continue
		}
		if privateKey != "" {
			return "", fmt.Errorf("owned config has multiple private keys")
		}
		privateKey = strings.TrimPrefix(line, "PrivateKey = ")
	}
	publicKey, err := wgutil.PublicKeyFromPrivate(privateKey)
	if err != nil {
		return "", fmt.Errorf("owned config has an invalid private key: %w", err)
	}
	return publicKey, nil
}

// interfaceKeys and peerKeys are every key Render emits; anything else means
// the file did not come from Render.
var (
	interfaceKeys = map[string]bool{
		"PrivateKey": true, "ListenPort": true, "Jc": true, "Jmin": true, "Jmax": true,
		"S1": true, "S2": true, "S3": true, "S4": true, "H1": true, "H2": true, "H3": true, "H4": true,
		"I1": true, "I2": true, "I3": true, "I4": true, "I5": true, "HeaderProtectionKey": true,
		"ContentPaddingAddition": true, "RekeyAfterTime": true, "RekeyTimeout": true, "RejectAfterTime": true,
		"KeepaliveTimeout": true, "MaxHandshakeAttempts": true, "RandomTrailers": true, "DisableCookies": true,
	}
	peerKeys = map[string]bool{"PublicKey": true, "PresharedKey": true, "AllowedIPs": true}
)

func validOwnedConfig(config, name string) bool {
	if !strings.HasPrefix(config, ownershipMarker(name)+"\n") || strings.ContainsAny(config, "\r\x00") {
		return false
	}
	section := ""
	interfaceCount := 0
	for _, line := range strings.Split(config, "\n")[1:] {
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#"):
			if section != "Peer" {
				return false
			}
			continue
		case line == "[Interface]":
			interfaceCount++
			if interfaceCount != 1 || section != "" {
				return false
			}
			section = "Interface"
			continue
		case line == "[Peer]":
			if section == "" {
				return false
			}
			section = "Peer"
			continue
		}
		key, value, ok := strings.Cut(line, " = ")
		if !ok || value == "" {
			return false
		}
		if (section == "Interface" && !interfaceKeys[key]) || (section == "Peer" && !peerKeys[key]) || section == "" {
			return false
		}
	}
	return interfaceCount == 1
}
