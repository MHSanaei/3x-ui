package runtime

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Every local per-client apply sends this map to the core, so a reverse client
// that loses its tag here is one the core will not let open a reverse tunnel.
func TestClientUserMapCarriesTheReverseTag(t *testing.T) {
	reverse := &model.ClientReverse{Tag: "portal"}
	carried := clientUserMap(model.Client{
		Email:   "reverse@example.test",
		ID:      "5f2eb9d6-3a2f-4a55-9812-6ea1e2f7a333",
		Reverse: reverse,
	})["reverse"]

	tag, ok := carried.(*model.ClientReverse)
	if !ok {
		t.Fatalf("the map carries reverse %#v, want the client's own *ClientReverse", carried)
	}
	if tag != reverse {
		t.Fatalf("the map carries reverse tag %q, want %q", tag.Tag, reverse.Tag)
	}
}
