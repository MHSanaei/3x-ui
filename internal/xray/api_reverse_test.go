package xray

import (
	"testing"

	"github.com/xtls/xray-core/proxy/vless"
	"google.golang.org/protobuf/proto"
)

// typedReverse stands in for model.ClientReverse, which this package cannot
// import (model imports xray); it marshals to the same {"tag":"…"} shape.
type typedReverse struct {
	Tag string `json:"tag"`
}

// A reverse client's account has to carry its tag: removing the user drops the
// outbound handler, and only the tag lets the core rebuild it (GetReverse).
func TestBuildUserAccountCarriesTheReverseTag(t *testing.T) {
	cases := []struct {
		name    string
		reverse any
		want    string
	}{
		{"settings json shape", map[string]any{"tag": "portal"}, "portal"},
		{"typed client field", &typedReverse{Tag: "portal"}, "portal"},
		{"blank tag", map[string]any{"tag": ""}, ""},
		{"typed nil", (*typedReverse)(nil), ""},
		{"absent", nil, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := map[string]any{
				"email":   "reverse@example.test",
				"id":      "5f2eb9d6-3a2f-4a55-9812-6ea1e2f7a333",
				"reverse": tc.reverse,
			}
			tm, err := buildUserAccount("vless", user)
			if err != nil {
				t.Fatalf("buildUserAccount: %v", err)
			}
			if tm == nil {
				t.Fatal("buildUserAccount returned no account for vless")
			}
			account := new(vless.Account)
			if err := proto.Unmarshal(tm.Value, account); err != nil {
				t.Fatalf("unmarshal vless account: %v", err)
			}
			if got := account.GetReverse().GetTag(); got != tc.want {
				t.Fatalf("the account carries reverse tag %q, want %q", got, tc.want)
			}
		})
	}
}
