package singbox

import (
	"context"
	"fmt"
	"io"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const singBoxAPIAddress = "127.0.0.1:10091"

type connectionSubscribeRequest struct {
	Interval int64
}

type closeConnectionRequest struct {
	ID string
}

type singBoxConnection struct {
	ID          string
	Inbound     string
	InboundType string
	Network     string
	Source      string
	Destination string
	Domain      string
	Protocol    string
	User        string
	FromOutbound string
	Outbound     string
	CreatedAt   int64
	Uplink      int64
	Downlink    int64
	Chain       []string
}

type connectionEvent struct {
	Type       int32
	ID         string
	Connection *singBoxConnection
}

type connectionEvents struct {
	Events []*connectionEvent
	Reset  bool
}

type connectionAPIProtoCodec struct{}

func (connectionAPIProtoCodec) Name() string { return "singbox-api" }

func (connectionAPIProtoCodec) Marshal(v any) ([]byte, error) {
	switch req := v.(type) {
	case *connectionSubscribeRequest:
		var out []byte
		if req.Interval != 0 {
			out = appendVarintField(out, 1, uint64(req.Interval))
		}
		return out, nil
	case *closeConnectionRequest:
		return appendStringField(nil, 1, req.ID), nil
	default:
		return nil, fmt.Errorf("unsupported sing-box API request type %T", v)
	}
}

func (connectionAPIProtoCodec) Unmarshal(data []byte, v any) error {
	resp, ok := v.(*connectionEvents)
	if !ok {
		if _, empty := v.(*emptypb.Empty); empty {
			return nil
		}
		return fmt.Errorf("unsupported sing-box API response type %T", v)
	}
	resp.Events = resp.Events[:0]
	resp.Reset = false
	for len(data) > 0 {
		field, wire, n, err := consumeKey(data)
		if err != nil { return err }
		data = data[n:]
		switch field {
		case 1:
			if wire != 2 { return fmt.Errorf("invalid ConnectionEvent wire type %d", wire) }
			payload, used, err := consumeBytes(data)
			if err != nil { return err }
			event, err := decodeConnectionEvent(payload)
			if err != nil { return err }
			resp.Events = append(resp.Events, event)
			data = data[used:]
		case 2:
			if wire != 0 { return fmt.Errorf("invalid ConnectionEvents.reset wire type %d", wire) }
			value, used := binaryUvarint(data)
			if used <= 0 { return fmt.Errorf("invalid ConnectionEvents.reset") }
			resp.Reset = value != 0
			data = data[used:]
		default:
			used, err := skipWire(data, wire)
			if err != nil { return err }
			data = data[used:]
		}
	}
	return nil
}

func binaryUvarint(data []byte) (uint64, int) {
	return uvarint(data)
}

func uvarint(data []byte) (uint64, int) {
	var value uint64
	var shift uint
	for i, b := range data {
		value |= uint64(b&0x7f) << shift
		if b < 0x80 {
			return value, i + 1
		}
		shift += 7
		if shift >= 64 {
			return 0, -1
		}
	}
	return 0, -1
}

func decodeConnectionEvent(data []byte) (*connectionEvent, error) {
	event := &connectionEvent{}
	for len(data) > 0 {
		field, wire, n, err := consumeKey(data)
		if err != nil { return nil, err }
		data = data[n:]
		switch field {
		case 1:
			if wire != 0 { return nil, fmt.Errorf("invalid ConnectionEvent.type wire type %d", wire) }
			value, used := uvarint(data)
			if used <= 0 { return nil, fmt.Errorf("invalid ConnectionEvent.type") }
			event.Type = int32(value)
			data = data[used:]
		case 2:
			if wire != 2 { return nil, fmt.Errorf("invalid ConnectionEvent.id wire type %d", wire) }
			value, used, err := consumeBytes(data)
			if err != nil { return nil, err }
			event.ID = string(value)
			data = data[used:]
		case 3:
			if wire != 2 { return nil, fmt.Errorf("invalid ConnectionEvent.connection wire type %d", wire) }
			value, used, err := consumeBytes(data)
			if err != nil { return nil, err }
			connection, err := decodeConnection(value)
			if err != nil { return nil, err }
			event.Connection = connection
			data = data[used:]
		default:
			used, err := skipWire(data, wire)
			if err != nil { return nil, err }
			data = data[used:]
		}
	}
	return event, nil
}

func decodeConnection(data []byte) (*singBoxConnection, error) {
	connection := &singBoxConnection{}
	for len(data) > 0 {
		field, wire, n, err := consumeKey(data)
		if err != nil { return nil, err }
		data = data[n:]
		switch field {
		case 1, 2, 3, 5, 6, 7, 8, 9, 10, 11, 19:
			if wire != 2 { return nil, fmt.Errorf("invalid Connection field %d wire type %d", field, wire) }
			value, used, err := consumeBytes(data)
			if err != nil { return nil, err }
			text := string(value)
			switch field {
			case 1: connection.ID = text
			case 2: connection.Inbound = text
			case 3: connection.InboundType = text
			case 5: connection.Network = text
			case 6: connection.Source = text
			case 7: connection.Destination = text
			case 8: connection.Domain = text
			case 9: connection.Protocol = text
			case 10: connection.User = text
			case 11: connection.FromOutbound = text
			case 19: connection.Outbound = text
			}
			data = data[used:]
		case 12, 14, 15:
			if wire != 0 { return nil, fmt.Errorf("invalid Connection numeric field %d wire type %d", field, wire) }
			value, used := uvarint(data)
			if used <= 0 { return nil, fmt.Errorf("invalid Connection numeric field %d", field) }
			switch field {
			case 12: connection.CreatedAt = int64(value)
			case 14: connection.Uplink = int64(value)
			case 15: connection.Downlink = int64(value)
			}
			data = data[used:]
		case 21:
			if wire != 2 { return nil, fmt.Errorf("invalid Connection.chainList wire type %d", wire) }
			value, used, err := consumeBytes(data)
			if err != nil { return nil, err }
			connection.Chain = append(connection.Chain, string(value))
			data = data[used:]
		default:
			used, err := skipWire(data, wire)
			if err != nil { return nil, err }
			data = data[used:]
		}
	}
	return connection, nil
}

type ConnectionAPIClient struct {
	conn *grpc.ClientConn
}

func NewConnectionAPIClient() *ConnectionAPIClient {
	return &ConnectionAPIClient{}
}

func (c *ConnectionAPIClient) Close() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *ConnectionAPIClient) connFor(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}
	dialCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(
		dialCtx,
		singBoxAPIAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil { return err }
	c.conn = conn
	return nil
}

func (c *ConnectionAPIClient) CloseConnection(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	if err := c.connFor(ctx); err != nil {
		return err
	}
	var response emptypb.Empty
	if err := c.conn.Invoke(
		ctx,
		"/daemon.StartedService/CloseConnection",
		&closeConnectionRequest{ID: id},
		&response,
		grpc.ForceCodec(connectionAPIProtoCodec{}),
	); err != nil {
		c.Close()
		return err
	}
	return nil
}

func (c *ConnectionAPIClient) Snapshot(ctx context.Context) ([]*singBoxConnection, error) {
	if err := c.connFor(ctx); err != nil { return nil, err }

	stream, err := c.conn.NewStream(
		ctx,
		&grpc.StreamDesc{ServerStreams: true},
		"/daemon.StartedService/SubscribeConnections",
		grpc.ForceCodec(connectionAPIProtoCodec{}),
	)
	if err != nil {
		c.Close()
		return nil, err
	}
	if err := stream.SendMsg(&connectionSubscribeRequest{Interval: int64(time.Second)}); err != nil {
		c.Close()
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		c.Close()
		return nil, err
	}

	var response connectionEvents
	if err := stream.RecvMsg(&response); err != nil {
		if err != io.EOF { c.Close(); return nil, err }
		return nil, nil
	}

	connections := make([]*singBoxConnection, 0, len(response.Events))
	for _, event := range response.Events {
		if event != nil && event.Connection != nil && event.Type != 2 {
			connections = append(connections, event.Connection)
		}
	}
	return connections, nil
}
