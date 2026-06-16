// Package wirecodec holds the shared envelope codec for node-to-node config
// transport: zstd (de)compression, SHA-256 integrity hashing, and the header /
// capability constants both the panel (sender) and node (receiver) agree on.
package wirecodec

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/klauspost/compress/zstd"
)

const (
	// HashHeader carries the lowercase-hex SHA-256 of the (uncompressed) body.
	HashHeader = "X-Config-Sha256"
	// CapsHeader is set by a node on its API responses to advertise support.
	CapsHeader = "X-3x-Node-Caps"
	// EncodingZstd is the Content-Encoding value for a zstd-compressed body.
	EncodingZstd = "zstd"
	// CapZstd is the capability token advertised in CapsHeader.
	CapZstd = "zstd"

	// maxDecodeBytes bounds in-memory decompression to defuse a zstd bomb from
	// an (authenticated) node-API caller.
	maxDecodeBytes = 16 << 20
)

// EncodeAll/DecodeAll on these shared instances are safe for concurrent use.
var (
	zstdEncoder, _ = zstd.NewWriter(nil)
	zstdDecoder, _ = zstd.NewReader(nil, zstd.WithDecoderMaxMemory(maxDecodeBytes))
)

// Compress zstd-compresses b.
func Compress(b []byte) []byte {
	return zstdEncoder.EncodeAll(b, nil)
}

// Decompress zstd-decompresses src, rejecting output larger than maxOut (and any
// input that would blow the in-memory bomb ceiling).
func Decompress(src []byte, maxOut int) ([]byte, error) {
	out, err := zstdDecoder.DecodeAll(src, nil)
	if err != nil {
		return nil, err
	}
	if maxOut > 0 && len(out) > maxOut {
		return nil, errors.New("wirecodec: decompressed body exceeds limit")
	}
	return out, nil
}

// Sha256Hex returns the lowercase-hex SHA-256 of b.
func Sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
