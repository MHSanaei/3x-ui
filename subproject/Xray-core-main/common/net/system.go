package net

import "net"

// DialTCP is an alias of net.DialTCP.
var (
	DialTCP  = net.DialTCP
	DialUDP  = net.DialUDP
	DialUnix = net.DialUnix
	Dial     = net.Dial
)

type ListenConfig = net.ListenConfig

type KeepAliveConfig = net.KeepAliveConfig

var (
	Listen     = net.Listen
	ListenTCP  = net.ListenTCP
	ListenUDP  = net.ListenUDP
	ListenUnix = net.ListenUnix
)

var LookupIP = net.LookupIP

var FileConn = net.FileConn

// ParseIP is an alias of net.ParseIP
var ParseIP = net.ParseIP

var ParseCIDR = net.ParseCIDR

var ResolveIPAddr = net.ResolveIPAddr

var InterfaceByName = net.InterfaceByName

var SplitHostPort = net.SplitHostPort

var CIDRMask = net.CIDRMask

type (
	Addr       = net.Addr
	Conn       = net.Conn
	PacketConn = net.PacketConn
)

type (
	TCPAddr = net.TCPAddr
	TCPConn = net.TCPConn
)

type (
	UDPAddr = net.UDPAddr
	UDPConn = net.UDPConn
)

type (
	UnixAddr = net.UnixAddr
	UnixConn = net.UnixConn
)

type IPAddr = net.IPAddr

// IP is an alias for net.IP.
type (
	IP     = net.IP
	IPMask = net.IPMask
	IPNet  = net.IPNet
)

const (
	IPv4len = net.IPv4len
	IPv6len = net.IPv6len
)

type (
	Error     = net.Error
	AddrError = net.AddrError
)

type (
	Dialer       = net.Dialer
	Listener     = net.Listener
	TCPListener  = net.TCPListener
	UnixListener = net.UnixListener
)

var (
	ResolveTCPAddr  = net.ResolveTCPAddr
	ResolveUDPAddr  = net.ResolveUDPAddr
	ResolveUnixAddr = net.ResolveUnixAddr
)

type Resolver = net.Resolver

var DefaultResolver = net.DefaultResolver

var JoinHostPort = net.JoinHostPort

var InterfaceAddrs = net.InterfaceAddrs

var Interfaces = net.Interfaces
