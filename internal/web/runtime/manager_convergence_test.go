package runtime

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestManagerRemoteForRefreshesChangedCredential(t *testing.T) {
	m := NewManager(LocalDeps{})
	first, err := m.RemoteFor(&model.Node{
		Id:       1,
		Name:     "node",
		Scheme:   "https",
		Address:  "node.example.com",
		Port:     2053,
		BasePath: "/",
		ApiToken: "old-token",
	})
	if err != nil {
		t.Fatalf("first RemoteFor: %v", err)
	}
	second, err := m.RemoteFor(&model.Node{
		Id:       1,
		Name:     "node",
		Scheme:   "https",
		Address:  "node.example.com",
		Port:     2053,
		BasePath: "/",
		ApiToken: "new-token",
	})
	if err != nil {
		t.Fatalf("second RemoteFor: %v", err)
	}
	if second == first {
		t.Fatal("RemoteFor reused stale Remote after ApiToken changed")
	}
	if got := second.node.ApiToken; got != "new-token" {
		t.Fatalf("cached Remote token = %q, want new-token", got)
	}
}

func TestManagerRemoteForIdentityFields(t *testing.T) {
	base := model.Node{
		Id:                  7,
		Name:                "node-a",
		Remark:              "old remark",
		Scheme:              "https",
		Address:             "node.example.com",
		Port:                2053,
		BasePath:            "/",
		ApiToken:            "token",
		AllowPrivateAddress: true,
		TlsVerifyMode:       "pin",
		PinnedCertSha256:    "sha",
		OutboundTag:         "warp",
		Status:              "online",
		InboundCount:        1,
	}

	cases := []struct {
		name    string
		mutate  func(*model.Node)
		refresh bool
	}{
		{"same", func(*model.Node) {}, false},
		{"name does not churn", func(n *model.Node) { n.Name = "renamed" }, false},
		{"remark does not churn", func(n *model.Node) { n.Remark = "new remark" }, false},
		{"status does not churn", func(n *model.Node) { n.Status = "offline" }, false},
		{"metrics do not churn", func(n *model.Node) { n.InboundCount = 99 }, false},
		{"scheme", func(n *model.Node) { n.Scheme = "http" }, true},
		{"address", func(n *model.Node) { n.Address = "other.example.com" }, true},
		{"port", func(n *model.Node) { n.Port = 8443 }, true},
		{"base path", func(n *model.Node) { n.BasePath = "/x/" }, true},
		{"api token", func(n *model.Node) { n.ApiToken = "next" }, true},
		{"allow private", func(n *model.Node) { n.AllowPrivateAddress = false }, true},
		{"tls verify mode", func(n *model.Node) { n.TlsVerifyMode = "skip" }, true},
		{"pin", func(n *model.Node) { n.PinnedCertSha256 = "other" }, true},
		{"outbound tag", func(n *model.Node) { n.OutboundTag = "direct" }, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewManager(LocalDeps{})
			firstNode := base
			first, err := m.RemoteFor(&firstNode)
			if err != nil {
				t.Fatalf("first RemoteFor: %v", err)
			}

			nextNode := base
			tc.mutate(&nextNode)
			second, err := m.RemoteFor(&nextNode)
			if err != nil {
				t.Fatalf("second RemoteFor: %v", err)
			}
			if gotRefresh := second != first; gotRefresh != tc.refresh {
				t.Fatalf("refresh = %v, want %v", gotRefresh, tc.refresh)
			}
		})
	}
}

func TestManagerRemoteForClonesInputNode(t *testing.T) {
	m := NewManager(LocalDeps{})
	n := &model.Node{
		Id:       9,
		Scheme:   "https",
		Address:  "node.example.com",
		Port:     2053,
		BasePath: "/",
		ApiToken: "original",
	}
	rt, err := m.RemoteFor(n)
	if err != nil {
		t.Fatalf("RemoteFor: %v", err)
	}
	n.ApiToken = "mutated-after-cache"
	if got := rt.node.ApiToken; got != "original" {
		t.Fatalf("cached Remote observed caller mutation: %q", got)
	}
}
