package router

import (
	"context"
	"net/netip"
	"sort"
	"strings"
	"sync"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"

	"go4.org/netipx"
)

type GeoIPMatcher interface {
	// TODO: (PERF) all net.IP -> netipx.Addr

	// Invalid IP always return false.
	Match(ip net.IP) bool

	// Returns true if *any* IP is valid and match.
	AnyMatch(ips []net.IP) bool

	// Returns true only if *all* IPs are valid and match. Any invalid IP, or non-matching valid IP, causes false.
	Matches(ips []net.IP) bool

	// Filters IPs. Invalid IPs are silently dropped and not included in either result.
	FilterIPs(ips []net.IP) (matched []net.IP, unmatched []net.IP)

	ToggleReverse()

	SetReverse(reverse bool)
}

type GeoIPSet struct {
	ipv4, ipv6 *netipx.IPSet
	max4, max6 uint8
}

type HeuristicGeoIPMatcher struct {
	ipset   *GeoIPSet
	reverse bool
}

type ipBucket struct {
	rep netip.Addr
	ips []net.IP
}

// Match implements GeoIPMatcher.
func (m *HeuristicGeoIPMatcher) Match(ip net.IP) bool {
	ipx, ok := netipx.FromStdIP(ip)
	if !ok {
		return false
	}
	return m.matchAddr(ipx)
}

func (m *HeuristicGeoIPMatcher) matchAddr(ipx netip.Addr) bool {
	if ipx.Is4() {
		return m.ipset.ipv4.Contains(ipx) != m.reverse
	}
	if ipx.Is6() {
		return m.ipset.ipv6.Contains(ipx) != m.reverse
	}
	return false
}

// AnyMatch implements GeoIPMatcher.
func (m *HeuristicGeoIPMatcher) AnyMatch(ips []net.IP) bool {
	n := len(ips)
	if n == 0 {
		return false
	}

	if n == 1 {
		return m.Match(ips[0])
	}

	heur4 := m.ipset.max4 <= 24
	heur6 := m.ipset.max6 <= 64
	if !heur4 && !heur6 {
		for _, ip := range ips {
			if ipx, ok := netipx.FromStdIP(ip); ok {
				if m.matchAddr(ipx) {
					return true
				}
			}
		}
		return false
	}

	buckets := make(map[[9]byte]struct{}, n)
	for _, ip := range ips {
		key, ok := prefixKeyFromIP(ip)
		if !ok {
			continue
		}
		heur := (key[0] == 4 && heur4) || (key[0] == 6 && heur6)
		if heur {
			if _, exists := buckets[key]; exists {
				continue
			}
		}
		ipx, ok := netipx.FromStdIP(ip)
		if !ok {
			continue
		}
		if m.matchAddr(ipx) {
			return true
		}
		if heur {
			buckets[key] = struct{}{}
		}
	}
	return false
}

// Matches implements GeoIPMatcher.
func (m *HeuristicGeoIPMatcher) Matches(ips []net.IP) bool {
	n := len(ips)
	if n == 0 {
		return false
	}

	if n == 1 {
		return m.Match(ips[0])
	}

	heur4 := m.ipset.max4 <= 24
	heur6 := m.ipset.max6 <= 64
	if !heur4 && !heur6 {
		for _, ip := range ips {
			ipx, ok := netipx.FromStdIP(ip)
			if !ok {
				return false
			}
			if !m.matchAddr(ipx) {
				return false
			}
		}
		return true
	}

	buckets := make(map[[9]byte]netip.Addr, n)
	precise := make([]netip.Addr, 0, n)

	for _, ip := range ips {
		key, ok := prefixKeyFromIP(ip)
		if !ok {
			return false
		}

		if (key[0] == 4 && heur4) || (key[0] == 6 && heur6) {
			if _, exists := buckets[key]; !exists {
				ipx, ok := netipx.FromStdIP(ip)
				if !ok {
					return false
				}
				buckets[key] = ipx
			}
		} else {
			ipx, ok := netipx.FromStdIP(ip)
			if !ok {
				return false
			}
			precise = append(precise, ipx)
		}
	}

	for _, ipx := range buckets {
		if !m.matchAddr(ipx) {
			return false
		}
	}
	for _, ipx := range precise {
		if !m.matchAddr(ipx) {
			return false
		}
	}
	return true
}

func prefixKeyFromIP(ip net.IP) (key [9]byte, ok bool) {
	if ip4 := ip.To4(); ip4 != nil {
		key[0] = 4
		key[1] = ip4[0]
		key[2] = ip4[1]
		key[3] = ip4[2] // /24
		return key, true
	}
	if ip16 := ip.To16(); ip16 != nil {
		key[0] = 6
		key[1] = ip16[0]
		key[2] = ip16[1]
		key[3] = ip16[2]
		key[4] = ip16[3]
		key[5] = ip16[4]
		key[6] = ip16[5]
		key[7] = ip16[6]
		key[8] = ip16[7] // /64
		return key, true
	}
	return key, false // illegal
}

// FilterIPs implements GeoIPMatcher.
func (m *HeuristicGeoIPMatcher) FilterIPs(ips []net.IP) (matched []net.IP, unmatched []net.IP) {
	n := len(ips)
	if n == 0 {
		return []net.IP{}, []net.IP{}
	}

	if n == 1 {
		ipx, ok := netipx.FromStdIP(ips[0])
		if !ok {
			return []net.IP{}, []net.IP{}
		}
		if m.matchAddr(ipx) {
			return ips, []net.IP{}
		}
		return []net.IP{}, ips
	}

	heur4 := m.ipset.max4 <= 24
	heur6 := m.ipset.max6 <= 64
	if !heur4 && !heur6 {
		matched = make([]net.IP, 0, n)
		unmatched = make([]net.IP, 0, n)
		for _, ip := range ips {
			ipx, ok := netipx.FromStdIP(ip)
			if !ok {
				continue // illegal ip, ignore
			}
			if m.matchAddr(ipx) {
				matched = append(matched, ip)
			} else {
				unmatched = append(unmatched, ip)
			}
		}
		return
	}

	buckets := make(map[[9]byte]*ipBucket, n)
	precise := make([]net.IP, 0, n)

	for _, ip := range ips {
		key, ok := prefixKeyFromIP(ip)
		if !ok {
			continue // illegal ip, ignore
		}

		if (key[0] == 4 && !heur4) || (key[0] == 6 && !heur6) {
			precise = append(precise, ip)
			continue
		}

		b, exists := buckets[key]
		if !exists {
			// build bucket
			ipx, ok := netipx.FromStdIP(ip)
			if !ok {
				continue // illegal ip, ignore
			}
			b = &ipBucket{
				rep: ipx,
				ips: make([]net.IP, 0, 4), // for dns answer
			}
			buckets[key] = b
		}
		b.ips = append(b.ips, ip)
	}

	matched = make([]net.IP, 0, n)
	unmatched = make([]net.IP, 0, n)
	for _, b := range buckets {
		if m.matchAddr(b.rep) {
			matched = append(matched, b.ips...)
		} else {
			unmatched = append(unmatched, b.ips...)
		}
	}
	for _, ip := range precise {
		ipx, ok := netipx.FromStdIP(ip)
		if !ok {
			continue // illegal ip, ignore
		}
		if m.matchAddr(ipx) {
			matched = append(matched, ip)
		} else {
			unmatched = append(unmatched, ip)
		}
	}
	return
}

// ToggleReverse implements GeoIPMatcher.
func (m *HeuristicGeoIPMatcher) ToggleReverse() {
	m.reverse = !m.reverse
}

// SetReverse implements GeoIPMatcher.
func (m *HeuristicGeoIPMatcher) SetReverse(reverse bool) {
	m.reverse = reverse
}

type GeneralMultiGeoIPMatcher struct {
	matchers []GeoIPMatcher
}

// Match implements GeoIPMatcher.
func (mm *GeneralMultiGeoIPMatcher) Match(ip net.IP) bool {
	for _, m := range mm.matchers {
		if m.Match(ip) {
			return true
		}
	}
	return false
}

// AnyMatch implements GeoIPMatcher.
func (mm *GeneralMultiGeoIPMatcher) AnyMatch(ips []net.IP) bool {
	for _, m := range mm.matchers {
		if m.AnyMatch(ips) {
			return true
		}
	}
	return false
}

// Matches implements GeoIPMatcher.
func (mm *GeneralMultiGeoIPMatcher) Matches(ips []net.IP) bool {
	for _, m := range mm.matchers {
		if m.Matches(ips) {
			return true
		}
	}
	return false
}

// FilterIPs implements GeoIPMatcher.
func (mm *GeneralMultiGeoIPMatcher) FilterIPs(ips []net.IP) (matched []net.IP, unmatched []net.IP) {
	matched = make([]net.IP, 0, len(ips))
	unmatched = ips
	for _, m := range mm.matchers {
		if len(unmatched) == 0 {
			break
		}
		var mtch []net.IP
		mtch, unmatched = m.FilterIPs(unmatched)
		if len(mtch) > 0 {
			matched = append(matched, mtch...)
		}
	}
	return
}

// ToggleReverse implements GeoIPMatcher.
func (mm *GeneralMultiGeoIPMatcher) ToggleReverse() {
	for _, m := range mm.matchers {
		m.ToggleReverse()
	}
}

// SetReverse implements GeoIPMatcher.
func (mm *GeneralMultiGeoIPMatcher) SetReverse(reverse bool) {
	for _, m := range mm.matchers {
		m.SetReverse(reverse)
	}
}

type HeuristicMultiGeoIPMatcher struct {
	matchers []*HeuristicGeoIPMatcher
}

// Match implements GeoIPMatcher.
func (mm *HeuristicMultiGeoIPMatcher) Match(ip net.IP) bool {
	ipx, ok := netipx.FromStdIP(ip)
	if !ok {
		return false
	}

	for _, m := range mm.matchers {
		if m.matchAddr(ipx) {
			return true
		}
	}
	return false
}

// AnyMatch implements GeoIPMatcher.
func (mm *HeuristicMultiGeoIPMatcher) AnyMatch(ips []net.IP) bool {
	n := len(ips)
	if n == 0 {
		return false
	}

	if n == 1 {
		return mm.Match(ips[0])
	}

	buckets := make(map[[9]byte]struct{}, n)
	for _, ip := range ips {
		var ipx netip.Addr
		state := uint8(0) // 0 = Not initialized, 1 = Initialized, 4 = IPv4 can be skipped, 6 = IPv6 can be skipped
		for _, m := range mm.matchers {
			heur4 := m.ipset.max4 <= 24
			heur6 := m.ipset.max6 <= 64

			if state == 0 && (heur4 || heur6) {
				key, ok := prefixKeyFromIP(ip)
				if !ok {
					break
				}
				if _, exists := buckets[key]; exists {
					state = key[0]
				} else {
					buckets[key] = struct{}{}
					state = 1
				}
			}
			if (heur4 && state == 4) || (heur6 && state == 6) {
				continue
			}

			if !ipx.IsValid() {
				nipx, ok := netipx.FromStdIP(ip)
				if !ok {
					break
				}
				ipx = nipx
			}
			if m.matchAddr(ipx) {
				return true
			}
		}
	}
	return false
}

// Matches implements GeoIPMatcher.
func (mm *HeuristicMultiGeoIPMatcher) Matches(ips []net.IP) bool {
	n := len(ips)
	if n == 0 {
		return false
	}

	if n == 1 {
		return mm.Match(ips[0])
	}

	var views ipViews
	for _, m := range mm.matchers {
		if !views.ensureForMatcher(m, ips) {
			return false
		}

		matched := true
		if m.ipset.max4 <= 24 {
			for _, ipx := range views.buckets4 {
				if !m.matchAddr(ipx) {
					matched = false
					break
				}
			}
		} else {
			for _, ipx := range views.precise4 {
				if !m.matchAddr(ipx) {
					matched = false
					break
				}
			}
		}
		if !matched {
			continue
		}

		if m.ipset.max6 <= 64 {
			for _, ipx := range views.buckets6 {
				if !m.matchAddr(ipx) {
					matched = false
					break
				}
			}
		} else {
			for _, ipx := range views.precise6 {
				if !m.matchAddr(ipx) {
					matched = false
					break
				}
			}
		}
		if matched {
			return true
		}
	}
	return false
}

type ipViews struct {
	buckets4, buckets6 map[[9]byte]netip.Addr
	precise4, precise6 []netip.Addr
}

func (v *ipViews) ensureForMatcher(m *HeuristicGeoIPMatcher, ips []net.IP) bool {
	needHeur4 := m.ipset.max4 <= 24 && v.buckets4 == nil
	needHeur6 := m.ipset.max6 <= 64 && v.buckets6 == nil
	needPrec4 := m.ipset.max4 > 24 && v.precise4 == nil
	needPrec6 := m.ipset.max6 > 64 && v.precise6 == nil

	if !needHeur4 && !needHeur6 && !needPrec4 && !needPrec6 {
		return true
	}

	if needHeur4 {
		v.buckets4 = make(map[[9]byte]netip.Addr, len(ips))
	}
	if needHeur6 {
		v.buckets6 = make(map[[9]byte]netip.Addr, len(ips))
	}
	if needPrec4 {
		v.precise4 = make([]netip.Addr, 0, len(ips))
	}
	if needPrec6 {
		v.precise6 = make([]netip.Addr, 0, len(ips))
	}

	for _, ip := range ips {
		key, ok := prefixKeyFromIP(ip)
		if !ok {
			return false
		}

		switch key[0] {
		case 4:
			var ipx netip.Addr
			if needHeur4 {
				if _, exists := v.buckets4[key]; !exists {
					ipx, ok = netipx.FromStdIP(ip)
					if !ok {
						return false
					}
					v.buckets4[key] = ipx
				}
			}
			if needPrec4 {
				if !ipx.IsValid() {
					ipx, ok = netipx.FromStdIP(ip)
					if !ok {
						return false
					}
				}
				v.precise4 = append(v.precise4, ipx)
			}
		case 6:
			var ipx netip.Addr
			if needHeur6 {
				if _, exists := v.buckets6[key]; !exists {
					ipx, ok = netipx.FromStdIP(ip)
					if !ok {
						return false
					}
					v.buckets6[key] = ipx
				}
			}
			if needPrec6 {
				if !ipx.IsValid() {
					ipx, ok = netipx.FromStdIP(ip)
					if !ok {
						return false
					}
				}
				v.precise6 = append(v.precise6, ipx)
			}
		default:
			return false
		}
	}

	return true
}

// FilterIPs implements GeoIPMatcher.
func (mm *HeuristicMultiGeoIPMatcher) FilterIPs(ips []net.IP) (matched []net.IP, unmatched []net.IP) {
	n := len(ips)
	if n == 0 {
		return []net.IP{}, []net.IP{}
	}

	if n == 1 {
		ipx, ok := netipx.FromStdIP(ips[0])
		if !ok {
			return []net.IP{}, []net.IP{}
		}
		for _, m := range mm.matchers {
			if m.matchAddr(ipx) {
				return ips, []net.IP{}
			}
		}
		return []net.IP{}, ips
	}

	var views ipBucketViews

	matched = make([]net.IP, 0, n)
	for _, m := range mm.matchers {
		views.ensureForMatcher(m, ips)

		if m.ipset.max4 <= 24 {
			for key, b := range views.buckets4 {
				if b == nil {
					continue
				}
				if m.matchAddr(b.rep) {
					views.buckets4[key] = nil
					matched = append(matched, b.ips...)
				}
			}
		} else {
			for ipx, ip := range views.precise4 {
				if ip == nil {
					continue
				}
				if m.matchAddr(ipx) {
					views.precise4[ipx] = nil
					matched = append(matched, ip)
				}
			}
		}

		if m.ipset.max6 <= 64 {
			for key, b := range views.buckets6 {
				if b == nil {
					continue
				}
				if m.matchAddr(b.rep) {
					views.buckets6[key] = nil
					matched = append(matched, b.ips...)
				}
			}
		} else {
			for ipx, ip := range views.precise6 {
				if ip == nil {
					continue
				}
				if m.matchAddr(ipx) {
					views.precise6[ipx] = nil
					matched = append(matched, ip)
				}
			}
		}
	}

	unmatched = make([]net.IP, 0, n-len(matched))
	if views.buckets4 != nil {
		for _, b := range views.buckets4 {
			if b == nil {
				continue
			}
			unmatched = append(unmatched, b.ips...)
		}
	}
	if views.precise4 != nil {
		for _, ip := range views.precise4 {
			if ip == nil {
				continue
			}
			unmatched = append(unmatched, ip)
		}
	}
	if views.buckets6 != nil {
		for _, b := range views.buckets6 {
			if b == nil {
				continue
			}
			unmatched = append(unmatched, b.ips...)
		}
	}
	if views.precise6 != nil {
		for _, ip := range views.precise6 {
			if ip == nil {
				continue
			}
			unmatched = append(unmatched, ip)
		}
	}

	return
}

type ipBucketViews struct {
	buckets4, buckets6 map[[9]byte]*ipBucket
	precise4, precise6 map[netip.Addr]net.IP
}

func (v *ipBucketViews) ensureForMatcher(m *HeuristicGeoIPMatcher, ips []net.IP) {
	needHeur4 := m.ipset.max4 <= 24 && v.buckets4 == nil
	needHeur6 := m.ipset.max6 <= 64 && v.buckets6 == nil
	needPrec4 := m.ipset.max4 > 24 && v.precise4 == nil
	needPrec6 := m.ipset.max6 > 64 && v.precise6 == nil

	if !needHeur4 && !needHeur6 && !needPrec4 && !needPrec6 {
		return
	}

	if needHeur4 {
		v.buckets4 = make(map[[9]byte]*ipBucket, len(ips))
	}
	if needHeur6 {
		v.buckets6 = make(map[[9]byte]*ipBucket, len(ips))
	}
	if needPrec4 {
		v.precise4 = make(map[netip.Addr]net.IP, len(ips))
	}
	if needPrec6 {
		v.precise6 = make(map[netip.Addr]net.IP, len(ips))
	}

	for _, ip := range ips {
		key, ok := prefixKeyFromIP(ip)
		if !ok {
			continue // illegal ip, ignore
		}

		switch key[0] {
		case 4:
			var ipx netip.Addr
			if needHeur4 {
				b, exists := v.buckets4[key]
				if !exists {
					// build bucket
					ipx, ok = netipx.FromStdIP(ip)
					if !ok {
						continue // illegal ip, ignore
					}
					b = &ipBucket{
						rep: ipx,
						ips: make([]net.IP, 0, 4), // for dns answer
					}
					v.buckets4[key] = b
				}
				b.ips = append(b.ips, ip)
			}
			if needPrec4 {
				if !ipx.IsValid() {
					ipx, ok = netipx.FromStdIP(ip)
					if !ok {
						continue // illegal ip, ignore
					}
				}
				v.precise4[ipx] = ip
			}
		case 6:
			var ipx netip.Addr
			if needHeur6 {
				b, exists := v.buckets6[key]
				if !exists {
					// build bucket
					ipx, ok = netipx.FromStdIP(ip)
					if !ok {
						continue // illegal ip, ignore
					}
					b = &ipBucket{
						rep: ipx,
						ips: make([]net.IP, 0, 4), // for dns answer
					}
					v.buckets6[key] = b
				}
				b.ips = append(b.ips, ip)
			}
			if needPrec6 {
				if !ipx.IsValid() {
					ipx, ok = netipx.FromStdIP(ip)
					if !ok {
						continue // illegal ip, ignore
					}
				}
				v.precise6[ipx] = ip
			}
		}
	}
}

// ToggleReverse implements GeoIPMatcher.
func (mm *HeuristicMultiGeoIPMatcher) ToggleReverse() {
	for _, m := range mm.matchers {
		m.ToggleReverse()
	}
}

// SetReverse implements GeoIPMatcher.
func (mm *HeuristicMultiGeoIPMatcher) SetReverse(reverse bool) {
	for _, m := range mm.matchers {
		m.SetReverse(reverse)
	}
}

type GeoIPSetFactory struct {
	sync.Mutex
	shared map[string]*GeoIPSet // TODO: cleanup
}

var ipsetFactory = GeoIPSetFactory{shared: make(map[string]*GeoIPSet)}

func (f *GeoIPSetFactory) GetOrCreate(key string, cidrGroups [][]*CIDR) (*GeoIPSet, error) {
	f.Lock()
	defer f.Unlock()

	if ipset := f.shared[key]; ipset != nil {
		return ipset, nil
	}

	ipset, err := f.Create(cidrGroups...)
	if err == nil {
		f.shared[key] = ipset
	}
	return ipset, err
}

func (f *GeoIPSetFactory) Create(cidrGroups ...[]*CIDR) (*GeoIPSet, error) {
	var ipv4Builder, ipv6Builder netipx.IPSetBuilder

	for _, cidrGroup := range cidrGroups {
		for i, cidrEntry := range cidrGroup {
			cidrGroup[i] = nil
			ipBytes := cidrEntry.GetIp()
			prefixLen := int(cidrEntry.GetPrefix())

			addr, ok := netip.AddrFromSlice(ipBytes)
			if !ok {
				errors.LogError(context.Background(), "ignore invalid IP byte slice: ", ipBytes)
				continue
			}

			prefix := netip.PrefixFrom(addr, prefixLen)
			if !prefix.IsValid() {
				errors.LogError(context.Background(), "ignore created invalid prefix from addr ", addr, " and length ", prefixLen)
				continue
			}

			if addr.Is4() {
				ipv4Builder.AddPrefix(prefix)
			} else if addr.Is6() {
				ipv6Builder.AddPrefix(prefix)
			}
		}
	}

	ipv4, err := ipv4Builder.IPSet()
	if err != nil {
		return nil, errors.New("failed to build IPv4 set").Base(err)
	}
	ipv6, err := ipv6Builder.IPSet()
	if err != nil {
		return nil, errors.New("failed to build IPv6 set").Base(err)
	}

	var max4, max6 int

	for _, p := range ipv4.Prefixes() {
		if b := p.Bits(); b > max4 {
			max4 = b
		}
	}
	for _, p := range ipv6.Prefixes() {
		if b := p.Bits(); b > max6 {
			max6 = b
		}
	}

	if max4 == 0 {
		max4 = 0xff
	}
	if max6 == 0 {
		max6 = 0xff
	}

	return &GeoIPSet{ipv4: ipv4, ipv6: ipv6, max4: uint8(max4), max6: uint8(max6)}, nil
}

func BuildOptimizedGeoIPMatcher(geoips ...*GeoIP) (GeoIPMatcher, error) {
	n := len(geoips)
	if n == 0 {
		return nil, errors.New("no geoip configs provided")
	}

	var subs []*HeuristicGeoIPMatcher
	pos := make([]*GeoIP, 0, n)
	neg := make([]*GeoIP, 0, n/2)

	for _, geoip := range geoips {
		if geoip == nil {
			return nil, errors.New("geoip entry is nil")
		}
		if geoip.CountryCode == "" {
			ipset, err := ipsetFactory.Create(geoip.Cidr)
			if err != nil {
				return nil, err
			}
			subs = append(subs, &HeuristicGeoIPMatcher{ipset: ipset, reverse: geoip.ReverseMatch})
			continue
		}
		if !geoip.ReverseMatch {
			pos = append(pos, geoip)
		} else {
			neg = append(neg, geoip)
		}
	}

	buildIPSet := func(mergeables []*GeoIP) (*GeoIPSet, error) {
		n := len(mergeables)
		if n == 0 {
			return nil, nil
		}

		sort.Slice(mergeables, func(i, j int) bool {
			gi, gj := mergeables[i], mergeables[j]
			return gi.CountryCode < gj.CountryCode
		})

		var sb strings.Builder
		sb.Grow(n * 3) // xx,
		cidrGroups := make([][]*CIDR, 0, n)
		var last *GeoIP
		for i, geoip := range mergeables {
			if i == 0 || (geoip.CountryCode != last.CountryCode) {
				last = geoip
				sb.WriteString(geoip.CountryCode)
				sb.WriteString(",")
				cidrGroups = append(cidrGroups, geoip.Cidr)
			}
		}

		return ipsetFactory.GetOrCreate(sb.String(), cidrGroups)
	}

	ipset, err := buildIPSet(pos)
	if err != nil {
		return nil, err
	}
	if ipset != nil {
		subs = append(subs, &HeuristicGeoIPMatcher{ipset: ipset, reverse: false})
	}

	ipset, err = buildIPSet(neg)
	if err != nil {
		return nil, err
	}
	if ipset != nil {
		subs = append(subs, &HeuristicGeoIPMatcher{ipset: ipset, reverse: true})
	}

	switch len(subs) {
	case 0:
		return nil, errors.New("no valid geoip matcher")
	case 1:
		return subs[0], nil
	default:
		return &HeuristicMultiGeoIPMatcher{matchers: subs}, nil
	}
}
