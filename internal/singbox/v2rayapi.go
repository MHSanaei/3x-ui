package singbox

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const v2rayStatsAddress = "127.0.0.1:10086"

type v2rayStat struct {
	Name  string
	Value int64
}

type v2rayQueryRequest struct {
	Pattern  string
	Patterns []string
	Regexp   bool
	Reset    bool
}

type v2rayQueryResponse struct {
	Stats []v2rayStat
}

type v2rayProtoCodec struct{}

func (v2rayProtoCodec) Name() string { return "singbox-v2rayapi" }

func (v2rayProtoCodec) Marshal(v any) ([]byte, error) {
	req, ok := v.(*v2rayQueryRequest)
	if !ok {
		return nil, fmt.Errorf("unsupported V2Ray API request type %T", v)
	}
	var out []byte
	if req.Pattern != "" {
		out = appendStringField(out, 1, req.Pattern)
	}
	if req.Reset {
		out = appendVarintField(out, 2, 1)
	}
	for _, pattern := range req.Patterns {
		out = appendStringField(out, 3, pattern)
	}
	if req.Regexp {
		out = appendVarintField(out, 4, 1)
	}
	return out, nil
}

func (v2rayProtoCodec) Unmarshal(data []byte, v any) error {
	resp, ok := v.(*v2rayQueryResponse)
	if !ok {
		return fmt.Errorf("unsupported V2Ray API response type %T", v)
	}
	resp.Stats = resp.Stats[:0]
	for len(data) > 0 {
		field, wire, n, err := consumeKey(data)
		if err != nil { return err }
		data = data[n:]
		switch field {
		case 1:
			if wire != 2 { return fmt.Errorf("invalid Stat wire type %d", wire) }
			payload, used, err := consumeBytes(data)
			if err != nil { return err }
			stat, err := decodeStat(payload)
			if err != nil { return err }
			resp.Stats = append(resp.Stats, stat)
			data = data[used:]
		default:
			used, err := skipWire(data, wire)
			if err != nil { return err }
			data = data[used:]
		}
	}
	return nil
}

func appendStringField(dst []byte, field int, value string) []byte {
	dst = appendKey(dst, field, 2)
	dst = appendUvarint(dst, uint64(len(value)))
	return append(dst, value...)
}

func appendKey(dst []byte, field, wire int) []byte {
	return appendUvarint(dst, uint64(field<<3|wire))
}

func appendUvarint(dst []byte, value uint64) []byte {
	var buf [10]byte
	n := binary.PutUvarint(buf[:], value)
	return append(dst, buf[:n]...)
}

func appendVarintField(dst []byte, field int, value uint64) []byte {
	dst = appendKey(dst, field, 0)
	return appendUvarint(dst, value)
}

func consumeKey(data []byte) (field, wire, used int, err error) {
	key, n := binary.Uvarint(data)
	if n <= 0 { return 0, 0, 0, fmt.Errorf("invalid protobuf key") }
	return int(key >> 3), int(key & 7), n, nil
}

func consumeBytes(data []byte) ([]byte, int, error) {
	n, used := binary.Uvarint(data)
	if used <= 0 || n > uint64(len(data)-used) {
		return nil, 0, fmt.Errorf("invalid protobuf length")
	}
	start := used
	end := start + int(n)
	return data[start:end], end, nil
}

func decodeStat(data []byte) (v2rayStat, error) {
	var stat v2rayStat
	for len(data) > 0 {
		field, wire, n, err := consumeKey(data)
		if err != nil { return stat, err }
		data = data[n:]
		switch field {
		case 1:
			if wire != 2 { return stat, fmt.Errorf("invalid Stat.name wire type %d", wire) }
			value, used, err := consumeBytes(data)
			if err != nil { return stat, err }
			stat.Name = string(value)
			data = data[used:]
		case 2:
			if wire != 0 { return stat, fmt.Errorf("invalid Stat.value wire type %d", wire) }
			value, used := binary.Varint(data)
			if used <= 0 { return stat, fmt.Errorf("invalid Stat.value") }
			stat.Value = value
			data = data[used:]
		default:
			used, err := skipWire(data, wire)
			if err != nil { return stat, err }
			data = data[used:]
		}
	}
	return stat, nil
}

func skipWire(data []byte, wire int) (int, error) {
	switch wire {
	case 0:
		_, n := binary.Uvarint(data)
		if n <= 0 { return 0, fmt.Errorf("invalid varint") }
		return n, nil
	case 1:
		if len(data) < 8 { return 0, io.ErrUnexpectedEOF }
		return 8, nil
	case 2:
		_, n, err := consumeBytes(data)
		return n, err
	case 5:
		if len(data) < 4 { return 0, io.ErrUnexpectedEOF }
		return 4, nil
	default:
		return 0, fmt.Errorf("unsupported protobuf wire type %d", wire)
	}
}

type V2RayStatsClient struct {
	mu   sync.Mutex
	conn *grpc.ClientConn
}

func (c *V2RayStatsClient) closeLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *V2RayStatsClient) connFor(ctx context.Context) (*grpc.ClientConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn, nil
	}
	dialCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(
		dialCtx,
		v2rayStatsAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

func (c *V2RayStatsClient) Query(ctx context.Context, patterns []string, reset bool) ([]v2rayStat, error) {
	conn, err := c.connFor(ctx)
	if err != nil { return nil, err }
	req := &v2rayQueryRequest{Patterns: patterns, Reset: reset}
	var resp v2rayQueryResponse
	err = conn.Invoke(
		ctx,
		"/experimental.v2rayapi.StatsService/QueryStats",
		req,
		&resp,
		grpc.ForceCodec(v2rayProtoCodec{}),
	)
	if err != nil {
		c.mu.Lock()
		c.closeLocked()
		c.mu.Unlock()
		return nil, err
	}
	return resp.Stats, nil
}

func (c *V2RayStatsClient) QueryUser(ctx context.Context, email string, reset bool) (up, down int64, err error) {
	stats, err := c.Query(ctx, []string{
		"user>>>" + email + ">>>traffic>>>uplink",
		"user>>>" + email + ">>>traffic>>>downlink",
	}, reset)
	if err != nil { return 0, 0, err }
	for _, stat := range stats {
		switch stat.Name {
		case "user>>>" + email + ">>>traffic>>>uplink":
			up += stat.Value
		case "user>>>" + email + ">>>traffic>>>downlink":
			down += stat.Value
		}
	}
	return up, down, nil
}
