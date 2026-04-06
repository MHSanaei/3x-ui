package conf

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/xtls/xray-core/app/dns/fakedns"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/features/dns"
)

type FakeDNSPoolElementConfig struct {
	IPPool  string `json:"ipPool"`
	LRUSize int64  `json:"poolSize"`
}

type FakeDNSConfig struct {
	pool  *FakeDNSPoolElementConfig
	pools []*FakeDNSPoolElementConfig
}

// MarshalJSON implements encoding/json.Marshaler.MarshalJSON
func (f *FakeDNSConfig) MarshalJSON() ([]byte, error) {
	if (f.pool != nil) != (f.pools != nil) {
		if f.pool != nil {
			return json.Marshal(f.pool)
		} else if f.pools != nil {
			return json.Marshal(f.pools)
		}
	}
	return nil, errors.New("unexpected config state")
}

// UnmarshalJSON implements encoding/json.Unmarshaler.UnmarshalJSON
func (f *FakeDNSConfig) UnmarshalJSON(data []byte) error {
	var pool FakeDNSPoolElementConfig
	var pools []*FakeDNSPoolElementConfig
	switch {
	case json.Unmarshal(data, &pool) == nil:
		f.pool = &pool
	case json.Unmarshal(data, &pools) == nil:
		f.pools = pools
	default:
		return errors.New("invalid fakedns config")
	}
	return nil
}

func (f *FakeDNSConfig) Build() (*fakedns.FakeDnsPoolMulti, error) {
	fakeDNSPool := fakedns.FakeDnsPoolMulti{}

	if f.pool != nil {
		fakeDNSPool.Pools = append(fakeDNSPool.Pools, &fakedns.FakeDnsPool{
			IpPool:  f.pool.IPPool,
			LruSize: f.pool.LRUSize,
		})
		return &fakeDNSPool, nil
	}

	if f.pools != nil {
		for _, v := range f.pools {
			fakeDNSPool.Pools = append(fakeDNSPool.Pools, &fakedns.FakeDnsPool{IpPool: v.IPPool, LruSize: v.LRUSize})
		}
		return &fakeDNSPool, nil
	}

	return nil, errors.New("no valid FakeDNS config")
}

type FakeDNSPostProcessingStage struct{}

func (FakeDNSPostProcessingStage) Process(config *Config) error {
	fakeDNSInUse := false
	isIPv4Enable, isIPv6Enable := true, true

	if config.DNSConfig != nil {
		for _, v := range config.DNSConfig.Servers {
			if v.Address.Family().IsDomain() && strings.EqualFold(v.Address.Domain(), "fakedns") {
				fakeDNSInUse = true
			}
		}

		switch strings.ToLower(config.DNSConfig.QueryStrategy) {
		case "useip4", "useipv4", "use_ip4", "use_ipv4", "use_ip_v4", "use-ip4", "use-ipv4", "use-ip-v4":
			isIPv4Enable, isIPv6Enable = true, false
		case "useip6", "useipv6", "use_ip6", "use_ipv6", "use_ip_v6", "use-ip6", "use-ipv6", "use-ip-v6":
			isIPv4Enable, isIPv6Enable = false, true
		}
	}

	if fakeDNSInUse {
		// Add a Fake DNS Config if there is none
		if config.FakeDNS == nil {
			config.FakeDNS = &FakeDNSConfig{}
			switch {
			case isIPv4Enable && isIPv6Enable:
				config.FakeDNS.pools = []*FakeDNSPoolElementConfig{
					{
						IPPool:  dns.FakeIPv4Pool,
						LRUSize: 32768,
					},
					{
						IPPool:  dns.FakeIPv6Pool,
						LRUSize: 32768,
					},
				}
			case !isIPv4Enable && isIPv6Enable:
				config.FakeDNS.pool = &FakeDNSPoolElementConfig{
					IPPool:  dns.FakeIPv6Pool,
					LRUSize: 65535,
				}
			case isIPv4Enable && !isIPv6Enable:
				config.FakeDNS.pool = &FakeDNSPoolElementConfig{
					IPPool:  dns.FakeIPv4Pool,
					LRUSize: 65535,
				}
			}
		}

		found := false
		// Check if there is a Outbound with necessary sniffer on
		var inbounds []InboundDetourConfig

		if len(config.InboundConfigs) > 0 {
			inbounds = append(inbounds, config.InboundConfigs...)
		}
		for _, v := range inbounds {
			if v.SniffingConfig != nil && v.SniffingConfig.Enabled && v.SniffingConfig.DestOverride != nil {
				for _, dov := range *v.SniffingConfig.DestOverride {
					if strings.EqualFold(dov, "fakedns") || strings.EqualFold(dov, "fakedns+others") {
						found = true
						break
					}
				}
			}
		}
		if !found {
			errors.LogWarning(context.Background(), "Defined FakeDNS but haven't enabled FakeDNS destOverride at any inbound.")
		}
	}

	return nil
}
