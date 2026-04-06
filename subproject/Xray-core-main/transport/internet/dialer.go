package internet

import (
	"context"
	"fmt"
	"strings"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/dice"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/net/cnc"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/features/outbound"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/pipe"
)

// Dialer is the interface for dialing outbound connections.
type Dialer interface {
	// Dial dials a system connection to the given destination.
	Dial(ctx context.Context, destination net.Destination) (stat.Connection, error)

	// DestIpAddress returns the ip of proxy server. It is useful in case of Android client, which prepare an IP before proxy connection is established
	DestIpAddress() net.IP

	// SetOutboundGateway set outbound gateway
	SetOutboundGateway(ctx context.Context, ob *session.Outbound)
}

// dialFunc is an interface to dial network connection to a specific destination.
type dialFunc func(ctx context.Context, dest net.Destination, streamSettings *MemoryStreamConfig) (stat.Connection, error)

var transportDialerCache = make(map[string]dialFunc)

// RegisterTransportDialer registers a Dialer with given name.
func RegisterTransportDialer(protocol string, dialer dialFunc) error {
	if _, found := transportDialerCache[protocol]; found {
		return errors.New(protocol, " dialer already registered").AtError()
	}
	transportDialerCache[protocol] = dialer
	return nil
}

// Dial dials a internet connection towards the given destination.
func Dial(ctx context.Context, dest net.Destination, streamSettings *MemoryStreamConfig) (stat.Connection, error) {
	if dest.Network == net.Network_TCP {
		if streamSettings == nil {
			s, err := ToMemoryStreamConfig(nil)
			if err != nil {
				return nil, errors.New("failed to create default stream settings").Base(err)
			}
			streamSettings = s
		}

		protocol := streamSettings.ProtocolName
		dialer := transportDialerCache[protocol]
		if dialer == nil {
			return nil, errors.New(protocol, " dialer not registered").AtError()
		}
		return dialer(ctx, dest, streamSettings)
	}

	if dest.Network == net.Network_UDP {
		udpDialer := transportDialerCache["udp"]
		if udpDialer == nil {
			return nil, errors.New("UDP dialer not registered").AtError()
		}
		return udpDialer(ctx, dest, streamSettings)
	}

	return nil, errors.New("unknown network ", dest.Network)
}

// DestIpAddress returns the ip of proxy server. It is useful in case of Android client, which prepare an IP before proxy connection is established
func DestIpAddress() net.IP {
	return effectiveSystemDialer.DestIpAddress()
}

var (
	dnsClient dns.Client
	obm       outbound.Manager
)

func LookupForIP(domain string, strategy DomainStrategy, localAddr net.Address) ([]net.IP, error) {
	if dnsClient == nil {
		return nil, errors.New("DNS client not initialized").AtError()
	}

	ips, _, err := dnsClient.LookupIP(domain, dns.IPOption{
		IPv4Enable: (localAddr == nil && strategy.PreferIP4()) || (localAddr != nil && localAddr.Family().IsIPv4() && (strategy.PreferIP4() || strategy.FallbackIP4())),
		IPv6Enable: (localAddr == nil && strategy.PreferIP6()) || (localAddr != nil && localAddr.Family().IsIPv6() && (strategy.PreferIP6() || strategy.FallbackIP6())),
	})
	{ // Resolve fallback
		if (len(ips) == 0 || err != nil) && strategy.HasFallback() && localAddr == nil {
			ips, _, err = dnsClient.LookupIP(domain, dns.IPOption{
				IPv4Enable: strategy.FallbackIP4(),
				IPv6Enable: strategy.FallbackIP6(),
			})
		}
	}

	if err == nil && len(ips) == 0 {
		return nil, dns.ErrEmptyResponse
	}
	return ips, err
}

func redirect(ctx context.Context, dst net.Destination, obt string, h outbound.Handler) net.Conn {
	errors.LogInfo(ctx, "redirecting request "+dst.String()+" to "+obt)
	outbounds := session.OutboundsFromContext(ctx)
	ctx = session.ContextWithOutbounds(ctx, append(outbounds, &session.Outbound{
		Target:  dst,
		Gateway: nil,
		Tag:     obt,
	})) // add another outbound in session ctx

	ur, uw := pipe.New(pipe.OptionsFromContext(ctx)...)
	dr, dw := pipe.New(pipe.OptionsFromContext(ctx)...)

	go h.Dispatch(context.WithoutCancel(ctx), &transport.Link{Reader: ur, Writer: dw})
	var readerOpt cnc.ConnectionOption
	if dst.Network == net.Network_TCP {
		readerOpt = cnc.ConnectionOutputMulti(dr)
	} else {
		readerOpt = cnc.ConnectionOutputMultiUDP(dr)
	}
	nc := cnc.NewConnection(
		cnc.ConnectionInputMulti(uw),
		readerOpt,
		cnc.ConnectionOnClose(common.ChainedClosable{uw, dw}),
	)
	return nc

}

func checkAddressPortStrategy(ctx context.Context, dest net.Destination, sockopt *SocketConfig) (*net.Destination, error) {
	if sockopt.AddressPortStrategy == AddressPortStrategy_None {
		return nil, nil
	}
	newDest := dest
	var OverridePort, OverrideAddress bool
	var OverrideBy string
	switch sockopt.AddressPortStrategy {
	case AddressPortStrategy_SrvPortOnly:
		OverridePort = true
		OverrideAddress = false
		OverrideBy = "srv"
	case AddressPortStrategy_SrvAddressOnly:
		OverridePort = false
		OverrideAddress = true
		OverrideBy = "srv"
	case AddressPortStrategy_SrvPortAndAddress:
		OverridePort = true
		OverrideAddress = true
		OverrideBy = "srv"
	case AddressPortStrategy_TxtPortOnly:
		OverridePort = true
		OverrideAddress = false
		OverrideBy = "txt"
	case AddressPortStrategy_TxtAddressOnly:
		OverridePort = false
		OverrideAddress = true
		OverrideBy = "txt"
	case AddressPortStrategy_TxtPortAndAddress:
		OverridePort = true
		OverrideAddress = true
		OverrideBy = "txt"
	default:
		return nil, errors.New("unknown AddressPortStrategy")
	}

	if !dest.Address.Family().IsDomain() {
		return nil, nil
	}

	if OverrideBy == "srv" {
		errors.LogDebug(ctx, "query SRV record for "+dest.Address.String())
		parts := strings.SplitN(dest.Address.String(), ".", 3)
		if len(parts) != 3 {
			return nil, errors.New("invalid address format", dest.Address.String())
		}
		_, srvRecords, err := net.DefaultResolver.LookupSRV(context.Background(), parts[0][1:], parts[1][1:], parts[2])
		if err != nil {
			return nil, errors.New("failed to lookup SRV record").Base(err)
		}
		errors.LogDebug(ctx, "SRV record: "+fmt.Sprintf("addr=%s, port=%d, priority=%d, weight=%d", srvRecords[0].Target, srvRecords[0].Port, srvRecords[0].Priority, srvRecords[0].Weight))
		if OverridePort {
			newDest.Port = net.Port(srvRecords[0].Port)
		}
		if OverrideAddress {
			newDest.Address = net.ParseAddress(srvRecords[0].Target)
		}
		return &newDest, nil
	}
	if OverrideBy == "txt" {
		errors.LogDebug(ctx, "query TXT record for "+dest.Address.String())
		txtRecords, err := net.DefaultResolver.LookupTXT(ctx, dest.Address.String())
		if err != nil {
			errors.LogError(ctx, "failed to lookup SRV record: "+err.Error())
			return nil, errors.New("failed to lookup SRV record").Base(err)
		}
		for _, txtRecord := range txtRecords {
			errors.LogDebug(ctx, "TXT record: "+txtRecord)
			addr_s, port_s, _ := net.SplitHostPort(string(txtRecord))
			addr := net.ParseAddress(addr_s)
			port, err := net.PortFromString(port_s)
			if err != nil {
				continue
			}

			if OverridePort {
				newDest.Port = port
			}
			if OverrideAddress {
				newDest.Address = addr
			}
			return &newDest, nil
		}
	}
	return nil, nil
}

// DialSystem calls system dialer to create a network connection.
func DialSystem(ctx context.Context, dest net.Destination, sockopt *SocketConfig) (net.Conn, error) {
	var src net.Address
	outbounds := session.OutboundsFromContext(ctx)
	var outboundName string
	var origTargetAddr net.Address
	if len(outbounds) > 0 {
		ob := outbounds[len(outbounds)-1]
		if sockopt == nil || len(sockopt.DialerProxy) == 0 {
			src = ob.Gateway
		}
		outboundName = ob.Name
		origTargetAddr = ob.OriginalTarget.Address
		if origTargetAddr == nil {
			origTargetAddr = ob.Target.Address
		}
	}
	if sockopt == nil {
		return effectiveSystemDialer.Dial(ctx, src, dest, sockopt)
	}

	if newDest, err := checkAddressPortStrategy(ctx, dest, sockopt); err == nil && newDest != nil {
		errors.LogInfo(ctx, "replace destination with "+newDest.String())
		dest = *newDest
	}

	if sockopt.DomainStrategy.HasStrategy() && dest.Address.Family().IsDomain() {
		finalStrategy := sockopt.DomainStrategy
		if outboundName == "freedom" && dest.Network == net.Network_UDP && origTargetAddr != nil && src == nil {
			finalStrategy = finalStrategy.GetDynamicStrategy(origTargetAddr.Family())
		}
		ips, err := LookupForIP(dest.Address.Domain(), finalStrategy, src)
		if err != nil {
			errors.LogErrorInner(ctx, err, "failed to resolve ip")
			if sockopt.DomainStrategy.ForceIP() {
				return nil, err
			}
		} else if sockopt.HappyEyeballs == nil || sockopt.HappyEyeballs.TryDelayMs == 0 || sockopt.HappyEyeballs.MaxConcurrentTry == 0 || len(ips) < 2 || len(sockopt.DialerProxy) > 0 || dest.Network != net.Network_TCP {
			dest.Address = net.IPAddress(ips[dice.Roll(len(ips))])
			errors.LogInfo(ctx, "replace destination with "+dest.String())
		} else {
			return TcpRaceDial(ctx, src, ips, dest.Port, sockopt, dest.Address.String())
		}
	}

	if len(sockopt.DialerProxy) > 0 {
		if obm == nil {
			return nil, errors.New("there is no outbound manager for dialerProxy").AtError()
		}
		h := obm.GetHandler(sockopt.DialerProxy)
		if h == nil {
			return nil, errors.New("there is no outbound handler for dialerProxy").AtError()
		}
		return redirect(ctx, dest, sockopt.DialerProxy, h), nil
	}

	return effectiveSystemDialer.Dial(ctx, src, dest, sockopt)
}

func InitSystemDialer(dc dns.Client, om outbound.Manager) {
	dnsClient = dc
	obm = om
}
