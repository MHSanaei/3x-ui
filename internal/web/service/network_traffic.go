package service

import (
	"errors"
	"fmt"
	"math"
	stdnet "net"
	"slices"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/net"
)

var errNetworkUnavailable = errors.New("network traffic interface is unavailable")

type (
	networkTrafficConfig struct{ mode, iface string }
	networkDefaultRoute  struct {
		iface  string
		family int
		metric uint32
	}
)

type networkTrafficSample struct {
	source                       string
	sent, recv, pktSent, pktRecv uint64
	members                      map[string]net.IOCountersStat
}

func selectNetworkTraffic(config networkTrafficConfig, counters []net.IOCountersStat, interfaces []stdnet.Interface, automatic string) (networkTrafficSample, error) {
	selected := config.iface
	switch config.mode {
	case "legacy":
	case "auto":
		selected = automatic
		if selected == "" {
			return networkTrafficSample{}, errNetworkUnavailable
		}
	case "manual":
		if selected == "" {
			return networkTrafficSample{}, errNetworkUnavailable
		}
	default:
		return networkTrafficSample{}, fmt.Errorf("%w: invalid monitoring mode", errNetworkUnavailable)
	}
	byName := make(map[string]stdnet.Interface, len(interfaces))
	for _, iface := range interfaces {
		byName[iface.Name] = iface
	}
	if config.mode != "legacy" {
		if _, ok := byName[selected]; !ok {
			return networkTrafficSample{}, fmt.Errorf("%w: %s", errNetworkUnavailable, selected)
		}
	}
	sample := networkTrafficSample{members: make(map[string]net.IOCountersStat)}
	var identities []string
	for _, counter := range counters {
		if config.mode == "legacy" {
			if isVirtualInterface(strings.ToLower(counter.Name)) {
				continue
			}
		} else if counter.Name != selected {
			continue
		}
		iface := byName[counter.Name]
		identity := fmt.Sprintf("%q:%d:%s", counter.Name, iface.Index, iface.HardwareAddr)
		if _, exists := sample.members[identity]; exists {
			continue
		}
		sample.members[identity] = counter
		identities = append(identities, identity)
		if math.MaxUint64-sample.sent < counter.BytesSent || math.MaxUint64-sample.recv < counter.BytesRecv || math.MaxUint64-sample.pktSent < counter.PacketsSent || math.MaxUint64-sample.pktRecv < counter.PacketsRecv {
			return networkTrafficSample{}, fmt.Errorf("%w: counter overflow", errNetworkUnavailable)
		}
		sample.sent += counter.BytesSent
		sample.recv += counter.BytesRecv
		sample.pktSent += counter.PacketsSent
		sample.pktRecv += counter.PacketsRecv
	}
	if config.mode != "legacy" && len(sample.members) == 0 {
		return networkTrafficSample{}, fmt.Errorf("%w: no counters for %s", errNetworkUnavailable, selected)
	}
	slices.Sort(identities)
	sample.source = config.mode + ":" + strings.Join(identities, ";")
	return sample, nil
}

func chooseNetworkDefaultRoute(routes []networkDefaultRoute, interfaces []stdnet.Interface) (string, error) {
	up := make(map[string]bool, len(interfaces))
	for _, iface := range interfaces {
		up[iface.Name] = iface.Flags&stdnet.FlagUp != 0 && iface.Flags&stdnet.FlagLoopback == 0
	}
	candidates := slices.Clone(routes)
	candidates = slices.DeleteFunc(candidates, func(r networkDefaultRoute) bool { return !up[r.iface] || (r.family != 4 && r.family != 6) })
	slices.SortFunc(candidates, func(a, b networkDefaultRoute) int {
		if a.family != b.family {
			return a.family - b.family
		}
		if a.metric != b.metric {
			if a.metric < b.metric {
				return -1
			}
			return 1
		}
		return strings.Compare(a.iface, b.iface)
	})
	if len(candidates) == 0 {
		return "", fmt.Errorf("%w: no usable default route", errNetworkUnavailable)
	}
	return candidates[0].iface, nil
}

func networkTrafficRates(previous, current *networkTrafficSample, elapsed time.Duration) [4]uint64 {
	if previous == nil || current == nil || previous.source != current.source || elapsed <= 0 {
		return [4]uint64{}
	}
	for identity, counter := range current.members {
		before, exists := previous.members[identity]
		if !exists || counter.BytesSent < before.BytesSent || counter.BytesRecv < before.BytesRecv || counter.PacketsSent < before.PacketsSent || counter.PacketsRecv < before.PacketsRecv {
			return [4]uint64{}
		}
	}
	seconds := elapsed.Seconds()
	deltas := [4]uint64{current.sent - previous.sent, current.recv - previous.recv, current.pktSent - previous.pktSent, current.pktRecv - previous.pktRecv}
	var rates [4]uint64
	for i, delta := range deltas {
		rate := float64(delta) / seconds
		if rate >= float64(math.MaxUint64) {
			return [4]uint64{}
		}
		rates[i] = uint64(rate)
	}
	return rates
}
