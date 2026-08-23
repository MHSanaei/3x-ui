package pia

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeServerListSource struct {
	snapshot ServerListSnapshot
	err      error
	calls    int
}

func (f *fakeServerListSource) Fetch(context.Context) (ServerListSnapshot, error) {
	f.calls++
	return f.snapshot, f.err
}

func TestCatalogCachesOnlyVerifiedParsedSnapshots(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "serverlist", "v6_valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	source := &fakeServerListSource{snapshot: ServerListSnapshot{Payload: raw, SchemaHint: "6", SignatureVerified: true}}
	now := time.Unix(1_700_000_000, 0)
	catalog := NewCatalog(source)
	catalog.CacheTTL = 30 * time.Minute
	catalog.Now = func() time.Time { return now }

	first, schema, err := catalog.ListRegions(context.Background())
	if err != nil || schema != "v6" || len(first) != 1 {
		t.Fatalf("unexpected first result: schema=%q regions=%v err=%v", schema, first, err)
	}
	first[0].WireGuard[0].Hostname = "mutated-by-caller"
	second, _, err := catalog.ListRegions(context.Background())
	if err != nil || source.calls != 1 {
		t.Fatalf("verified snapshot was not cached: calls=%d err=%v", source.calls, err)
	}
	if second[0].WireGuard[0].Hostname == "mutated-by-caller" {
		t.Fatal("catalog returned mutable cached storage")
	}

	now = now.Add(-time.Second)
	if _, _, err := catalog.ListRegions(context.Background()); err != nil || source.calls != 2 {
		t.Fatalf("backward clock movement incorrectly extended the cache: calls=%d err=%v", source.calls, err)
	}

	now = now.Add(catalog.CacheTTL + time.Second)
	if _, _, err := catalog.ListRegions(context.Background()); err != nil || source.calls != 3 {
		t.Fatalf("expired snapshot was not refreshed: calls=%d err=%v", source.calls, err)
	}
}

type gatedServerListSource struct {
	snapshot  ServerListSnapshot
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
	calls     atomic.Int32
}

func (g *gatedServerListSource) Fetch(context.Context) (ServerListSnapshot, error) {
	g.calls.Add(1)
	g.startOnce.Do(func() { close(g.started) })
	<-g.release
	return g.snapshot, nil
}

func TestCatalogCoalescesConcurrentRefresh(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "serverlist", "v6_valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	source := &gatedServerListSource{
		snapshot: ServerListSnapshot{Payload: raw, SchemaHint: "6", SignatureVerified: true},
		started:  make(chan struct{}),
		release:  make(chan struct{}),
	}
	catalog := NewCatalog(source)
	catalog.CacheTTL = time.Hour

	errc := make(chan error, 2)
	go func() {
		_, _, err := catalog.ListRegions(context.Background())
		errc <- err
	}()
	<-source.started
	go func() {
		_, _, err := catalog.ListRegions(context.Background())
		errc <- err
	}()
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if source.calls.Load() > 1 {
			close(source.release)
			t.Fatalf("concurrent refresh issued %d fetches, want 1", source.calls.Load())
		}
		time.Sleep(time.Millisecond)
	}
	close(source.release)
	for range 2 {
		if err := <-errc; err != nil {
			t.Fatal(err)
		}
	}
	if source.calls.Load() != 1 {
		t.Fatalf("concurrent refresh issued %d fetches, want 1", source.calls.Load())
	}
}

func TestCatalogRejectsUnverifiedSnapshot(t *testing.T) {
	source := &fakeServerListSource{snapshot: ServerListSnapshot{
		Payload: []byte(`{"version":6,"groups":{"wg":[]},"regions":[]}`), SchemaHint: "6", SignatureVerified: false,
	}}
	catalog := NewCatalog(source)
	_, _, err := catalog.ListRegions(context.Background())
	if CodeOf(err) != CodeCatalogSignatureInvalid {
		t.Fatalf("unverified snapshot returned %s, want %s: %v", CodeOf(err), CodeCatalogSignatureInvalid, err)
	}
}
