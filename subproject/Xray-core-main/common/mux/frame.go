package mux

import (
	"encoding/binary"
	"io"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/bitmask"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/common/session"
)

type SessionStatus byte

const (
	SessionStatusNew       SessionStatus = 0x01
	SessionStatusKeep      SessionStatus = 0x02
	SessionStatusEnd       SessionStatus = 0x03
	SessionStatusKeepAlive SessionStatus = 0x04
)

const (
	OptionData  bitmask.Byte = 0x01
	OptionError bitmask.Byte = 0x02
)

type TargetNetwork byte

const (
	TargetNetworkTCP TargetNetwork = 0x01
	TargetNetworkUDP TargetNetwork = 0x02
)

var addrParser = protocol.NewAddressParser(
	protocol.AddressFamilyByte(byte(protocol.AddressTypeIPv4), net.AddressFamilyIPv4),
	protocol.AddressFamilyByte(byte(protocol.AddressTypeDomain), net.AddressFamilyDomain),
	protocol.AddressFamilyByte(byte(protocol.AddressTypeIPv6), net.AddressFamilyIPv6),
	protocol.PortThenAddress(),
)

/*
Frame format
2 bytes - length
2 bytes - session id
1 bytes - status
1 bytes - option

1 byte - network
2 bytes - port
n bytes - address

*/

type FrameMetadata struct {
	Target        net.Destination
	SessionID     uint16
	Option        bitmask.Byte
	SessionStatus SessionStatus
	GlobalID      [8]byte
	Inbound       *session.Inbound
}

func (f FrameMetadata) WriteTo(b *buf.Buffer) error {
	lenBytes := b.Extend(2)

	len0 := b.Len()
	sessionBytes := b.Extend(2)
	binary.BigEndian.PutUint16(sessionBytes, f.SessionID)

	common.Must(b.WriteByte(byte(f.SessionStatus)))
	common.Must(b.WriteByte(byte(f.Option)))

	if f.SessionStatus == SessionStatusNew {
		switch f.Target.Network {
		case net.Network_TCP:
			common.Must(b.WriteByte(byte(TargetNetworkTCP)))
		case net.Network_UDP:
			common.Must(b.WriteByte(byte(TargetNetworkUDP)))
		}
		if err := addrParser.WriteAddressPort(b, f.Target.Address, f.Target.Port); err != nil {
			return err
		}
		if f.Inbound != nil {
			if f.Inbound.Source.Network == net.Network_TCP || f.Inbound.Source.Network == net.Network_UDP {
				common.Must(b.WriteByte(byte(f.Inbound.Source.Network - 1)))
				if err := addrParser.WriteAddressPort(b, f.Inbound.Source.Address, f.Inbound.Source.Port); err != nil {
					return err
				}
				if f.Inbound.Local.Network == net.Network_TCP || f.Inbound.Local.Network == net.Network_UDP {
					common.Must(b.WriteByte(byte(f.Inbound.Local.Network - 1)))
					if err := addrParser.WriteAddressPort(b, f.Inbound.Local.Address, f.Inbound.Local.Port); err != nil {
						return err
					}
				}
			}
		} else if b.UDP != nil { // make sure it's user's proxy request
			b.Write(f.GlobalID[:]) // no need to check whether it's empty
		}
	} else if b.UDP != nil {
		b.WriteByte(byte(TargetNetworkUDP))
		addrParser.WriteAddressPort(b, b.UDP.Address, b.UDP.Port)
	}

	len1 := b.Len()
	binary.BigEndian.PutUint16(lenBytes, uint16(len1-len0))
	return nil
}

// Unmarshal reads FrameMetadata from the given reader.
func (f *FrameMetadata) Unmarshal(reader io.Reader, readSourceAndLocal bool) error {
	metaLen, err := serial.ReadUint16(reader)
	if err != nil {
		return err
	}
	if metaLen > 512 {
		return errors.New("invalid metalen ", metaLen).AtError()
	}

	b := buf.New()
	defer b.Release()

	if _, err := b.ReadFullFrom(reader, int32(metaLen)); err != nil {
		return err
	}
	return f.UnmarshalFromBuffer(b, readSourceAndLocal)
}

// UnmarshalFromBuffer reads a FrameMetadata from the given buffer.
// Visible for testing only.
func (f *FrameMetadata) UnmarshalFromBuffer(b *buf.Buffer, readSourceAndLocal bool) error {
	if b.Len() < 4 {
		return errors.New("insufficient buffer: ", b.Len())
	}

	f.SessionID = binary.BigEndian.Uint16(b.BytesTo(2))
	f.SessionStatus = SessionStatus(b.Byte(2))
	f.Option = bitmask.Byte(b.Byte(3))
	f.Target.Network = net.Network_Unknown

	if f.SessionStatus == SessionStatusNew || (f.SessionStatus == SessionStatusKeep && b.Len() > 4 &&
		TargetNetwork(b.Byte(4)) == TargetNetworkUDP) { // MUST check the flag first
		if b.Len() < 8 {
			return errors.New("insufficient buffer: ", b.Len())
		}
		network := TargetNetwork(b.Byte(4))
		b.Advance(5)

		addr, port, err := addrParser.ReadAddressPort(nil, b)
		if err != nil {
			return errors.New("failed to parse address and port").Base(err)
		}

		switch network {
		case TargetNetworkTCP:
			f.Target = net.TCPDestination(addr, port)
		case TargetNetworkUDP:
			f.Target = net.UDPDestination(addr, port)
		default:
			return errors.New("unknown network type: ", network)
		}
	}

	if f.SessionStatus == SessionStatusNew && readSourceAndLocal {
		f.Inbound = &session.Inbound{}

		if b.Len() == 0 {
			return nil // for heartbeat, etc.
		}
		network := TargetNetwork(b.Byte(0))
		if network == 0 {
			return nil // may be padding
		}
		b.Advance(1)
		addr, port, err := addrParser.ReadAddressPort(nil, b)
		if err != nil {
			return errors.New("reading source: failed to parse address and port").Base(err)
		}
		switch network {
		case TargetNetworkTCP:
			f.Inbound.Source = net.TCPDestination(addr, port)
		case TargetNetworkUDP:
			f.Inbound.Source = net.UDPDestination(addr, port)
		default:
			return errors.New("reading source: unknown network type: ", network)
		}

		if b.Len() == 0 {
			return nil
		}
		network = TargetNetwork(b.Byte(0))
		if network == 0 {
			return nil
		}
		b.Advance(1)
		addr, port, err = addrParser.ReadAddressPort(nil, b)
		if err != nil {
			return errors.New("reading local: failed to parse address and port").Base(err)
		}
		switch network {
		case TargetNetworkTCP:
			f.Inbound.Local = net.TCPDestination(addr, port)
		case TargetNetworkUDP:
			f.Inbound.Local = net.UDPDestination(addr, port)
		default:
			return errors.New("reading local: unknown network type: ", network)
		}

		return nil
	}

	// Application data is essential, to test whether the pipe is closed.
	if f.SessionStatus == SessionStatusNew && f.Option.Has(OptionData) &&
		f.Target.Network == net.Network_UDP && b.Len() >= 8 {
		copy(f.GlobalID[:], b.Bytes())
	}

	return nil
}
