package job

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Regression (#6283): a node onboarded in selected mode with an empty tag
// list empties its snapshot via FilterNodeSnapshot before the merge sees it,
// so that sync adopts nothing and must not stamp InboundsAdoptedAt.
func TestSyncCanAdoptInbounds(t *testing.T) {
	cases := []struct {
		name     string
		node     *model.Node
		aliases  []string
		expected bool
	}{
		{"all mode always adopts", &model.Node{InboundSyncMode: "all"}, nil, true},
		{
			"selected with tags adopts",
			&model.Node{InboundSyncMode: "selected", InboundTags: []string{"in-443-tcp"}},
			nil,
			true,
		},
		{
			"selected empty with adopted alias adopts",
			&model.Node{InboundSyncMode: "selected"},
			[]string{"in-443-tcp"},
			true,
		},
		{
			// The reported bug: registering in selected mode and choosing
			// tags afterwards stamped adoption while adopting nothing.
			"selected empty with no aliases adopts nothing",
			&model.Node{InboundSyncMode: "selected"},
			nil,
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := syncCanAdoptInbounds(c.node, c.aliases); got != c.expected {
				t.Fatalf("syncCanAdoptInbounds(%+v, %v) = %v, want %v", c.node, c.aliases, got, c.expected)
			}
		})
	}
}
