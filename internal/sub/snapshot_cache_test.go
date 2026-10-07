package sub

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func snapshotTestBuild(port int) func() ([]*model.Inbound, map[string]xray.ClientTraffic, error) {
	return func() ([]*model.Inbound, map[string]xray.ClientTraffic, error) {
		return []*model.Inbound{{Id: 1, Tag: "in-1", Port: port}},
			map[string]xray.ClientTraffic{"a@e": {Email: "a@e", Up: 10, Down: 20}}, nil
	}
}

func TestSnapshotStoreHitMissExpiry(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	store := newSubSnapshotStore(10*time.Second, 8)
	store.now = func() time.Time { return now }
	var builds atomic.Int32
	build := func() ([]*model.Inbound, map[string]xray.ClientTraffic, error) {
		builds.Add(1)
		return []*model.Inbound{{Id: 1, Tag: "in-1", Port: 443}},
			map[string]xray.ClientTraffic{"a@e": {Email: "a@e", Up: 10}}, nil
	}

	first, _, err := store.inboundsFor("s1", build)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if builds.Load() != 1 {
		t.Fatalf("builds after miss = %d, want 1", builds.Load())
	}

	second, stats, err := store.inboundsFor("s1", build)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if builds.Load() != 1 {
		t.Fatalf("hit rebuilt the snapshot: builds = %d", builds.Load())
	}
	if first[0] == second[0] {
		t.Fatal("hit must hand out freshly decoded structs, not the first caller's")
	}

	first[0].Port = 1234
	third, _, err := store.inboundsFor("s1", build)
	if err != nil {
		t.Fatalf("third read: %v", err)
	}
	if third[0].Port != 443 {
		t.Fatalf("caller mutation leaked into the cache: port = %d", third[0].Port)
	}

	stats["a@e"] = xray.ClientTraffic{Up: 999}
	_, stats2, err := store.inboundsFor("s1", build)
	if err != nil {
		t.Fatalf("fourth read: %v", err)
	}
	if stats2["a@e"].Up != 10 {
		t.Fatalf("stats map shared with a caller: up = %d", stats2["a@e"].Up)
	}

	now = now.Add(11 * time.Second)
	if _, _, err := store.inboundsFor("s1", build); err != nil {
		t.Fatalf("post-expiry read: %v", err)
	}
	if builds.Load() != 2 {
		t.Fatalf("expired entry must rebuild: builds = %d", builds.Load())
	}
}

func TestSnapshotStoreSingleFlight(t *testing.T) {
	store := newSubSnapshotStore(time.Minute, 8)
	release := make(chan struct{})
	var builds atomic.Int32
	build := func() ([]*model.Inbound, map[string]xray.ClientTraffic, error) {
		builds.Add(1)
		<-release
		return []*model.Inbound{{Id: 7, Port: 8443}}, nil, nil
	}

	const callers = 8
	var wg sync.WaitGroup
	ports := make([]int, callers)
	for i := range ports {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			inbounds, _, err := store.inboundsFor("sf", build)
			if err != nil {
				t.Errorf("caller %d: %v", i, err)
				return
			}
			ports[i] = inbounds[0].Port
		}(i)
	}
	// Let every caller reach the shared build before releasing it.
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()

	if builds.Load() != 1 {
		t.Fatalf("%d concurrent callers produced %d builds, want 1", callers, builds.Load())
	}
	for i, port := range ports {
		if port != 8443 {
			t.Fatalf("caller %d got port %d, want 8443", i, port)
		}
	}
}

func TestSnapshotStoreEvictsByCapacity(t *testing.T) {
	store := newSubSnapshotStore(time.Minute, 2)
	var builds atomic.Int32
	build := func() ([]*model.Inbound, map[string]xray.ClientTraffic, error) {
		builds.Add(1)
		return []*model.Inbound{{Id: int(builds.Load())}}, nil, nil
	}
	for _, key := range []string{"k1", "k2", "k3"} {
		if _, _, err := store.inboundsFor(key, build); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	if builds.Load() != 3 {
		t.Fatalf("builds = %d, want 3", builds.Load())
	}

	before := builds.Load()
	if _, _, err := store.inboundsFor("k3", build); err != nil {
		t.Fatalf("k3 re-read: %v", err)
	}
	if builds.Load() != before {
		t.Fatal("most recent entry was evicted")
	}
	if _, _, err := store.inboundsFor("k1", build); err != nil {
		t.Fatalf("k1 re-read: %v", err)
	}
	if builds.Load() != before+1 {
		t.Fatal("evicted entry was served from cache")
	}
}

// The subscription-body path may serve the 10s snapshot after a behind-the-
// scenes DB change, while panel-side reads on a second service see it at once.
func TestGetInboundsBySubIdCacheGate(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "gate", 4601, 1, `{"network":"tcp","security":"none"}`)

	cached := NewSubService("")
	cached.subscriptionBody = true
	fresh := NewSubService("")

	first, err := cached.getInboundsBySubId("s1")
	if err != nil {
		t.Fatalf("cached read: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("inbounds = %d, want 1", len(first))
	}

	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", first[0].Id).
		Update("enable", false).Error; err != nil {
		t.Fatalf("disable inbound: %v", err)
	}

	stale, err := cached.getInboundsBySubId("s1")
	if err != nil {
		t.Fatalf("cached re-read: %v", err)
	}
	if len(stale) != 1 {
		t.Fatal("subscription body must serve the cached snapshot within the TTL")
	}

	updated, err := fresh.getInboundsBySubId("s1")
	if err != nil {
		t.Fatalf("panel-side read: %v", err)
	}
	if len(updated) != 0 {
		t.Fatal("panel-side reads must bypass the snapshot cache")
	}
}
