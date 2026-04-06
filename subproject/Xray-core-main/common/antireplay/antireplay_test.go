package antireplay

import (
	"bufio"
	"crypto/rand"
	"testing"
)

func BenchmarkMapFilter(b *testing.B) {
	filter := NewMapFilter[[16]byte](120)
	var sample [16]byte
	reader := bufio.NewReader(rand.Reader)
	reader.Read(sample[:])
	b.ResetTimer()
	for range b.N {
		reader.Read(sample[:])
		filter.Check(sample)
	}
}

func TestMapFilter(t *testing.T) {
	filter := NewMapFilter[[16]byte](120)
	var sample [16]byte
	rand.Read(sample[:])
	filter.Check(sample)
	if filter.Check(sample) {
		t.Error("Unexpected true negative")
	}
	sample[0]++
	if !filter.Check(sample) {
		t.Error("Unexpected false positive")
	}
}
