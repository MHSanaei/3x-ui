package runtime

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Only a later call for the same node pruned its pooled client, so a deleted
// node kept its client and transport cached for the life of the process.
func TestInvalidateNodeDropsItsCachedHTTPClient(t *testing.T) {
	node := &model.Node{Id: 8801, Address: "node.example.test", Port: 443, Scheme: "https", TlsVerifyMode: "skip"}
	if _, err := HTTPClientForNode(node, ""); err != nil {
		t.Fatalf("HTTPClientForNode: %v", err)
	}
	if got := nodeClientEntries(node.Id); got != 1 {
		t.Fatalf("cached clients before invalidation = %d, want 1", got)
	}

	NewManager(LocalDeps{}).InvalidateNode(node.Id)

	if got := nodeClientEntries(node.Id); got != 0 {
		t.Fatalf("cached clients after InvalidateNode = %d, want 0", got)
	}
}
