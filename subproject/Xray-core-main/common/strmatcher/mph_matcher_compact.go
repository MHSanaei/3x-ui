package strmatcher

import (
	"bytes"
	"encoding/gob"
	"io"
)

func init() {
	gob.Register(&RegexMatcher{})
	gob.Register(fullMatcher(""))
	gob.Register(substrMatcher(""))
	gob.Register(domainMatcher(""))
}

func (g *MphMatcherGroup) Serialize(w io.Writer) error {
	data := MphMatcherGroup{
		Ac:            g.Ac,
		OtherMatchers: g.OtherMatchers,
		Rules:         g.Rules,
		Level0:        g.Level0,
		Level0Mask:    g.Level0Mask,
		Level1:        g.Level1,
		Level1Mask:    g.Level1Mask,
		Count:         g.Count,
	}
	return gob.NewEncoder(w).Encode(data)
}

func NewMphMatcherGroupFromBuffer(data []byte) (*MphMatcherGroup, error) {
	var gData MphMatcherGroup
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&gData); err != nil {
		return nil, err
	}

	g := NewMphMatcherGroup()
	g.Ac = gData.Ac
	g.OtherMatchers = gData.OtherMatchers
	g.Rules = gData.Rules
	g.Level0 = gData.Level0
	g.Level0Mask = gData.Level0Mask
	g.Level1 = gData.Level1
	g.Level1Mask = gData.Level1Mask
	g.Count = gData.Count

	return g, nil
}
