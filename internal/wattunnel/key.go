package wattunnel

import (
	"encoding/base64"
	"encoding/hex"
)

func hexKeyBase64(key string) string {
	raw, _ := hex.DecodeString(key)
	return base64.StdEncoding.EncodeToString(raw)
}
