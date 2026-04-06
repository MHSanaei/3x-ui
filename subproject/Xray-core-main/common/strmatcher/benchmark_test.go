package strmatcher_test

import (
	"strconv"
	"testing"

	"github.com/xtls/xray-core/common"
	. "github.com/xtls/xray-core/common/strmatcher"
)

func BenchmarkACAutomaton(b *testing.B) {
	ac := NewACAutomaton()
	for i := 1; i <= 1024; i++ {
		ac.Add(strconv.Itoa(i)+".xray.com", Domain)
	}
	ac.Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ac.Match("0.xray.com")
	}
}

func BenchmarkDomainMatcherGroup(b *testing.B) {
	g := new(DomainMatcherGroup)

	for i := 1; i <= 1024; i++ {
		g.Add(strconv.Itoa(i)+".example.com", uint32(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Match("0.example.com")
	}
}

func BenchmarkFullMatcherGroup(b *testing.B) {
	g := new(FullMatcherGroup)

	for i := 1; i <= 1024; i++ {
		g.Add(strconv.Itoa(i)+".example.com", uint32(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Match("0.example.com")
	}
}

func BenchmarkMarchGroup(b *testing.B) {
	g := new(MatcherGroup)
	for i := 1; i <= 1024; i++ {
		m, err := Domain.New(strconv.Itoa(i) + ".example.com")
		common.Must(err)
		g.Add(m)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Match("0.example.com")
	}
}
