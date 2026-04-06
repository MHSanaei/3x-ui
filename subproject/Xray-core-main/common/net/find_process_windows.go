//go:build windows

package net

import (
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/xtls/xray-core/common/errors"
)

const (
	tcpTableFunc    = "GetExtendedTcpTable"
	tcpTablePidConn = 4
	udpTableFunc    = "GetExtendedUdpTable"
	udpTablePid     = 1
)

var (
	getExTCPTable uintptr
	getExUDPTable uintptr

	once    sync.Once
	initErr error
)

func initWin32API() error {
	h, err := windows.LoadLibrary("iphlpapi.dll")
	if err != nil {
		return errors.New("LoadLibrary iphlpapi.dll failed").Base(err)
	}

	getExTCPTable, err = windows.GetProcAddress(h, tcpTableFunc)
	if err != nil {
		return errors.New("GetProcAddress of ", tcpTableFunc, " failed").Base(err)
	}

	getExUDPTable, err = windows.GetProcAddress(h, udpTableFunc)
	if err != nil {
		return errors.New("GetProcAddress of ", udpTableFunc, " failed").Base(err)
	}

	return nil
}

func FindProcess(dest Destination) (PID int, Name string, AbsolutePath string, err error) {
	once.Do(func() {
		initErr = initWin32API()
	})
	if initErr != nil {
		return 0, "", "", initErr
	}
	isLocal, err := IsLocal(dest.Address.IP())
	if err != nil {
		return 0, "", "", errors.New("failed to determine if address is local: ", err)
	}
	if !isLocal {
		return 0, "", "", ErrNotLocal
	}
	if dest.Network != Network_TCP && dest.Network != Network_UDP {
		panic("Unsupported network type for process lookup.")
	}
	// the core should never has a domain as source(?
	if dest.Address.Family() == AddressFamilyDomain {
		panic("Domain addresses are not supported for process lookup.")
	}
	var class int
	var fn uintptr
	switch dest.Network {
	case Network_TCP:
		fn = getExTCPTable
		class = tcpTablePidConn
	case Network_UDP:
		fn = getExUDPTable
		class = udpTablePid
	default:
		panic("Unsupported network type for process lookup.")
	}
	ip := dest.Address.IP()
	port := int(dest.Port)

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return 0, "", "", errors.New("invalid IP address")
	}
	addr = addr.Unmap()

	family := windows.AF_INET
	if addr.Is6() {
		family = windows.AF_INET6
	}

	buf, err := getTransportTable(fn, family, class)
	if err != nil {
		return 0, "", "", err
	}

	s := newSearcher(dest.Network, dest.Address.Family())

	pid, err := s.Search(buf, addr, uint16(port))
	if err != nil {
		return 0, "", "", err
	}
	NameWithPath, err := getExecPathFromPID(pid)
	NameWithPath = filepath.ToSlash(NameWithPath)

	// drop .exe and path
	nameSplit := strings.Split(NameWithPath, "/")
	procName := nameSplit[len(nameSplit)-1]
	procName = strings.TrimSuffix(procName, ".exe")
	return int(pid), procName, NameWithPath, err
}

type searcher struct {
	itemSize int
	port     int
	ip       int
	ipSize   int
	pid      int
	tcpState int
}

func (s *searcher) Search(b []byte, ip netip.Addr, port uint16) (uint32, error) {
	n := int(readNativeUint32(b[:4]))
	itemSize := s.itemSize
	for i := range n {
		row := b[4+itemSize*i : 4+itemSize*(i+1)]

		if s.tcpState >= 0 {
			tcpState := readNativeUint32(row[s.tcpState : s.tcpState+4])
			// MIB_TCP_STATE_ESTAB, only check established connections for TCP
			if tcpState != 5 {
				continue
			}
		}

		// according to MSDN, only the lower 16 bits of dwLocalPort are used and the port number is in network endian.
		// this field can be illustrated as follows depends on different machine endianess:
		//     little endian: [ MSB LSB  0   0  ]   interpret as native uint32 is ((LSB<<8)|MSB)
		//       big  endian: [  0   0  MSB LSB ]   interpret as native uint32 is ((MSB<<8)|LSB)
		// so we need an syscall.Ntohs on the lower 16 bits after read the port as native uint32
		srcPort := syscall.Ntohs(uint16(readNativeUint32(row[s.port : s.port+4])))
		if srcPort != port {
			continue
		}

		srcIP, _ := netip.AddrFromSlice(row[s.ip : s.ip+s.ipSize])
		srcIP = srcIP.Unmap()
		// windows binds an unbound udp socket to 0.0.0.0/[::] while first sendto
		if ip != srcIP && (!srcIP.IsUnspecified() || s.tcpState != -1) {
			continue
		}

		pid := readNativeUint32(row[s.pid : s.pid+4])
		return pid, nil
	}
	return 0, errors.New("not found")
}

func newSearcher(network Network, family AddressFamily) *searcher {
	var itemSize, port, ip, ipSize, pid int
	tcpState := -1
	switch network {
	case Network_TCP:
		if family == AddressFamilyIPv4 {
			// struct MIB_TCPROW_OWNER_PID
			itemSize, port, ip, ipSize, pid, tcpState = 24, 8, 4, 4, 20, 0
		}
		if family == AddressFamilyIPv6 {
			// struct MIB_TCP6ROW_OWNER_PID
			itemSize, port, ip, ipSize, pid, tcpState = 56, 20, 0, 16, 52, 48
		}
	case Network_UDP:
		if family == AddressFamilyIPv4 {
			// struct MIB_UDPROW_OWNER_PID
			itemSize, port, ip, ipSize, pid = 12, 4, 0, 4, 8
		}
		if family == AddressFamilyIPv6 {
			// struct MIB_UDP6ROW_OWNER_PID
			itemSize, port, ip, ipSize, pid = 28, 20, 0, 16, 24
		}
	}

	return &searcher{
		itemSize: itemSize,
		port:     port,
		ip:       ip,
		ipSize:   ipSize,
		pid:      pid,
		tcpState: tcpState,
	}
}

func getTransportTable(fn uintptr, family int, class int) ([]byte, error) {
	for size, buf := uint32(8), make([]byte, 8); ; {
		ptr := unsafe.Pointer(&buf[0])
		err, _, _ := syscall.Syscall6(fn, 6, uintptr(ptr), uintptr(unsafe.Pointer(&size)), 0, uintptr(family), uintptr(class), 0)

		switch err {
		case 0:
			return buf, nil
		case uintptr(syscall.ERROR_INSUFFICIENT_BUFFER):
			buf = make([]byte, size)
		default:
			return nil, errors.New("syscall error: ", int(err))
		}
	}
}

func readNativeUint32(b []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[0]))
}

func getExecPathFromPID(pid uint32) (string, error) {
	// kernel process starts with a colon in order to distinguish with normal processes
	switch pid {
	case 0:
		// reserved pid for system idle process
		return ":System Idle Process", nil
	case 4:
		// reserved pid for windows kernel image
		return ":System", nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, syscall.MAX_LONG_PATH)
	size := uint32(len(buf))
	err = windows.QueryFullProcessImageName(h, 0, &buf[0], &size)
	if err != nil {
		return "", err
	}
	return syscall.UTF16ToString(buf[:size]), nil
}
