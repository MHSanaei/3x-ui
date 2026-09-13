// Package dnsconf validates the JSON-subscription DNS setting against xray's own
// schema, so a block the client could not load never reaches an emitted document.
package dnsconf

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/xtls/xray-core/infra/conf"
)

// Parse resolves the setting into the dns subtree to emit: a full dns object, or
// a bare array of servers wrapped into one. A blank value means "no override".
func Parse(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, fmt.Errorf("invalid DNS JSON: %w", err)
	}

	block := make(map[string]any)
	servers, isList := decoded.([]any)
	if isList {
		block["servers"] = servers
	} else {
		object, isObject := decoded.(map[string]any)
		if !isObject {
			return nil, errors.New("DNS config must be a JSON object or an array of servers")
		}
		block = object
	}

	if err := validate(block); err != nil {
		return nil, err
	}
	return block, nil
}

// validate decodes the block into xray's own schema. Build() is deliberately not
// run: it resolves geosite tokens from geodata files the panel may not have.
func validate(block map[string]any) (err error) {
	// Third-party parser fed by a panel setting: a panic must degrade to
	// "unusable value", never take the panel or sub server down.
	defer func() {
		if panicValue := recover(); panicValue != nil {
			err = fmt.Errorf("invalid DNS config: %v", panicValue)
		}
	}()

	payload, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("invalid DNS config: %w", err)
	}
	var parsed conf.DNSConfig
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return fmt.Errorf("invalid DNS config: %w", err)
	}

	servers, _ := block["servers"].([]any)
	if len(servers) == 0 {
		// xray quietly installs the system resolver when no server is
		// configured, which would leak lookups outside the tunnel.
		return errors.New(`"servers" must list at least one DNS server`)
	}
	for index, entry := range servers {
		if err := validateServer(index, entry); err != nil {
			return err
		}
	}
	if err := validateClientIP("dns", parsed.ClientIP); err != nil {
		return err
	}
	for index, server := range parsed.Servers {
		if err := validateClientIP(fmt.Sprintf("DNS server #%d", index+1), server.ClientIP); err != nil {
			return err
		}
	}
	return nil
}

// validateServer enforces the address rule Build() would: xray refuses a name
// server without one, but only reports it when the client starts.
func validateServer(index int, entry any) error {
	label := fmt.Sprintf("DNS server #%d", index+1)
	switch server := entry.(type) {
	case string:
		if strings.TrimSpace(server) == "" {
			return fmt.Errorf("%s is empty", label)
		}
	case map[string]any:
		address, ok := server["address"]
		if !ok {
			return fmt.Errorf(`%s needs a non-empty "address"`, label)
		}
		text, ok := address.(string)
		if ok && strings.TrimSpace(text) == "" {
			return fmt.Errorf(`%s needs a non-empty "address"`, label)
		}
		if _, isObject := address.(map[string]any); !ok && !isObject {
			return fmt.Errorf(`%s needs a non-empty "address"`, label)
		}
	default:
		return fmt.Errorf("%s must be a string or an object", label)
	}
	return nil
}

func validateClientIP(label string, clientIP *conf.Address) error {
	if clientIP != nil && !clientIP.Family().IsIP() {
		return fmt.Errorf("%s clientIp must be an IP address", label)
	}
	return nil
}
