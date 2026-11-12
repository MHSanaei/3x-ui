package wirecodec

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestCompressRoundTrip(t *testing.T) {
	orig := []byte(strings.Repeat("inbound config payload ", 200))
	packed := Compress(orig)
	if len(packed) == 0 {
		t.Fatal("Compress returned empty output")
	}
	got, err := Decompress(packed, 1<<20)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(got, orig) {
		t.Fatal("round trip mismatch")
	}
}

func TestDecompressRejectsOversize(t *testing.T) {
	orig := bytes.Repeat([]byte("A"), 1<<16) // 64 KiB, highly compressible
	packed := Compress(orig)
	if _, err := Decompress(packed, 1024); err == nil {
		t.Fatal("Decompress must reject output that exceeds the cap (bomb guard)")
	}
}

func TestDecompressRejectsGarbage(t *testing.T) {
	if _, err := Decompress([]byte("not a zstd frame"), 1<<20); err == nil {
		t.Fatal("Decompress must reject non-zstd input")
	}
}

func TestSha256HexStableAndSensitive(t *testing.T) {
	a := Sha256Hex([]byte("config-A"))
	b := Sha256Hex([]byte("config-A"))
	c := Sha256Hex([]byte("config-B"))
	if a != b {
		t.Fatal("hash must be stable for identical input")
	}
	if a == c {
		t.Fatal("hash must differ when the body changes")
	}
	if len(a) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(a))
	}
}

// Every extra frame made the decoder copy all output so far into a larger buffer, so a
// few KiB of tiny frames cost gigabytes of allocation for one authenticated request.
func TestDecompressOfManyFramesAllocatesTheOutputOnce(t *testing.T) {
	frame := Compress(bytes.Repeat([]byte("A"), 4096))
	src := bytes.Repeat(frame, 2000)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, _ = Decompress(src, 8<<20)
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 64<<20 {
		t.Fatalf("decoding %d bytes of frames allocated %d MiB", len(src), allocated>>20)
	}
}
