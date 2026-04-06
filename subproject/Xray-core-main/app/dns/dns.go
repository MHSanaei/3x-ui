// Package dns is an implementation of core.DNS feature.
package dns

import (
	"context"
	go_errors "errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/platform"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/strmatcher"
	"github.com/xtls/xray-core/features/dns"
)

// DNS is a DNS rely server.
type DNS struct {
	sync.Mutex
	disableFallback        bool
	disableFallbackIfMatch bool
	enableParallelQuery    bool
	ipOption               *dns.IPOption
	hosts                  *StaticHosts
	clients                []*Client
	ctx                    context.Context
	domainMatcher          strmatcher.IndexMatcher
	matcherInfos           []*DomainMatcherInfo
	checkSystem            bool
}

// DomainMatcherInfo contains information attached to index returned by Server.domainMatcher
type DomainMatcherInfo struct {
	clientIdx     uint16
	domainRuleIdx uint16
}

// New creates a new DNS server with given configuration.
func New(ctx context.Context, config *Config) (*DNS, error) {
	var clientIP net.IP
	switch len(config.ClientIp) {
	case 0, net.IPv4len, net.IPv6len:
		clientIP = net.IP(config.ClientIp)
	default:
		return nil, errors.New("unexpected client IP length ", len(config.ClientIp))
	}

	var ipOption dns.IPOption
	checkSystem := false
	switch config.QueryStrategy {
	case QueryStrategy_USE_IP:
		ipOption = dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		}
	case QueryStrategy_USE_SYS:
		ipOption = dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: true,
			FakeEnable: false,
		}
		checkSystem = true
	case QueryStrategy_USE_IP4:
		ipOption = dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: false,
			FakeEnable: false,
		}
	case QueryStrategy_USE_IP6:
		ipOption = dns.IPOption{
			IPv4Enable: false,
			IPv6Enable: true,
			FakeEnable: false,
		}
	default:
		return nil, errors.New("unexpected query strategy ", config.QueryStrategy)
	}

	var hosts *StaticHosts
	mphLoaded := false
	domainMatcherPath := platform.NewEnvFlag(platform.MphCachePath).GetValue(func() string { return "" })
	if domainMatcherPath != "" {
		if f, err := os.Open(domainMatcherPath); err == nil {
			defer f.Close()
			if m, err := router.LoadGeoSiteMatcher(f, "HOSTS"); err == nil {
				f.Seek(0, 0)
				if hostIPs, err := router.LoadGeoSiteHosts(f); err == nil {
					if sh, err := NewStaticHostsFromCache(m, hostIPs); err == nil {
						hosts = sh
						mphLoaded = true
						errors.LogDebug(ctx, "MphDomainMatcher loaded from cache for DNS hosts, size: ", sh.matchers.Size())
					}
				}
			}
		}
	}

	if !mphLoaded {
		sh, err := NewStaticHosts(config.StaticHosts)
		if err != nil {
			return nil, errors.New("failed to create hosts").Base(err)
		}
		hosts = sh
	}

	var clients []*Client
	domainRuleCount := 0

	var defaultTag = config.Tag
	if len(config.Tag) == 0 {
		defaultTag = generateRandomTag()
	}

	for _, ns := range config.NameServer {
		domainRuleCount += len(ns.PrioritizedDomain)
	}

	// MatcherInfos is ensured to cover the maximum index domainMatcher could return, where matcher's index starts from 1
	matcherInfos := make([]*DomainMatcherInfo, domainRuleCount+1)
	domainMatcher := &strmatcher.MatcherGroup{}

	for _, ns := range config.NameServer {
		clientIdx := len(clients)
		updateDomain := func(domainRule strmatcher.Matcher, originalRuleIdx int, matcherInfos []*DomainMatcherInfo) {
			midx := domainMatcher.Add(domainRule)
			matcherInfos[midx] = &DomainMatcherInfo{
				clientIdx:     uint16(clientIdx),
				domainRuleIdx: uint16(originalRuleIdx),
			}
		}

		myClientIP := clientIP
		switch len(ns.ClientIp) {
		case net.IPv4len, net.IPv6len:
			myClientIP = net.IP(ns.ClientIp)
		}

		disableCache := config.DisableCache
		if ns.DisableCache != nil {
			disableCache = *ns.DisableCache
		}

		serveStale := config.ServeStale
		if ns.ServeStale != nil {
			serveStale = *ns.ServeStale
		}

		serveExpiredTTL := config.ServeExpiredTTL
		if ns.ServeExpiredTTL != nil {
			serveExpiredTTL = *ns.ServeExpiredTTL
		}

		var tag = defaultTag
		if len(ns.Tag) > 0 {
			tag = ns.Tag
		}
		clientIPOption := ResolveIpOptionOverride(ns.QueryStrategy, ipOption)
		if !clientIPOption.IPv4Enable && !clientIPOption.IPv6Enable {
			return nil, errors.New("no QueryStrategy available for ", ns.Address)
		}

		client, err := NewClient(ctx, ns, myClientIP, disableCache, serveStale, serveExpiredTTL, tag, clientIPOption, &matcherInfos, updateDomain)
		if err != nil {
			return nil, errors.New("failed to create client").Base(err)
		}
		clients = append(clients, client)
	}

	// If there is no DNS client in config, add a `localhost` DNS client
	if len(clients) == 0 {
		clients = append(clients, NewLocalDNSClient(ipOption))
	}

	return &DNS{
		hosts:                  hosts,
		ipOption:               &ipOption,
		clients:                clients,
		ctx:                    ctx,
		domainMatcher:          domainMatcher,
		matcherInfos:           matcherInfos,
		disableFallback:        config.DisableFallback,
		disableFallbackIfMatch: config.DisableFallbackIfMatch,
		enableParallelQuery:    config.EnableParallelQuery,
		checkSystem:            checkSystem,
	}, nil
}

// Type implements common.HasType.
func (*DNS) Type() interface{} {
	return dns.ClientType()
}

// Start implements common.Runnable.
func (s *DNS) Start() error {
	return nil
}

// Close implements common.Closable.
func (s *DNS) Close() error {
	return nil
}

// IsOwnLink implements proxy.dns.ownLinkVerifier
func (s *DNS) IsOwnLink(ctx context.Context) bool {
	inbound := session.InboundFromContext(ctx)
	if inbound == nil {
		return false
	}
	for _, client := range s.clients {
		if client.tag == inbound.Tag {
			return true
		}
	}
	return false
}

// LookupIP implements dns.Client.
func (s *DNS) LookupIP(domain string, option dns.IPOption) ([]net.IP, uint32, error) {
	// Normalize the FQDN form query
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		return nil, 0, errors.New("empty domain name")
	}

	if s.checkSystem {
		supportIPv4, supportIPv6 := checkRoutes()
		option.IPv4Enable = option.IPv4Enable && supportIPv4
		option.IPv6Enable = option.IPv6Enable && supportIPv6
	} else {
		option.IPv4Enable = option.IPv4Enable && s.ipOption.IPv4Enable
		option.IPv6Enable = option.IPv6Enable && s.ipOption.IPv6Enable
	}

	if !option.IPv4Enable && !option.IPv6Enable {
		return nil, 0, dns.ErrEmptyResponse
	}

	// Static host lookup
	switch addrs, err := s.hosts.Lookup(domain, option); {
	case err != nil:
		if go_errors.Is(err, dns.ErrEmptyResponse) {
			return nil, 0, dns.ErrEmptyResponse
		}
		return nil, 0, errors.New("returning nil for domain ", domain).Base(err)
	case addrs == nil: // Domain not recorded in static host
		break
	case len(addrs) == 0: // Domain recorded, but no valid IP returned (e.g. IPv4 address with only IPv6 enabled)
		return nil, 0, dns.ErrEmptyResponse
	case len(addrs) == 1 && addrs[0].Family().IsDomain(): // Domain replacement
		errors.LogInfo(s.ctx, "domain replaced: ", domain, " -> ", addrs[0].Domain())
		domain = addrs[0].Domain()
	default: // Successfully found ip records in static host
		errors.LogInfo(s.ctx, "returning ", len(addrs), " IP(s) for domain ", domain, " -> ", addrs)
		ips, err := toNetIP(addrs)
		if err != nil {
			return nil, 0, err
		}
		return ips, 10, nil // Hosts ttl is 10
	}

	// Name servers lookup
	if s.enableParallelQuery {
		return s.parallelQuery(domain, option)
	} else {
		return s.serialQuery(domain, option)
	}
}

func (s *DNS) sortClients(domain string) []*Client {
	clients := make([]*Client, 0, len(s.clients))
	clientUsed := make([]bool, len(s.clients))
	clientNames := make([]string, 0, len(s.clients))
	domainRules := []string{}

	// Priority domain matching
	hasMatch := false
	MatchSlice := s.domainMatcher.Match(domain)
	sort.Slice(MatchSlice, func(i, j int) bool {
		return MatchSlice[i] < MatchSlice[j]
	})
	for _, match := range MatchSlice {
		info := s.matcherInfos[match]
		client := s.clients[info.clientIdx]
		domainRule := client.domains[info.domainRuleIdx]
		domainRules = append(domainRules, fmt.Sprintf("%s(DNS idx:%d)", domainRule, info.clientIdx))
		if clientUsed[info.clientIdx] {
			continue
		}
		clientUsed[info.clientIdx] = true
		clients = append(clients, client)
		clientNames = append(clientNames, client.Name())
		hasMatch = true
		if client.finalQuery {
			return clients
		}
	}

	if !(s.disableFallback || s.disableFallbackIfMatch && hasMatch) {
		// Default round-robin query
		for idx, client := range s.clients {
			if clientUsed[idx] || client.skipFallback {
				continue
			}
			clientUsed[idx] = true
			clients = append(clients, client)
			clientNames = append(clientNames, client.Name())
			if client.finalQuery {
				return clients
			}
		}
	}

	if len(domainRules) > 0 {
		errors.LogDebug(s.ctx, "domain ", domain, " matches following rules: ", domainRules)
	}
	if len(clientNames) > 0 {
		errors.LogDebug(s.ctx, "domain ", domain, " will use DNS in order: ", clientNames)
	}

	if len(clients) == 0 {
		if len(s.clients) > 0 {
			clients = append(clients, s.clients[0])
			clientNames = append(clientNames, s.clients[0].Name())
			errors.LogWarning(s.ctx, "domain ", domain, " will use the first DNS: ", clientNames)
		} else {
			errors.LogError(s.ctx, "no DNS clients available for domain ", domain, " and no default clients configured")
		}
	}

	return clients
}

func mergeQueryErrors(domain string, errs []error) error {
	if len(errs) == 0 {
		return dns.ErrEmptyResponse
	}

	var noRNF error
	for _, err := range errs {
		if go_errors.Is(err, errRecordNotFound) {
			continue // server no response, ignore
		} else if noRNF == nil {
			noRNF = err
		} else if !go_errors.Is(err, noRNF) {
			return errors.New("returning nil for domain ", domain).Base(errors.Combine(errs...))
		}
	}
	if go_errors.Is(noRNF, dns.ErrEmptyResponse) {
		return dns.ErrEmptyResponse
	}
	if noRNF == nil {
		noRNF = errRecordNotFound
	}
	return errors.New("returning nil for domain ", domain).Base(noRNF)
}

func (s *DNS) serialQuery(domain string, option dns.IPOption) ([]net.IP, uint32, error) {
	var errs []error
	for _, client := range s.sortClients(domain) {
		if !option.FakeEnable && strings.EqualFold(client.Name(), "FakeDNS") {
			errors.LogDebug(s.ctx, "skip DNS resolution for domain ", domain, " at server ", client.Name())
			continue
		}

		ips, ttl, err := client.QueryIP(s.ctx, domain, option)

		if len(ips) > 0 {
			return ips, ttl, nil
		}

		errors.LogInfoInner(s.ctx, err, "failed to lookup ip for domain ", domain, " at server ", client.Name(), " in serial query mode")
		if err == nil {
			err = dns.ErrEmptyResponse
		}
		errs = append(errs, err)
	}
	return nil, 0, mergeQueryErrors(domain, errs)
}

func (s *DNS) parallelQuery(domain string, option dns.IPOption) ([]net.IP, uint32, error) {
	var errs []error
	clients := s.sortClients(domain)

	resultsChan := asyncQueryAll(domain, option, clients, s.ctx)

	groups, groupOf := makeGroups( /*s.ctx,*/ clients)
	results := make([]*queryResult, len(clients))
	pending := make([]int, len(groups))
	for gi, g := range groups {
		pending[gi] = g.end - g.start + 1
	}

	nextGroup := 0
	for range clients {
		result := <-resultsChan
		results[result.index] = &result

		gi := groupOf[result.index]
		pending[gi]--

		for nextGroup < len(groups) {
			g := groups[nextGroup]

			// group race, minimum rtt -> return
			for j := g.start; j <= g.end; j++ {
				r := results[j]
				if r != nil && r.err == nil && len(r.ips) > 0 {
					return r.ips, r.ttl, nil
				}
			}

			// current group is incomplete and no one success -> continue pending
			if pending[nextGroup] > 0 {
				break
			}

			// all failed -> log and continue next group
			for j := g.start; j <= g.end; j++ {
				r := results[j]
				e := r.err
				if e == nil {
					e = dns.ErrEmptyResponse
				}
				errors.LogInfoInner(s.ctx, e, "failed to lookup ip for domain ", domain, " at server ", clients[j].Name(), " in parallel query mode")
				errs = append(errs, e)
			}
			nextGroup++
		}
	}

	return nil, 0, mergeQueryErrors(domain, errs)
}

type queryResult struct {
	ips   []net.IP
	ttl   uint32
	err   error
	index int
}

func asyncQueryAll(domain string, option dns.IPOption, clients []*Client, ctx context.Context) chan queryResult {
	if len(clients) == 0 {
		ch := make(chan queryResult)
		close(ch)
		return ch
	}

	ch := make(chan queryResult, len(clients))
	for i, client := range clients {
		if !option.FakeEnable && strings.EqualFold(client.Name(), "FakeDNS") {
			errors.LogDebug(ctx, "skip DNS resolution for domain ", domain, " at server ", client.Name())
			ch <- queryResult{err: dns.ErrEmptyResponse, index: i}
			continue
		}

		go func(i int, c *Client) {
			qctx := ctx
			if !c.server.IsDisableCache() {
				nctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.timeoutMs*2)
				qctx = nctx
				defer cancel()
			}
			ips, ttl, err := c.QueryIP(qctx, domain, option)
			ch <- queryResult{ips: ips, ttl: ttl, err: err, index: i}
		}(i, client)
	}
	return ch
}

type group struct{ start, end int }

// merge only adjacent and rule-equivalent Client into a single group
func makeGroups( /*ctx context.Context,*/ clients []*Client) ([]group, []int) {
	n := len(clients)
	if n == 0 {
		return nil, nil
	}
	groups := make([]group, 0, n)
	groupOf := make([]int, n)

	s, e := 0, 0
	for i := 1; i < n; i++ {
		if clients[i-1].policyID == clients[i].policyID {
			e = i
		} else {
			for k := s; k <= e; k++ {
				groupOf[k] = len(groups)
			}
			groups = append(groups, group{start: s, end: e})
			s, e = i, i
		}
	}
	for k := s; k <= e; k++ {
		groupOf[k] = len(groups)
	}
	groups = append(groups, group{start: s, end: e})

	// var b strings.Builder
	// b.WriteString("dns grouping: total clients=")
	// b.WriteString(strconv.Itoa(n))
	// b.WriteString(", groups=")
	// b.WriteString(strconv.Itoa(len(groups)))

	// for gi, g := range groups {
	// 	b.WriteString("\n  [")
	// 	b.WriteString(strconv.Itoa(g.start))
	// 	b.WriteString("..")
	// 	b.WriteString(strconv.Itoa(g.end))
	// 	b.WriteString("] gid=")
	// 	b.WriteString(strconv.Itoa(gi))
	// 	b.WriteString(" pid=")
	// 	b.WriteString(strconv.FormatUint(uint64(clients[g.start].policyID), 10))
	// 	b.WriteString(" members: ")

	// 	for i := g.start; i <= g.end; i++ {
	// 		if i > g.start {
	// 			b.WriteString(", ")
	// 		}
	// 		b.WriteString(strconv.Itoa(i))
	// 		b.WriteByte(':')
	// 		b.WriteString(clients[i].Name())
	// 	}
	// }
	// errors.LogDebug(ctx, b.String())

	return groups, groupOf
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return New(ctx, config.(*Config))
	}))
}

func probeRoutes() (ipv4 bool, ipv6 bool) {
	if conn, err := net.Dial("udp4", "192.33.4.12:53"); err == nil {
		ipv4 = true
		conn.Close()
	}
	if conn, err := net.Dial("udp6", "[2001:500:2::c]:53"); err == nil {
		ipv6 = true
		conn.Close()
	}
	return
}

var routeCache struct {
	sync.Once
	sync.RWMutex
	expire     time.Time
	ipv4, ipv6 bool
}

func checkRoutes() (bool, bool) {
	if !isGUIPlatform {
		routeCache.Once.Do(func() {
			routeCache.ipv4, routeCache.ipv6 = probeRoutes()
		})
		return routeCache.ipv4, routeCache.ipv6
	}

	routeCache.RWMutex.RLock()
	now := time.Now()
	if routeCache.expire.After(now) {
		routeCache.RWMutex.RUnlock()
		return routeCache.ipv4, routeCache.ipv6
	}
	routeCache.RWMutex.RUnlock()

	routeCache.RWMutex.Lock()
	defer routeCache.RWMutex.Unlock()

	now = time.Now()
	if routeCache.expire.After(now) { // double-check
		return routeCache.ipv4, routeCache.ipv6
	}
	routeCache.ipv4, routeCache.ipv6 = probeRoutes()    // ~2ms
	routeCache.expire = now.Add(100 * time.Millisecond) // ttl
	return routeCache.ipv4, routeCache.ipv6
}

var isGUIPlatform = detectGUIPlatform()

func detectGUIPlatform() bool {
	switch runtime.GOOS {
	case "android", "ios", "windows", "darwin":
		return true
	case "linux", "freebsd", "openbsd":
		if t := os.Getenv("XDG_SESSION_TYPE"); t == "wayland" || t == "x11" {
			return true
		}
		if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
			return true
		}
	}
	return false
}
