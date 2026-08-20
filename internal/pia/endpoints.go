// Copyright (c) 2026 Masterain. MIT License.
// Adapted from PIA-Wireguard-Config-Generator-GUI (commit 53686fcd).
package pia

import (
	_ "embed"
	"time"
)

const (
	DefaultTokenEndpoint      = "https://www.privateinternetaccess.com/api/client/v2/token"
	DefaultServerListEndpoint = "https://serverlist.piaservers.net/vpninfo/servers/v6"
	DefaultAddKeyPort         = uint16(1337)
	DefaultUserAgent          = "3x-ui-pia/1.0"
	DefaultMaxServerListBody  = int64(8 << 20)
	DefaultMaxResponseBody    = int64(64 << 10)
	DefaultRequestTimeout     = 20 * time.Second
	DefaultCatalogFreshTTL    = 6 * time.Hour
	DefaultTokenTTL           = 24 * time.Hour
)

//go:embed trust/ca.rsa.4096.crt
var EmbeddedPIACA []byte

//go:embed trust/serverlist_public_key.pem
var EmbeddedServerListPublicKey []byte
