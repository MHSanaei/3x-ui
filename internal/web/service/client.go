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

// MarshalJSON avoids promoting model.ClientRecord.MarshalJSON, which would
// drop InboundIds and Traffic, while encoding everything in a single pass.
func (c ClientWithAttachments) MarshalJSON() ([]byte, error) {
	type recordAlias model.ClientRecord
	return json.Marshal(struct {
		recordAlias
		Reverse    json.RawMessage     `json:"reverse"`
		InboundIds []int               `json:"inboundIds"`
		Traffic    *xray.ClientTraffic `json:"traffic,omitempty"`
	}{
		recordAlias: recordAlias(c.ClientRecord),
		Reverse:     c.ReverseJSON(),
		InboundIds:  c.InboundIds,
		Traffic:     c.Traffic,
	})
}

type ClientService struct{}

// ErrClientNotInInbound is returned (wrapped) when a client cannot be located
// in an inbound's settings during deletion. Deletion treats it as non-fatal so
// the operation stays idempotent and tolerant of pre-existing data drift
// between the clients table and the inbound's settings JSON.
var ErrClientNotInInbound = errors.New("client not found in inbound")

type ClientCreatePayload struct {
	Client     model.Client           `json:"client"`
	InboundIds []int                  `json:"inboundIds"`
	LimitHwid  int                    `json:"-"`
	Traffic    *ClientPortableTraffic `json:"traffic,omitempty"`
}

const sqlInChunk = 400

type clientPayloadWithHwid struct {
	model.Client
	LimitHwid int `json:"limitHwid"`
}

func (p *ClientCreatePayload) UnmarshalJSON(data []byte) error {
	var raw struct {
		Client     json.RawMessage        `json:"client"`
		InboundIds []int                  `json:"inboundIds"`
		Traffic    *ClientPortableTraffic `json:"traffic"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var withHwid clientPayloadWithHwid
	if len(raw.Client) > 0 {
		if err := json.Unmarshal(raw.Client, &withHwid); err != nil {
			return err
		}
	}
	p.Client = withHwid.Client
	p.InboundIds = raw.InboundIds
	p.LimitHwid = withHwid.LimitHwid
	p.Traffic = raw.Traffic
	// Omit enable → true (legacy API); explicit false is preserved (#6478).
	var keys map[string]json.RawMessage
	if len(raw.Client) > 0 && json.Unmarshal(raw.Client, &keys) == nil {
		if _, ok := keys["enable"]; !ok {
			p.Client.Enable = true
		}
	}
	return nil
}

func (p ClientCreatePayload) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Client     clientPayloadWithHwid  `json:"client"`
		InboundIds []int                  `json:"inboundIds"`
		Traffic    *ClientPortableTraffic `json:"traffic,omitempty"`
	}{
		Client:     clientPayloadWithHwid{Client: p.Client, LimitHwid: p.LimitHwid},
		InboundIds: p.InboundIds,
		Traffic:    p.Traffic,
	})
}
