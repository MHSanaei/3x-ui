package salamander_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xtls/xray-core/transport/internet/finalmask/salamander"
)

const (
	smSaltLen = 8
)

func BenchmarkSalamanderObfuscator_Obfuscate(b *testing.B) {
	o, _ := salamander.NewSalamanderObfuscator([]byte("average_password"))
	in := make([]byte, 1200)
	_, _ = rand.Read(in)
	out := make([]byte, 2048)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.Obfuscate(in, out)
	}
}

func BenchmarkSalamanderObfuscator_Deobfuscate(b *testing.B) {
	o, _ := salamander.NewSalamanderObfuscator([]byte("average_password"))
	in := make([]byte, 1200)
	_, _ = rand.Read(in)
	out := make([]byte, 2048)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.Deobfuscate(in, out)
	}
}

func TestSalamanderObfuscator(t *testing.T) {
	o, _ := salamander.NewSalamanderObfuscator([]byte("average_password"))
	in := make([]byte, 1200)
	oOut := make([]byte, 2048)
	dOut := make([]byte, 2048)
	for i := 0; i < 1000; i++ {
		_, _ = rand.Read(in)
		n := o.Obfuscate(in, oOut)
		assert.Equal(t, len(in)+smSaltLen, n)
		n = o.Deobfuscate(oOut[:n], dOut)
		assert.Equal(t, len(in), n)
		assert.Equal(t, in, dOut[:n])
	}
}

func TestSalamanderInPlace(t *testing.T) {
	o, _ := salamander.NewSalamanderObfuscator([]byte("average_password"))

	in := make([]byte, 1200)
	out := make([]byte, 2048)
	_, _ = rand.Read(in)
	o.Obfuscate(in, out)

	out2 := make([]byte, 2048)
	copy(out2[smSaltLen:], in)
	o.Obfuscate(out2[smSaltLen:], out2)

	dOut := make([]byte, 2048)
	o.Deobfuscate(out, dOut)

	o.Deobfuscate(out2, out2)

	assert.Equal(t, in, dOut[:1200])
	assert.Equal(t, in, out2[:1200])
}

func TestSalamanderBounce(t *testing.T) {
	o, _ := salamander.NewSalamanderObfuscator([]byte("average_password"))
	buf := make([]byte, 8)
	for i := 0; i < 1000; i++ {
		_, _ = rand.Read(buf)
		n := o.Deobfuscate(buf, buf)
		assert.Equal(t, 0, n)
	}
}
