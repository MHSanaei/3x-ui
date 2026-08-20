// Copyright (c) 2026 Masterain. MIT License.
// Adapted from PIA-Wireguard-Config-Generator-GUI (commit 53686fcd).
package pia

import (
	"context"
	"sync"
	"time"
)

type Catalog struct {
	Source   ServerListSource
	CacheTTL time.Duration
	Now      func() time.Time

	mu         sync.Mutex
	cached     []Region
	schema     string
	verified   bool
	fetchedAt  time.Time
	refreshing chan struct{}
}

func NewCatalog(source ServerListSource) *Catalog {
	return &Catalog{Source: source, CacheTTL: DefaultCatalogFreshTTL, Now: time.Now}
}

func (c *Catalog) ListRegions(ctx context.Context) ([]Region, string, error) {
	for {
		c.mu.Lock()
		age := c.Now().Sub(c.fetchedAt)
		if len(c.cached) > 0 && c.verified && c.CacheTTL > 0 && age >= 0 && age < c.CacheTTL {
			regions, schema := cloneRegions(c.cached), c.schema
			c.mu.Unlock()
			return regions, schema, nil
		}
		if wait := c.refreshing; wait != nil {
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, "", ctx.Err()
			case <-wait:
				continue
			}
		}
		done := make(chan struct{})
		c.refreshing = done
		c.mu.Unlock()

		snapshot, err := c.Source.Fetch(ctx)
		var regions []Region
		var schema string
		if err == nil && !snapshot.SignatureVerified {
			err = NewError(CodeCatalogSignatureInvalid, "The PIA region list was not signature-verified.")
		}
		if err == nil {
			regions, schema, err = ParseServerList(snapshot.Payload, snapshot.SchemaHint)
		}

		c.mu.Lock()
		if err == nil {
			c.cached = cloneRegions(regions)
			c.schema = schema
			c.verified = true
			c.fetchedAt = c.Now()
		}
		c.refreshing = nil
		close(done)
		c.mu.Unlock()
		if err != nil {
			return nil, "", err
		}
		return cloneRegions(regions), schema, nil
	}
}

func cloneRegions(regions []Region) []Region {
	result := make([]Region, len(regions))
	for i, region := range regions {
		result[i] = region
		result[i].WireGuard = append([]WireGuardServer(nil), region.WireGuard...)
	}
	return result
}
