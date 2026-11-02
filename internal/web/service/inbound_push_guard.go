package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// jsonLeaf is one scalar of a JSON document with the lowercased key path leading to it.
// Repeated keys each yield a leaf: xray-core binds keys case-folded, and the last wins.
type jsonLeaf struct {
	path  []string
	value string
}

func (l jsonLeaf) endsWith(suffix ...string) bool {
	return len(l.path) >= len(suffix) && slices.Equal(l.path[len(l.path)-len(suffix):], suffix)
}

// jsonLeaves walks raw token by token, so no repeated key hides behind another.
func jsonLeaves(raw string) []jsonLeaf {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var leaves []jsonLeaf
	var walk func(path []string) bool
	walk = func(path []string) bool {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		switch tok {
		case json.Delim('{'):
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return false
				}
				name, _ := key.(string)
				if !walk(append(slices.Clone(path), strings.ToLower(name))) {
					return false
				}
			}
			_, err = dec.Token()
			return err == nil
		case json.Delim('['):
			for dec.More() {
				if !walk(append(slices.Clone(path), "[]")) {
					return false
				}
			}
			_, err = dec.Token()
			return err == nil
		}
		leaves = append(leaves, jsonLeaf{path: path, value: fmt.Sprint(tok)})
		return true
	}
	walk(nil)
	return leaves
}

// isEnvRef matches what xray-core expands from the panel's environment: a whole value, or
// one entry of a comma list such as a port list.
func isEnvRef(value string) bool {
	for part := range strings.SplitSeq(value, ",") {
		if strings.HasPrefix(strings.TrimSpace(part), "env:") {
			return true
		}
	}
	return false
}

// validateInboundEnvRefs refuses "env:NAME" anywhere the core would expand it: the panel's
// environment (database DSN, token key) must not reach a config or a load error quoting it.
func validateInboundEnvRefs(inbound *model.Inbound) error {
	found := isEnvRef(inbound.Listen)
	for _, raw := range []string{inbound.Settings, inbound.StreamSettings, inbound.Sniffing} {
		for _, leaf := range jsonLeaves(raw) {
			found = found || isEnvRef(leaf.value)
		}
	}
	if found {
		return common.NewError(`an inbound must not refer to environment variables ("env:")`)
	}
	return nil
}

// pushReach is one field of a pushed inbound that names a resource on this host.
type pushReach struct {
	field, value string
}

// validatePushedInbound refuses, on a master's push, the fields that reach this host's own
// files, sockets and services; a value the stored row already carries was set here and stays.
func validatePushedInbound(inbound, stored *model.Inbound) error {
	if (stored == nil || stored.Protocol != inbound.Protocol) && !nodeEligibleProtocols[inbound.Protocol] {
		return common.NewErrorf("%s inbounds cannot be pushed to a node", inbound.Protocol)
	}
	reaches, err := hostReaches(inbound)
	if err != nil {
		return err
	}
	var allowed []pushReach
	if stored != nil {
		if allowed, err = hostReaches(stored); err != nil {
			return err
		}
	}
	for _, r := range reaches {
		if !slices.Contains(allowed, r) {
			return common.NewErrorf("a pushed inbound may not set %s to %q on this node", r.field, r.value)
		}
	}
	return nil
}

func hostReaches(inbound *model.Inbound) ([]pushReach, error) {
	local, err := localRelayTargets(inbound.Id)
	if err != nil {
		return nil, err
	}
	var reaches []pushReach
	add := func(field string, leaf jsonLeaf) {
		reaches = append(reaches, pushReach{field: field, value: leaf.value})
	}
	streamLeaves := jsonLeaves(inbound.StreamSettings)
	// A whole object is one reach: a push keeping the type but moving the url or folder
	// is a different reach than the one this node stores.
	object := func(field string, container ...string) {
		var parts []string
		for _, l := range streamLeaves {
			if i := slices.Index(l.path, container[len(container)-1]); i >= len(container)-1 && slices.Equal(l.path[i+1-len(container):i+1], container) {
				parts = append(parts, strings.Join(l.path[i+1:], ".")+"="+l.value)
			}
		}
		slices.Sort(parts)
		reaches = append(reaches, pushReach{field: field, value: strings.Join(parts, ";")})
	}
	for _, leaf := range streamLeaves {
		lower := strings.ToLower(strings.TrimSpace(leaf.value))
		switch {
		case leaf.endsWith("hysteriasettings", "masquerade", "type") && (lower == "file" || lower == "proxy"):
			object("hysteria masquerade", "hysteriasettings", "masquerade")
		case leaf.endsWith("masterkeylog") && lower != "" && lower != "none":
			add("masterKeyLog", leaf)
		case leaf.endsWith("xdrivesettings", "service") && (lower == "local" || lower == "template" || lower == "s3"):
			object("xdrive service", "xdrivesettings")
		case leaf.endsWith("echconfiglist") && strings.Contains(lower, "://"):
			add("echConfigList", leaf)
		case slices.Contains(leaf.path, "finalmask") && leaf.endsWith("type") && lower == "realm":
			add("finalmask", leaf)
		case leaf.endsWith("realitysettings", "target") || leaf.endsWith("realitysettings", "dest"):
			if !isPublicRelayTarget(leaf.value) {
				add("REALITY target", leaf)
			}
		}
	}
	for _, leaf := range jsonLeaves(inbound.Settings) {
		switch {
		case leaf.endsWith("fallbacks", "[]", "dest"):
			if !isPublicRelayTarget(leaf.value) && !slices.Contains(local, normalizeRelayTarget(leaf.value)) {
				add("fallback dest", leaf)
			}
		case leaf.endsWith("domainfronting", "ip"):
			if ip, err := netip.ParseAddr(strings.TrimSpace(leaf.value)); err == nil && isBlockedIP(net.IP(ip.AsSlice())) {
				add("domainFronting ip", leaf)
			}
		}
	}
	return reaches, nil
}

// isPublicRelayTarget reports whether a relay target dials a public host, not this one: a
// bare port, a unix socket and a loopback or private host all stay on the machine.
func isPublicRelayTarget(target string) bool {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "@") {
		return false
	}
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		return false
	}
	return !namesInternalHost(host)
}

// normalizeRelayTarget reduces a fallback dest to the form localRelayTargets lists.
func normalizeRelayTarget(target string) string {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "@") {
		return target
	}
	if _, err := strconv.Atoi(target); err == nil {
		return target
	}
	if _, port, err := net.SplitHostPort(target); err == nil {
		return port
	}
	return target
}

// localRelayTargets lists what this node's other inbounds listen on, the only local
// fallback dests a master generates; the core's own listeners are never among them.
func localRelayTargets(selfID int) ([]string, error) {
	var rows []model.Inbound
	if err := database.GetDB().Select("id", "listen", "port").Where("node_id IS NULL AND id <> ?", selfID).Find(&rows).Error; err != nil {
		return nil, err
	}
	reserved := map[string]bool{strconv.Itoa(reservedAPIPort()): true}
	for _, port := range xrayOwnPorts() {
		reserved[port] = true
	}
	var targets []string
	for _, ib := range rows {
		port := strconv.Itoa(ib.Port)
		if reserved[port] {
			continue
		}
		targets = append(targets, port)
		if listen := strings.TrimSpace(ib.Listen); strings.HasPrefix(listen, "/") || strings.HasPrefix(listen, "@") {
			targets = append(targets, strings.SplitN(listen, ",", 2)[0])
		}
	}
	return targets, nil
}
