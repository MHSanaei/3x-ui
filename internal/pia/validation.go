// Copyright (c) 2026 Masterain. MIT License.
// Adapted from PIA-Wireguard-Config-Generator-GUI (commit 53686fcd).
package pia

import (
	"encoding/base64"
	"net"
	"strings"
	"unicode"
)

func validSecret(value []byte, min, max int) bool {
	if len(value) < min || len(value) > max {
		return false
	}
	for _, b := range value {
		if b == 0 || b == '\r' || b == '\n' {
			return false
		}
	}
	return true
}

func validHostname(host string) bool {
	if host == "" || len(host) > 253 || net.ParseIP(host) != nil || strings.HasSuffix(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if r > unicode.MaxASCII || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-') {
				return false
			}
		}
	}
	return true
}

func validWGKey(key string) bool {
	decoded, err := base64.StdEncoding.DecodeString(key)
	return err == nil && len(decoded) == 32
}
