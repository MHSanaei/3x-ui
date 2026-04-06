package xicmp_test

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func TestICMPEchoMarshal(t *testing.T) {
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   65535,
			Seq:  65537,
			Data: nil,
		},
	}
	ICMPTypeEcho, _ := msg.Marshal(nil)
	fmt.Println("ICMPTypeEcho", len(ICMPTypeEcho), ICMPTypeEcho)

	msg = icmp.Message{
		Type: ipv4.ICMPTypeEchoReply,
		Code: 0,
		Body: &icmp.Echo{
			ID:   65535,
			Seq:  65537,
			Data: nil,
		},
	}
	ICMPTypeEchoReply, _ := msg.Marshal(nil)
	fmt.Println("ICMPTypeEchoReply", len(ICMPTypeEchoReply), ICMPTypeEchoReply)

	msg = icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &icmp.Echo{
			ID:   65535,
			Seq:  65537,
			Data: nil,
		},
	}
	ICMPTypeEchoRequest, _ := msg.Marshal(nil)
	fmt.Println("ICMPTypeEchoRequest", len(ICMPTypeEchoRequest), ICMPTypeEchoRequest)

	msg = icmp.Message{
		Type: ipv6.ICMPTypeEchoReply,
		Code: 0,
		Body: &icmp.Echo{
			ID:   65535,
			Seq:  65537,
			Data: nil,
		},
	}
	V6ICMPTypeEchoReply, _ := msg.Marshal(nil)
	fmt.Println("V6ICMPTypeEchoReply", len(V6ICMPTypeEchoReply), V6ICMPTypeEchoReply)

	if !bytes.Equal(ICMPTypeEcho[0:2], []byte{8, 0}) || !bytes.Equal(ICMPTypeEcho[4:], []byte{255, 255, 0, 1}) {
		t.Fatalf("ICMPTypeEcho Type/Code or ID/Seq mismatch: %v", ICMPTypeEcho)
	}
	if !bytes.Equal(ICMPTypeEchoReply[0:2], []byte{0, 0}) || !bytes.Equal(ICMPTypeEchoReply[4:], []byte{255, 255, 0, 1}) {
		t.Fatalf("ICMPTypeEchoReply Type/Code or ID/Seq mismatch: %v", ICMPTypeEchoReply)
	}
	if !bytes.Equal(ICMPTypeEchoRequest[0:2], []byte{128, 0}) || !bytes.Equal(ICMPTypeEchoRequest[4:], []byte{255, 255, 0, 1}) {
		t.Fatalf("ICMPTypeEchoRequest Type/Code or ID/Seq mismatch: %v", ICMPTypeEchoRequest)
	}
	if !bytes.Equal(V6ICMPTypeEchoReply[0:2], []byte{129, 0}) || !bytes.Equal(V6ICMPTypeEchoReply[4:], []byte{255, 255, 0, 1}) {
		t.Fatalf("V6ICMPTypeEchoReply Type/Code or ID/Seq mismatch: %v", V6ICMPTypeEchoReply)
	}
}
