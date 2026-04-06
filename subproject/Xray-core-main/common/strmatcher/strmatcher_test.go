package strmatcher_test

import (
	"reflect"
	"testing"

	"github.com/xtls/xray-core/common"
	. "github.com/xtls/xray-core/common/strmatcher"
)

func TestMatcherGroup(t *testing.T) {
	rules := []struct {
		Type   Type
		Domain string
	}{
		{
			Type:   Regex,
			Domain: "apis\\.us$",
		},
		{
			Type:   Substr,
			Domain: "apis",
		},
		{
			Type:   Domain,
			Domain: "googleapis.com",
		},
		{
			Type:   Domain,
			Domain: "com",
		},
		{
			Type:   Full,
			Domain: "www.baidu.com",
		},
		{
			Type:   Substr,
			Domain: "apis",
		},
		{
			Type:   Domain,
			Domain: "googleapis.com",
		},
		{
			Type:   Full,
			Domain: "fonts.googleapis.com",
		},
		{
			Type:   Full,
			Domain: "www.baidu.com",
		},
		{
			Type:   Domain,
			Domain: "example.com",
		},
	}
	cases := []struct {
		Input  string
		Output []uint32
	}{
		{
			Input:  "www.baidu.com",
			Output: []uint32{5, 9, 4},
		},
		{
			Input:  "fonts.googleapis.com",
			Output: []uint32{8, 3, 7, 4, 2, 6},
		},
		{
			Input:  "example.googleapis.com",
			Output: []uint32{3, 7, 4, 2, 6},
		},
		{
			Input:  "testapis.us",
			Output: []uint32{1, 2, 6},
		},
		{
			Input:  "example.com",
			Output: []uint32{10, 4},
		},
	}
	matcherGroup := &MatcherGroup{}
	for _, rule := range rules {
		matcher, err := rule.Type.New(rule.Domain)
		common.Must(err)
		matcherGroup.Add(matcher)
	}
	for _, test := range cases {
		if m := matcherGroup.Match(test.Input); !reflect.DeepEqual(m, test.Output) {
			t.Error("unexpected output: ", m, " for test case ", test)
		}
	}
}

func TestACAutomaton(t *testing.T) {
	cases1 := []struct {
		pattern string
		mType   Type
		input   string
		output  bool
	}{
		{
			pattern: "xtls.github.io",
			mType:   Domain,
			input:   "www.xtls.github.io",
			output:  true,
		},
		{
			pattern: "xtls.github.io",
			mType:   Domain,
			input:   "xtls.github.io",
			output:  true,
		},
		{
			pattern: "xtls.github.io",
			mType:   Domain,
			input:   "www.xtis.github.io",
			output:  false,
		},
		{
			pattern: "xtls.github.io",
			mType:   Domain,
			input:   "tls.github.io",
			output:  false,
		},
		{
			pattern: "xtls.github.io",
			mType:   Domain,
			input:   "xxtls.github.io",
			output:  false,
		},
		{
			pattern: "xtls.github.io",
			mType:   Full,
			input:   "xtls.github.io",
			output:  true,
		},
		{
			pattern: "xtls.github.io",
			mType:   Full,
			input:   "xxtls.github.io",
			output:  false,
		},
	}
	for _, test := range cases1 {
		ac := NewACAutomaton()
		ac.Add(test.pattern, test.mType)
		ac.Build()
		if m := ac.Match(test.input); m != test.output {
			t.Error("unexpected output: ", m, " for test case ", test)
		}
	}
	{
		cases2Input := []struct {
			pattern string
			mType   Type
		}{
			{
				pattern: "163.com",
				mType:   Domain,
			},
			{
				pattern: "m.126.com",
				mType:   Full,
			},
			{
				pattern: "3.com",
				mType:   Full,
			},
			{
				pattern: "google.com",
				mType:   Substr,
			},
			{
				pattern: "vgoogle.com",
				mType:   Substr,
			},
		}
		ac := NewACAutomaton()
		for _, test := range cases2Input {
			ac.Add(test.pattern, test.mType)
		}
		ac.Build()
		cases2Output := []struct {
			pattern string
			res     bool
		}{
			{
				pattern: "126.com",
				res:     false,
			},
			{
				pattern: "m.163.com",
				res:     true,
			},
			{
				pattern: "mm163.com",
				res:     false,
			},
			{
				pattern: "m.126.com",
				res:     true,
			},
			{
				pattern: "163.com",
				res:     true,
			},
			{
				pattern: "63.com",
				res:     false,
			},
			{
				pattern: "oogle.com",
				res:     false,
			},
			{
				pattern: "vvgoogle.com",
				res:     true,
			},
			{
				pattern: "½",
				res:     false,
			},
		}
		for _, test := range cases2Output {
			if m := ac.Match(test.pattern); m != test.res {
				t.Error("unexpected output: ", m, " for test case ", test)
			}
		}
	}
	{
		cases3Input := []struct {
			pattern string
			mType   Type
		}{
			{
				pattern: "video.google.com",
				mType:   Domain,
			},
			{
				pattern: "gle.com",
				mType:   Domain,
			},
		}
		ac := NewACAutomaton()
		for _, test := range cases3Input {
			ac.Add(test.pattern, test.mType)
		}
		ac.Build()
		cases3Output := []struct {
			pattern string
			res     bool
		}{
			{
				pattern: "google.com",
				res:     false,
			},
		}
		for _, test := range cases3Output {
			if m := ac.Match(test.pattern); m != test.res {
				t.Error("unexpected output: ", m, " for test case ", test)
			}
		}
	}
}
