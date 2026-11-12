// Package wirecodec holds the shared envelope codec for node-to-node config
// transport: zstd (de)compression, SHA-256 integrity hashing, and the header /
// capability constants both the panel (sender) and node (receiver) agree on.
package wirecodec

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"runtime"

	"github.com/klauspost/compress/zstd"
)

const (
	// HashHeader carries the lowercase-hex SHA-256 of the (uncompressed) body.
	HashHeader = "X-Config-Sha256"
	// CapsHeader is set by a node on its API responses to advertise support.
	CapsHeader = "X-3x-Node-Caps"
	// MasterPushHeader marks a request as a master's push, whatever its token scope.
	MasterPushHeader = "X-3x-Master-Push"
	// EncodingZstd is the Content-Encoding value for a zstd-compressed body.
	EncodingZstd = "zstd"
	// CapZstd is the capability token advertised in CapsHeader.
	CapZstd = "zstd"
	// CapAmneziaWGKernel marks a node that honours an AmneziaWG inbound's
	// kernelMode; an older node drops the flag and serves it embedded.
	CapAmneziaWGKernel = "awg-kernel"
	// NodeCaps is the full CapsHeader value this build advertises.
	NodeCaps = CapZstd + "," + CapAmneziaWGKernel

	// maxDecodeBytes bounds in-memory decompression to defuse a zstd bomb from
	// an (authenticated) node-API caller.
	maxDecodeBytes = 16 << 20
)

// EncodeAll/DecodeAll on these shared instances are safe for concurrent use.
var (
	decoderWorkers = runtime.GOMAXPROCS(0)
	zstdEncoder, _ = zstd.NewWriter(nil)
	zstdDecoder, _ = zstd.NewReader(nil, zstd.WithDecoderMaxMemory(maxDecodeBytes),
		zstd.WithDecodeAllCapLimit(true), zstd.WithDecoderConcurrency(decoderWorkers))
	// decodeSlots keeps callers waiting for a decoder worker from each holding an output buffer.
	decodeSlots        = make(chan struct{}, decoderWorkers)
	errDecodedTooLarge = errors.New("wirecodec: decompressed body exceeds limit")
)

// Compress zstd-compresses b.
func Compress(b []byte) []byte {
	return zstdEncoder.EncodeAll(b, nil)
}

// Decompress zstd-decompresses src, rejecting output larger than maxOut (and any
// input that would blow the in-memory bomb ceiling).
func Decompress(src []byte, maxOut int) ([]byte, error) {
	if maxOut <= 0 || maxOut > maxDecodeBytes {
		maxOut = maxDecodeBytes
	}
	var header zstd.Header
	if err := header.Decode(src); err != nil {
		return nil, err
	}
	// The output buffer is sized once, from the first frame (Compress writes a single
	// frame): letting it grow per frame copied all output so far for every extra frame.
	size := maxOut
	if header.HasFCS {
		if header.FrameContentSize > uint64(maxOut) {
			return nil, errDecodedTooLarge
		}
		size = int(header.FrameContentSize)
	}
	decodeSlots <- struct{}{}
	defer func() { <-decodeSlots }()
	out, err := zstdDecoder.DecodeAll(src, make([]byte, 0, size))
	if errors.Is(err, zstd.ErrDecoderSizeExceeded) {
		return nil, errDecodedTooLarge
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Sha256Hex returns the lowercase-hex SHA-256 of b.
func Sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
