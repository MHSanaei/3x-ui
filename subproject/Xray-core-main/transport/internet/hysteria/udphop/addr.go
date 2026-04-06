package udphop

import (
	"fmt"
	"net"
)

type InvalidPortError struct {
	PortStr string
}

func (e InvalidPortError) Error() string {
	return fmt.Sprintf("%s is not a valid port number or range", e.PortStr)
}

// UDPHopAddr contains an IP address and a list of ports.
type UDPHopAddr struct {
	IP      net.IP
	Ports   []uint32
	PortStr string
}

func (a *UDPHopAddr) Network() string {
	return "udphop"
}

func (a *UDPHopAddr) String() string {
	return net.JoinHostPort(a.IP.String(), a.PortStr)
}

// addrs returns a list of net.Addr's, one for each port.
func (a *UDPHopAddr) addrs() ([]net.Addr, error) {
	var addrs []net.Addr
	for _, port := range a.Ports {
		addr := &net.UDPAddr{
			IP:   a.IP,
			Port: int(port),
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

// func ResolveUDPHopAddr(addr string) (*UDPHopAddr, error) {
// 	host, portStr, err := net.SplitHostPort(addr)
// 	if err != nil {
// 		return nil, err
// 	}
// 	ip, err := net.ResolveIPAddr("ip", host)
// 	if err != nil {
// 		return nil, err
// 	}
// 	result := &UDPHopAddr{
// 		IP:      ip.IP,
// 		PortStr: portStr,
// 	}

// 	pu := utils.ParsePortUnion(portStr)
// 	if pu == nil {
// 		return nil, InvalidPortError{portStr}
// 	}
// 	result.Ports = pu.Ports()

// 	return result, nil
// }
