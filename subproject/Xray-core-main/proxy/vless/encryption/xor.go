package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"net"

	"lukechampine.com/blake3"
)

func NewCTR(key, iv []byte) cipher.Stream {
	k := make([]byte, 32)
	blake3.DeriveKey(k, "VLESS", key) // avoids using key directly
	block, _ := aes.NewCipher(k)
	return cipher.NewCTR(block, iv)
	//chacha20.NewUnauthenticatedCipher()
}

type XorConn struct {
	net.Conn
	CTR       cipher.Stream
	PeerCTR   cipher.Stream
	OutSkip   int
	OutHeader []byte
	InSkip    int
	InHeader  []byte
}

func NewXorConn(conn net.Conn, ctr, peerCTR cipher.Stream, outSkip, inSkip int) *XorConn {
	return &XorConn{
		Conn:      conn,
		CTR:       ctr,
		PeerCTR:   peerCTR,
		OutSkip:   outSkip,
		OutHeader: make([]byte, 0, 5), // important
		InSkip:    inSkip,
		InHeader:  make([]byte, 0, 5), // important
	}
}

func (c *XorConn) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	for p := b; ; {
		if len(p) <= c.OutSkip {
			c.OutSkip -= len(p)
			break
		}
		p = p[c.OutSkip:]
		c.OutSkip = 0
		need := 5 - len(c.OutHeader)
		if len(p) < need {
			c.OutHeader = append(c.OutHeader, p...)
			c.CTR.XORKeyStream(p, p)
			break
		}
		c.OutSkip, _ = DecodeHeader(append(c.OutHeader, p[:need]...))
		c.OutHeader = c.OutHeader[:0]
		c.CTR.XORKeyStream(p[:need], p[:need])
		p = p[need:]
	}
	if _, err := c.Conn.Write(b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *XorConn) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	n, err := c.Conn.Read(b)
	for p := b[:n]; ; {
		if len(p) <= c.InSkip {
			c.InSkip -= len(p)
			break
		}
		p = p[c.InSkip:]
		c.InSkip = 0
		need := 5 - len(c.InHeader)
		if len(p) < need {
			c.PeerCTR.XORKeyStream(p, p)
			c.InHeader = append(c.InHeader, p...)
			break
		}
		c.PeerCTR.XORKeyStream(p[:need], p[:need])
		c.InSkip, _ = DecodeHeader(append(c.InHeader, p[:need]...))
		c.InHeader = c.InHeader[:0]
		p = p[need:]
	}
	return n, err
}
