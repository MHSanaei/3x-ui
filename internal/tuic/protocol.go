package tuic

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
)

const (
	// ProtocolVersion is the TUIC protocol version (0x05).
	ProtocolVersion byte = 0x05

	// Command types
	CmdAuthenticate byte = 0x00
	CmdConnect      byte = 0x01
	CmdPacket       byte = 0x02
	CmdDissociate   byte = 0x03
	CmdHeartbeat    byte = 0x04

	// Address types
	AddrTypeDomain byte = 0x00
	AddrTypeIPv4   byte = 0x01
	AddrTypeIPv6   byte = 0x02
	AddrTypeNone   byte = 0xff
)

var (
	ErrInvalidVersion = errors.New("tuic: invalid protocol version")
	ErrInvalidCmd     = errors.New("tuic: invalid command type")
	ErrInvalidAddr    = errors.New("tuic: invalid address format")
)

// Address represents a network endpoint (host + port) in TUIC v5.
type Address struct {
	Type byte
	Host string
	IP   net.IP
	Port uint16
}

// String returns "host:port" suitable for net.Dial.
func (a *Address) String() string {
	if a == nil || a.Type == AddrTypeNone {
		return ""
	}
	if len(a.IP) > 0 {
		return net.JoinHostPort(a.IP.String(), strconv.Itoa(int(a.Port)))
	}
	return net.JoinHostPort(a.Host, strconv.Itoa(int(a.Port)))
}

// ReadAddress decodes a TUIC v5 address from the reader.
func ReadAddress(r io.Reader) (*Address, error) {
	var typeBuf [1]byte
	if _, err := io.ReadFull(r, typeBuf[:]); err != nil {
		return nil, err
	}
	addrType := typeBuf[0]

	if addrType == AddrTypeNone {
		return &Address{Type: AddrTypeNone}, nil
	}

	addr := &Address{Type: addrType}

	switch addrType {
	case AddrTypeIPv4:
		var ip [4]byte
		if _, err := io.ReadFull(r, ip[:]); err != nil {
			return nil, err
		}
		addr.IP = net.IP(ip[:])
		addr.Host = addr.IP.String()

	case AddrTypeIPv6:
		var ip [16]byte
		if _, err := io.ReadFull(r, ip[:]); err != nil {
			return nil, err
		}
		addr.IP = net.IP(ip[:])
		addr.Host = addr.IP.String()

	case AddrTypeDomain:
		var lenBuf [1]byte
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			return nil, err
		}
		dLen := int(lenBuf[0])
		if dLen == 0 {
			return nil, ErrInvalidAddr
		}
		domainBuf := make([]byte, dLen)
		if _, err := io.ReadFull(r, domainBuf); err != nil {
			return nil, err
		}
		addr.Host = string(domainBuf)

	default:
		return nil, fmt.Errorf("%w: unknown type 0x%02x", ErrInvalidAddr, addrType)
	}

	var portBuf [2]byte
	if _, err := io.ReadFull(r, portBuf[:]); err != nil {
		return nil, err
	}
	addr.Port = binary.BigEndian.Uint16(portBuf[:])

	return addr, nil
}

// WriteAddress encodes a TUIC v5 address to the writer.
func WriteAddress(w io.Writer, addr *Address) error {
	if addr == nil || addr.Type == AddrTypeNone {
		_, err := w.Write([]byte{AddrTypeNone})
		return err
	}

	var buf []byte
	switch addr.Type {
	case AddrTypeIPv4:
		ip4 := addr.IP.To4()
		if len(ip4) != 4 {
			return ErrInvalidAddr
		}
		buf = make([]byte, 1+4+2)
		buf[0] = AddrTypeIPv4
		copy(buf[1:5], ip4)
		binary.BigEndian.PutUint16(buf[5:7], addr.Port)

	case AddrTypeIPv6:
		ip16 := addr.IP.To16()
		if len(ip16) != 16 {
			return ErrInvalidAddr
		}
		buf = make([]byte, 1+16+2)
		buf[0] = AddrTypeIPv6
		copy(buf[1:17], ip16)
		binary.BigEndian.PutUint16(buf[17:19], addr.Port)

	case AddrTypeDomain:
		dLen := len(addr.Host)
		if dLen == 0 || dLen > 255 {
			return ErrInvalidAddr
		}
		buf = make([]byte, 1+1+dLen+2)
		buf[0] = AddrTypeDomain
		buf[1] = byte(dLen)
		copy(buf[2:2+dLen], []byte(addr.Host))
		binary.BigEndian.PutUint16(buf[2+dLen:4+dLen], addr.Port)

	default:
		return ErrInvalidAddr
	}

	_, err := w.Write(buf)
	return err
}

// ReadCommand reads the 2-byte TUIC command header: [VER (1)][TYPE (1)].
func ReadCommand(r io.Reader) (byte, byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, 0, err
	}
	if hdr[0] != ProtocolVersion {
		return hdr[0], hdr[1], fmt.Errorf("%w: got 0x%02x, want 0x%02x", ErrInvalidVersion, hdr[0], ProtocolVersion)
	}
	return hdr[0], hdr[1], nil
}

// PacketHeader represents the header of a UDP Packet command (0x02).
type PacketHeader struct {
	AssocID   uint16
	PktID     uint16
	FragTotal uint8
	FragID    uint8
	Size      uint16
	Addr      *Address
}

// ReadPacketHeader reads the packet command fields following [VER][0x02].
func ReadPacketHeader(r io.Reader) (*PacketHeader, error) {
	var fixed [8]byte
	if _, err := io.ReadFull(r, fixed[:]); err != nil {
		return nil, err
	}
	ph := &PacketHeader{
		AssocID:   binary.BigEndian.Uint16(fixed[0:2]),
		PktID:     binary.BigEndian.Uint16(fixed[2:4]),
		FragTotal: fixed[4],
		FragID:    fixed[5],
		Size:      binary.BigEndian.Uint16(fixed[6:8]),
	}

	addr, err := ReadAddress(r)
	if err != nil {
		return nil, err
	}
	ph.Addr = addr
	return ph, nil
}

// WritePacket writes a complete Packet command frame to w.
func WritePacket(w io.Writer, assocID, pktID uint16, fragTotal, fragID uint8, addr *Address, payload []byte) error {
	hdr := make([]byte, 10)
	hdr[0] = ProtocolVersion
	hdr[1] = CmdPacket
	binary.BigEndian.PutUint16(hdr[2:4], assocID)
	binary.BigEndian.PutUint16(hdr[4:6], pktID)
	hdr[6] = fragTotal
	hdr[7] = fragID
	binary.BigEndian.PutUint16(hdr[8:10], uint16(len(payload)))

	if _, err := w.Write(hdr); err != nil {
		return err
	}
	if err := WriteAddress(w, addr); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}
