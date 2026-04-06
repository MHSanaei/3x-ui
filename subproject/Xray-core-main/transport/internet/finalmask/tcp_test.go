package finalmask_test

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/xtls/xray-core/transport/internet/finalmask"
	"github.com/xtls/xray-core/transport/internet/finalmask/header/custom"
)

func mustSendRecvTcp(
	t *testing.T,
	from net.Conn,
	to net.Conn,
	msg []byte,
) {
	t.Helper()

	go func() {
		_, err := from.Write(msg)
		if err != nil {
			t.Error(err)
		}
	}()

	buf := make([]byte, 1024)
	n, err := io.ReadFull(to, buf[:len(msg)])
	if err != nil {
		t.Fatal(err)
	}

	if n != len(msg) {
		t.Fatalf("unexpected size: %d", n)
	}

	if !bytes.Equal(buf[:n], msg) {
		t.Fatalf("unexpected data %q", buf[:n])
	}
}

type layerMaskTcp struct {
	name string
	mask finalmask.Tcpmask
}

func TestConnReadWrite(t *testing.T) {
	cases := []layerMaskTcp{
		{
			name: "custom",
			mask: &custom.TCPConfig{
				Clients: []*custom.TCPSequence{
					{
						Sequence: []*custom.TCPItem{
							{
								Packet: []byte{1},
							},
							{
								Rand: 1,
							},
						},
					},
				},
				Servers: []*custom.TCPSequence{
					{
						Sequence: []*custom.TCPItem{
							{
								Packet: []byte{2},
							},
							{
								Rand: 1,
							},
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mask := c.mask

			maskManager := finalmask.NewTcpmaskManager([]finalmask.Tcpmask{mask})

			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}

			client, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				t.Fatal(err)
			}

			client, err = maskManager.WrapConnClient(client)
			if err != nil {
				t.Fatal(err)
			}

			server, err := ln.Accept()
			if err != nil {
				t.Fatal(err)
			}

			server, err = maskManager.WrapConnServer(server)
			if err != nil {
				t.Fatal(err)
			}

			_ = client.SetDeadline(time.Now().Add(time.Second))
			_ = server.SetDeadline(time.Now().Add(time.Second))

			mustSendRecvTcp(t, client, server, []byte("client -> server"))
			mustSendRecvTcp(t, server, client, []byte("server -> client"))

			mustSendRecvTcp(t, client, server, []byte{})
			mustSendRecvTcp(t, server, client, []byte{})
		})
	}
}
