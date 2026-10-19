package amneziawgkern

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ownershipMarker(name string) string {
	return "# 3x-ui-owned-amneziawg: " + name
}

func ownedInterfaceName(id int) string {
	return fmt.Sprintf("awg%d", id)
}

func parseOwnedInterfaceName(name string) (int, bool) {
	if !strings.HasPrefix(name, "awg") {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimPrefix(name, "awg"))
	if err != nil || id <= 0 || ownedInterfaceName(id) != name {
		return 0, false
	}
	return id, true
}

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
		name := entry.Name()
		if !strings.HasSuffix(name, ".conf") {
			continue
		}
		id, ok := parseOwnedInterfaceName(strings.TrimSuffix(name, ".conf"))
		if !ok {
			continue
		}
		info, err := os.Lstat(filepath.Join(dir, name))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if _, found, err := readOwnedConfig(dir, id); found && err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

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
		return "", false, fmt.Errorf("amneziawgkern: refusing non-regular config %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	if !validOwnedConfig(string(data), name) {
		return "", true, fmt.Errorf("amneziawgkern: refusing untrusted config %s", path)
	}
	return string(data), true, nil
}

func validOwnedConfig(config, name string) bool {
	id, ok := parseOwnedInterfaceName(name)
	if !ok || !strings.HasPrefix(config, ownershipMarker(name)+"\n") || strings.ContainsAny(config, "\r\x00") {
		return false
	}
	scanner := bufio.NewScanner(strings.NewReader(config))
	lineNo := 0
	section := ""
	interfaceCount := 0
	postDownCount := 0
	seenPostUp := map[string]bool{}
	for scanner.Scan() {
		line := scanner.Text()
		lineNo++
		if lineNo == 1 {
			continue
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if section != "Peer" {
				return false
			}
			continue
		}
		if strings.HasPrefix(line, "[") {
			switch line {
			case "[Interface]":
				interfaceCount++
				if interfaceCount != 1 || section != "" {
					return false
				}
				section = "Interface"
			case "[Peer]":
				if section == "" || interfaceCount != 1 {
					return false
				}
				section = "Peer"
			default:
				return false
			}
			continue
		}
		key, value, ok := strings.Cut(line, " = ")
		if !ok || strings.ContainsAny(value, "\r\n\x00") {
			return false
		}
		switch section {
		case "Interface":
			switch key {
			case "PrivateKey", "Address", "ListenPort", "MTU", "Jc", "Jmin", "Jmax", "S1", "S2", "S3", "S4",
				"H1", "H2", "H3", "H4", "I1", "I2", "I3", "I4", "I5", "HeaderProtectionKey",
				"ContentPaddingAddition", "RekeyAfterTime", "RekeyTimeout", "RejectAfterTime", "KeepaliveTimeout",
				"MaxHandshakeAttempts", "RandomTrailers", "DisableCookies":
				if value == "" {
					return false
				}
			case "PostUp":
				if !validOwnedPostUp(value, id) || seenPostUp[value] {
					return false
				}
				seenPostUp[value] = true
			case "PostDown":
				if value != fmt.Sprintf("nft delete table ip awgnat%d", id) {
					return false
				}
				postDownCount++
			default:
				return false
			}
		case "Peer":
			switch key {
			case "PublicKey", "PresharedKey", "AllowedIPs":
				if value == "" {
					return false
				}
			default:
				return false
			}
		default:
			return false
		}
	}
	return scanner.Err() == nil && interfaceCount == 1 && postDownCount <= 1
}

func validOwnedPostUp(value string, id int) bool {
	if value == "sysctl -q -w net.ipv4.ip_forward=1" ||
		value == fmt.Sprintf("nft add table ip awgnat%d", id) ||
		value == fmt.Sprintf("nft add chain ip awgnat%d post '{ type nat hook postrouting priority 100 ; }'", id) {
		return true
	}
	prefix := fmt.Sprintf("nft add rule ip awgnat%d post ip saddr ", id)
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	rest := strings.TrimPrefix(value, prefix)
	cidr, rest, ok := strings.Cut(rest, " oifname \"")
	if !ok {
		return false
	}
	p, err := netip.ParsePrefix(cidr)
	if err != nil || !p.Addr().Is4() {
		return false
	}
	if !strings.HasSuffix(rest, "\" masquerade") {
		return false
	}
	iface := strings.TrimSuffix(rest, "\" masquerade")
	if len(iface) == 0 || strings.ContainsAny(iface, "\\\"' $;|&<>`") {
		return false
	}
	return true
}
