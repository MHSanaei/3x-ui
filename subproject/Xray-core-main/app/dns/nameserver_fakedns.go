package dns

import (
	"context"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/features/dns"
)

type FakeDNSServer struct {
	fakeDNSEngine dns.FakeDNSEngine
}

func NewFakeDNSServer(fd dns.FakeDNSEngine) *FakeDNSServer {
	return &FakeDNSServer{fakeDNSEngine: fd}
}

func (FakeDNSServer) Name() string {
	return "FakeDNS"
}

// IsDisableCache implements Server.
func (s *FakeDNSServer) IsDisableCache() bool {
	return true
}

func (f *FakeDNSServer) QueryIP(ctx context.Context, domain string, opt dns.IPOption) ([]net.IP, uint32, error) {
	if f.fakeDNSEngine == nil {
		return nil, 0, errors.New("Unable to locate a fake DNS Engine").AtError()
	}

	var ips []net.Address
	if fkr0, ok := f.fakeDNSEngine.(dns.FakeDNSEngineRev0); ok {
		ips = fkr0.GetFakeIPForDomain3(domain, opt.IPv4Enable, opt.IPv6Enable)
	} else {
		ips = f.fakeDNSEngine.GetFakeIPForDomain(domain)
	}

	netIP, err := toNetIP(ips)
	if err != nil {
		return nil, 0, errors.New("Unable to convert IP to net ip").Base(err).AtError()
	}

	errors.LogInfo(ctx, f.Name(), " got answer: ", domain, " -> ", ips)

	if len(netIP) > 0 {
		return netIP, 1, nil // fakeIP ttl is 1
	}
	return nil, 0, dns.ErrEmptyResponse
}
