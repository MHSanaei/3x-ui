package strmatcher

import (
	"errors"
	"regexp"
)

// Matcher is the interface to determine a string matches a pattern.
type Matcher interface {
	// Match returns true if the given string matches a predefined pattern.
	Match(string) bool
	String() string
}

// Type is the type of the matcher.
type Type byte

const (
	// Full is the type of matcher that the input string must exactly equal to the pattern.
	Full Type = iota
	// Substr is the type of matcher that the input string must contain the pattern as a sub-string.
	Substr
	// Domain is the type of matcher that the input string must be a sub-domain or itself of the pattern.
	Domain
	// Regex is the type of matcher that the input string must matches the regular-expression pattern.
	Regex
)

// New creates a new Matcher based on the given pattern.
func (t Type) New(pattern string) (Matcher, error) {
	// 1. regex matching is case-sensitive
	switch t {
	case Full:
		return fullMatcher(pattern), nil
	case Substr:
		return substrMatcher(pattern), nil
	case Domain:
		return domainMatcher(pattern), nil
	case Regex:
		r, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		return &RegexMatcher{
			Pattern: pattern,
			reg:     r,
		}, nil
	default:
		return nil, errors.New("unk type")
	}
}

// IndexMatcher is the interface for matching with a group of matchers.
type IndexMatcher interface {
	// Match returns the index of a matcher that matches the input. It returns empty array if no such matcher exists.
	Match(input string) []uint32
	// Size returns the number of matchers in the group.
	Size() uint32
}

type MatcherEntry struct {
	M  Matcher
	Id uint32
}

// MatcherGroup is an implementation of IndexMatcher.
// Empty initialization works.
type MatcherGroup struct {
	count         uint32
	fullMatcher   FullMatcherGroup
	domainMatcher DomainMatcherGroup
	otherMatchers []MatcherEntry
}

// Add adds a new Matcher into the MatcherGroup, and returns its index. The index will never be 0.
func (g *MatcherGroup) Add(m Matcher) uint32 {
	g.count++
	c := g.count

	switch tm := m.(type) {
	case fullMatcher:
		g.fullMatcher.addMatcher(tm, c)
	case domainMatcher:
		g.domainMatcher.addMatcher(tm, c)
	default:
		g.otherMatchers = append(g.otherMatchers, MatcherEntry{
			M:  m,
			Id: c,
		})
	}

	return c
}

// Match implements IndexMatcher.Match.
func (g *MatcherGroup) Match(pattern string) []uint32 {
	result := []uint32{}
	result = append(result, g.fullMatcher.Match(pattern)...)
	result = append(result, g.domainMatcher.Match(pattern)...)
	for _, e := range g.otherMatchers {
		if e.M.Match(pattern) {
			result = append(result, e.Id)
		}
	}
	return result
}

// Size returns the number of matchers in the MatcherGroup.
func (g *MatcherGroup) Size() uint32 {
	return g.count
}

type IndexMatcherGroup struct {
	Matchers []IndexMatcher
}

func (g *IndexMatcherGroup) Match(input string) []uint32 {
	var offset uint32
	for _, m := range g.Matchers {
		if res := m.Match(input); len(res) > 0 {
			if offset == 0 {
				return res
			}
			shifted := make([]uint32, len(res))
			for i, id := range res {
				shifted[i] = id + offset
			}
			return shifted
		}
		offset += m.Size()
	}
	return nil
}

func (g *IndexMatcherGroup) Size() uint32 {
	var count uint32
	for _, m := range g.Matchers {
		count += m.Size()
	}
	return count
}
