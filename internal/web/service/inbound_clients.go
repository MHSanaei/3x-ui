package service

import (


)

type CopyClientsResult struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors"`
}

// EnrichClientStats parses each inbound's clients once, fills in the
// UUID/SubId fields on the preloaded ClientStats, and tops up rows owned by
// a sibling inbound (shared-email mode — the row is keyed on email so it
// only preloads on its owning inbound).


// EmailUsedByOtherInbounds reports whether email lives in any inbound other
// than exceptInboundId. Empty email returns false.


// EmailsByInbound returns the list of client emails currently configured on
// an inbound's settings.clients[]. Used by the "delete all clients" flow on
// the inbounds page, which then feeds the list into ClientService.BulkDelete.
