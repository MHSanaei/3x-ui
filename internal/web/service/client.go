// Package service implements the panel's business-logic layer.
//
// ClientService owns the lifecycle of VPN clients: creation, update, deletion,
// attach/detach to inbounds, bulk operations, group membership, traffic resets,
// and the paginated clients listing. Its surface is split across client_*.go
// files by responsibility (see each file's contents); they all belong to the
// same package, so the split is purely organizational. ClientService and
// InboundService are mutually dependent — most ClientService methods take an
// *InboundService and InboundService embeds a ClientService — which is why the
// client code lives in package service rather than a sub-package.
package service

import (
	"encoding/json"
	"errors"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type ClientWithAttachments struct {
	model.ClientRecord
	InboundIds []int               `json:"inboundIds"`
	Traffic    *xray.ClientTraffic `json:"traffic,omitempty"`
}

// MarshalJSON is required because model.ClientRecord defines its own
// MarshalJSON. Go promotes the embedded method to the outer struct, so without
// this the encoder would call ClientRecord.MarshalJSON for the whole value and
// silently drop InboundIds and Traffic from the API response.
func (c ClientWithAttachments) MarshalJSON() ([]byte, error) {
	rec, err := json.Marshal(c.ClientRecord)
	if err != nil {
		return nil, err
	}
	extras := struct {
		InboundIds []int               `json:"inboundIds"`
		Traffic    *xray.ClientTraffic `json:"traffic,omitempty"`
	}{InboundIds: c.InboundIds, Traffic: c.Traffic}
	extra, err := json.Marshal(extras)
	if err != nil {
		return nil, err
	}
	if len(rec) < 2 || rec[len(rec)-1] != '}' || len(extra) <= 2 {
		return rec, nil
	}
	const maxMarshalSize = 256 << 20
	if len(rec) > maxMarshalSize || len(extra) > maxMarshalSize {
		return rec, nil
	}
	out := make([]byte, 0, len(rec)+len(extra))
	out = append(out, rec[:len(rec)-1]...)
	if len(rec) > 2 {
		out = append(out, ',')
	}
	out = append(out, extra[1:]...)
	return out, nil
}

type ClientService struct{}

// ErrClientNotInInbound is returned (wrapped) when a client cannot be located
// in an inbound's settings during deletion. Deletion treats it as non-fatal so
// the operation stays idempotent and tolerant of pre-existing data drift
// between the clients table and the inbound's settings JSON.
var ErrClientNotInInbound = errors.New("client not found in inbound")

type ClientCreatePayload struct {
	Client     model.Client `json:"client"`
	InboundIds []int        `json:"inboundIds"`
}

const sqlInChunk = 400
