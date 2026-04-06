package dispatcher

import (
	"context"
	"strings"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/dns"
)

// newFakeDNSSniffer Creates a Fake DNS metadata sniffer
func newFakeDNSSniffer(ctx context.Context) (protocolSnifferWithMetadata, error) {
	var fakeDNSEngine dns.FakeDNSEngine
	{
		fakeDNSEngineFeat := core.MustFromContext(ctx).GetFeature((*dns.FakeDNSEngine)(nil))
		if fakeDNSEngineFeat != nil {
			fakeDNSEngine = fakeDNSEngineFeat.(dns.FakeDNSEngine)
		}
	}

	if fakeDNSEngine == nil {
		errNotInit := errors.New("FakeDNSEngine is not initialized, but such a sniffer is used").AtError()
		return protocolSnifferWithMetadata{}, errNotInit
	}
	return protocolSnifferWithMetadata{protocolSniffer: func(ctx context.Context, bytes []byte) (SniffResult, error) {
		outbounds := session.OutboundsFromContext(ctx)
		ob := outbounds[len(outbounds)-1]
		if ob.Target.Network == net.Network_TCP || ob.Target.Network == net.Network_UDP {
			domainFromFakeDNS := fakeDNSEngine.GetDomainFromFakeDNS(ob.Target.Address)
			if domainFromFakeDNS != "" {
				errors.LogInfo(ctx, "fake dns got domain: ", domainFromFakeDNS, " for ip: ", ob.Target.Address.String())
				return &fakeDNSSniffResult{domainName: domainFromFakeDNS}, nil
			}
		}

		if ipAddressInRangeValueI := ctx.Value(ipAddressInRange); ipAddressInRangeValueI != nil {
			ipAddressInRangeValue := ipAddressInRangeValueI.(*ipAddressInRangeOpt)
			if fkr0, ok := fakeDNSEngine.(dns.FakeDNSEngineRev0); ok {
				inPool := fkr0.IsIPInIPPool(ob.Target.Address)
				ipAddressInRangeValue.addressInRange = &inPool
			}
		}

		return nil, common.ErrNoClue
	}, metadataSniffer: true}, nil
}

type fakeDNSSniffResult struct {
	domainName string
}

func (fakeDNSSniffResult) Protocol() string {
	return "fakedns"
}

func (f fakeDNSSniffResult) Domain() string {
	return f.domainName
}

type fakeDNSExtraOpts int

const ipAddressInRange fakeDNSExtraOpts = 1

type ipAddressInRangeOpt struct {
	addressInRange *bool
}

type DNSThenOthersSniffResult struct {
	domainName           string
	protocolOriginalName string
}

func (f DNSThenOthersSniffResult) IsProtoSubsetOf(protocolName string) bool {
	return strings.HasPrefix(protocolName, f.protocolOriginalName)
}

func (DNSThenOthersSniffResult) Protocol() string {
	return "fakedns+others"
}

func (f DNSThenOthersSniffResult) Domain() string {
	return f.domainName
}

func newFakeDNSThenOthers(ctx context.Context, fakeDNSSniffer protocolSnifferWithMetadata, others []protocolSnifferWithMetadata) (
	protocolSnifferWithMetadata, error,
) { // nolint: unparam
	// ctx may be used in the future
	_ = ctx
	return protocolSnifferWithMetadata{
		protocolSniffer: func(ctx context.Context, bytes []byte) (SniffResult, error) {
			ipAddressInRangeValue := &ipAddressInRangeOpt{}
			ctx = context.WithValue(ctx, ipAddressInRange, ipAddressInRangeValue)
			result, err := fakeDNSSniffer.protocolSniffer(ctx, bytes)
			if err == nil {
				return result, nil
			}
			if ipAddressInRangeValue.addressInRange != nil {
				if *ipAddressInRangeValue.addressInRange {
					for _, v := range others {
						if v.metadataSniffer || bytes != nil {
							if result, err := v.protocolSniffer(ctx, bytes); err == nil {
								return DNSThenOthersSniffResult{domainName: result.Domain(), protocolOriginalName: result.Protocol()}, nil
							}
						}
					}
					return nil, common.ErrNoClue
				}
				errors.LogDebug(ctx, "ip address not in fake dns range, return as is")
				return nil, common.ErrNoClue
			}
			errors.LogWarning(ctx, "fake dns sniffer did not set address in range option, assume false.")
			return nil, common.ErrNoClue
		},
		metadataSniffer: false,
	}, nil
}
