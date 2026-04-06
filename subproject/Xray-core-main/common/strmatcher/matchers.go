package strmatcher

import (
	"regexp"
	"strings"
)

type fullMatcher string

func (m fullMatcher) Match(s string) bool {
	return string(m) == s
}

func (m fullMatcher) String() string {
	return "full:" + string(m)
}

type substrMatcher string

func (m substrMatcher) Match(s string) bool {
	return strings.Contains(s, string(m))
}

func (m substrMatcher) String() string {
	return "keyword:" + string(m)
}

type domainMatcher string

func (m domainMatcher) Match(s string) bool {
	pattern := string(m)
	if !strings.HasSuffix(s, pattern) {
		return false
	}
	return len(s) == len(pattern) || s[len(s)-len(pattern)-1] == '.'
}

func (m domainMatcher) String() string {
	return "domain:" + string(m)
}

type RegexMatcher struct {
	Pattern string
	reg     *regexp.Regexp
}

func (m *RegexMatcher) Match(s string) bool {
	if m.reg == nil {
		m.reg = regexp.MustCompile(m.Pattern)
	}
	return m.reg.MatchString(s)
}

func (m *RegexMatcher) String() string {
	return "regexp:" + m.Pattern
}
