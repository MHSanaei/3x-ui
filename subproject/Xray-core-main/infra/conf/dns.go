package conf

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/xtls/xray-core/app/dns"
	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
)

type NameServerConfig struct {
	Address         *Address   `json:"address"`
	ClientIP        *Address   `json:"clientIp"`
	Port            uint16     `json:"port"`
	SkipFallback    bool       `json:"skipFallback"`
	Domains         []string   `json:"domains"`
	ExpectedIPs     StringList `json:"expectedIPs"`
	ExpectIPs       StringList `json:"expectIPs"`
	QueryStrategy   string     `json:"queryStrategy"`
	Tag             string     `json:"tag"`
	TimeoutMs       uint64     `json:"timeoutMs"`
	DisableCache    *bool      `json:"disableCache"`
	ServeStale      *bool      `json:"serveStale"`
	ServeExpiredTTL *uint32    `json:"serveExpiredTTL"`
	FinalQuery      bool       `json:"finalQuery"`
	UnexpectedIPs   StringList `json:"unexpectedIPs"`
}

// UnmarshalJSON implements encoding/json.Unmarshaler.UnmarshalJSON
func (c *NameServerConfig) UnmarshalJSON(data []byte) error {
	var address Address
	if err := json.Unmarshal(data, &address); err == nil {
		c.Address = &address
		return nil
	}

	var advanced struct {
		Address         *Address   `json:"address"`
		ClientIP        *Address   `json:"clientIp"`
		Port            uint16     `json:"port"`
		SkipFallback    bool       `json:"skipFallback"`
		Domains         []string   `json:"domains"`
		ExpectedIPs     StringList `json:"expectedIPs"`
		ExpectIPs       StringList `json:"expectIPs"`
		QueryStrategy   string     `json:"queryStrategy"`
		Tag             string     `json:"tag"`
		TimeoutMs       uint64     `json:"timeoutMs"`
		DisableCache    *bool      `json:"disableCache"`
		ServeStale      *bool      `json:"serveStale"`
		ServeExpiredTTL *uint32    `json:"serveExpiredTTL"`
		FinalQuery      bool       `json:"finalQuery"`
		UnexpectedIPs   StringList `json:"unexpectedIPs"`
	}
	if err := json.Unmarshal(data, &advanced); err == nil {
		c.Address = advanced.Address
		c.ClientIP = advanced.ClientIP
		c.Port = advanced.Port
		c.SkipFallback = advanced.SkipFallback
		c.Domains = advanced.Domains
		c.ExpectedIPs = advanced.ExpectedIPs
		c.ExpectIPs = advanced.ExpectIPs
		c.QueryStrategy = advanced.QueryStrategy
		c.Tag = advanced.Tag
		c.TimeoutMs = advanced.TimeoutMs
		c.DisableCache = advanced.DisableCache
		c.ServeStale = advanced.ServeStale
		c.ServeExpiredTTL = advanced.ServeExpiredTTL
		c.FinalQuery = advanced.FinalQuery
		c.UnexpectedIPs = advanced.UnexpectedIPs
		return nil
	}

	return errors.New("failed to parse name server: ", string(data))
}

func toDomainMatchingType(t router.Domain_Type) dns.DomainMatchingType {
	switch t {
	case router.Domain_Domain:
		return dns.DomainMatchingType_Subdomain
	case router.Domain_Full:
		return dns.DomainMatchingType_Full
	case router.Domain_Plain:
		return dns.DomainMatchingType_Keyword
	case router.Domain_Regex:
		return dns.DomainMatchingType_Regex
	default:
		panic("unknown domain type")
	}
}

func (c *NameServerConfig) Build() (*dns.NameServer, error) {
	if c.Address == nil {
		return nil, errors.New("NameServer address is not specified.")
	}

	var domains []*dns.NameServer_PriorityDomain
	var originalRules []*dns.NameServer_OriginalRule

	for _, rule := range c.Domains {
		parsedDomain, err := parseDomainRule(rule)
		if err != nil {
			return nil, errors.New("invalid domain rule: ", rule).Base(err)
		}

		for _, pd := range parsedDomain {
			domains = append(domains, &dns.NameServer_PriorityDomain{
				Type:   toDomainMatchingType(pd.Type),
				Domain: pd.Value,
			})
		}
		originalRules = append(originalRules, &dns.NameServer_OriginalRule{
			Rule: rule,
			Size: uint32(len(parsedDomain)),
		})
	}

	if len(c.ExpectedIPs) == 0 {
		c.ExpectedIPs = c.ExpectIPs
	}

	actPrior := false
	var newExpectedIPs StringList
	for _, s := range c.ExpectedIPs {
		if s == "*" {
			actPrior = true
		} else {
			newExpectedIPs = append(newExpectedIPs, s)
		}
	}

	actUnprior := false
	var newUnexpectedIPs StringList
	for _, s := range c.UnexpectedIPs {
		if s == "*" {
			actUnprior = true
		} else {
			newUnexpectedIPs = append(newUnexpectedIPs, s)
		}
	}

	expectedGeoipList, err := ToCidrList(newExpectedIPs)
	if err != nil {
		return nil, errors.New("invalid expected IP rule: ", c.ExpectedIPs).Base(err)
	}

	unexpectedGeoipList, err := ToCidrList(newUnexpectedIPs)
	if err != nil {
		return nil, errors.New("invalid unexpected IP rule: ", c.UnexpectedIPs).Base(err)
	}

	var myClientIP []byte
	if c.ClientIP != nil {
		if !c.ClientIP.Family().IsIP() {
			return nil, errors.New("not an IP address:", c.ClientIP.String())
		}
		myClientIP = []byte(c.ClientIP.IP())
	}

	return &dns.NameServer{
		Address: &net.Endpoint{
			Network: net.Network_UDP,
			Address: c.Address.Build(),
			Port:    uint32(c.Port),
		},
		ClientIp:          myClientIP,
		SkipFallback:      c.SkipFallback,
		PrioritizedDomain: domains,
		ExpectedGeoip:     expectedGeoipList,
		OriginalRules:     originalRules,
		QueryStrategy:     resolveQueryStrategy(c.QueryStrategy),
		ActPrior:          actPrior,
		Tag:               c.Tag,
		TimeoutMs:         c.TimeoutMs,
		DisableCache:      c.DisableCache,
		ServeStale:        c.ServeStale,
		ServeExpiredTTL:   c.ServeExpiredTTL,
		FinalQuery:        c.FinalQuery,
		UnexpectedGeoip:   unexpectedGeoipList,
		ActUnprior:        actUnprior,
	}, nil
}

var typeMap = map[router.Domain_Type]dns.DomainMatchingType{
	router.Domain_Full:   dns.DomainMatchingType_Full,
	router.Domain_Domain: dns.DomainMatchingType_Subdomain,
	router.Domain_Plain:  dns.DomainMatchingType_Keyword,
	router.Domain_Regex:  dns.DomainMatchingType_Regex,
}

// DNSConfig is a JSON serializable object for dns.Config.
type DNSConfig struct {
	Servers                []*NameServerConfig `json:"servers"`
	Hosts                  *HostsWrapper       `json:"hosts"`
	ClientIP               *Address            `json:"clientIp"`
	Tag                    string              `json:"tag"`
	QueryStrategy          string              `json:"queryStrategy"`
	DisableCache           bool                `json:"disableCache"`
	ServeStale             bool                `json:"serveStale"`
	ServeExpiredTTL        uint32              `json:"serveExpiredTTL"`
	DisableFallback        bool                `json:"disableFallback"`
	DisableFallbackIfMatch bool                `json:"disableFallbackIfMatch"`
	EnableParallelQuery    bool                `json:"enableParallelQuery"`
	UseSystemHosts         bool                `json:"useSystemHosts"`
}

type HostAddress struct {
	addr  *Address
	addrs []*Address
}

// MarshalJSON implements encoding/json.Marshaler.MarshalJSON
func (h *HostAddress) MarshalJSON() ([]byte, error) {
	if (h.addr != nil) != (h.addrs != nil) {
		if h.addr != nil {
			return json.Marshal(h.addr)
		} else if h.addrs != nil {
			return json.Marshal(h.addrs)
		}
	}
	return nil, errors.New("unexpected config state")
}

// UnmarshalJSON implements encoding/json.Unmarshaler.UnmarshalJSON
func (h *HostAddress) UnmarshalJSON(data []byte) error {
	addr := new(Address)
	var addrs []*Address
	switch {
	case json.Unmarshal(data, &addr) == nil:
		h.addr = addr
	case json.Unmarshal(data, &addrs) == nil:
		h.addrs = addrs
	default:
		return errors.New("invalid address")
	}
	return nil
}

type HostsWrapper struct {
	Hosts map[string]*HostAddress
}

func getHostMapping(ha *HostAddress) *dns.Config_HostMapping {
	if ha.addr != nil {
		if ha.addr.Family().IsDomain() {
			return &dns.Config_HostMapping{
				ProxiedDomain: ha.addr.Domain(),
			}
		}
		return &dns.Config_HostMapping{
			Ip: [][]byte{ha.addr.IP()},
		}
	}

	ips := make([][]byte, 0, len(ha.addrs))
	for _, addr := range ha.addrs {
		if addr.Family().IsDomain() {
			return &dns.Config_HostMapping{
				ProxiedDomain: addr.Domain(),
			}
		}
		ips = append(ips, []byte(addr.IP()))
	}
	return &dns.Config_HostMapping{
		Ip: ips,
	}
}

// MarshalJSON implements encoding/json.Marshaler.MarshalJSON
func (m *HostsWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Hosts)
}

// UnmarshalJSON implements encoding/json.Unmarshaler.UnmarshalJSON
func (m *HostsWrapper) UnmarshalJSON(data []byte) error {
	hosts := make(map[string]*HostAddress)
	err := json.Unmarshal(data, &hosts)
	if err == nil {
		m.Hosts = hosts
		return nil
	}
	return errors.New("invalid DNS hosts").Base(err)
}

// Build implements Buildable
func (m *HostsWrapper) Build() ([]*dns.Config_HostMapping, error) {
	mappings := make([]*dns.Config_HostMapping, 0, 20)

	domains := make([]string, 0, len(m.Hosts))
	for domain := range m.Hosts {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	for _, domain := range domains {
		switch {
		case strings.HasPrefix(domain, "domain:"):
			domainName := domain[7:]
			if len(domainName) == 0 {
				return nil, errors.New("empty domain type of rule: ", domain)
			}
			mapping := getHostMapping(m.Hosts[domain])
			mapping.Type = dns.DomainMatchingType_Subdomain
			mapping.Domain = domainName
			mappings = append(mappings, mapping)

		case strings.HasPrefix(domain, "geosite:"):
			listName := domain[8:]
			if len(listName) == 0 {
				return nil, errors.New("empty geosite rule: ", domain)
			}
			geositeList, err := loadGeositeWithAttr("geosite.dat", listName)
			if err != nil {
				return nil, errors.New("failed to load geosite: ", listName).Base(err)
			}
			for _, d := range geositeList {
				mapping := getHostMapping(m.Hosts[domain])
				mapping.Type = typeMap[d.Type]
				mapping.Domain = d.Value
				mappings = append(mappings, mapping)
			}

		case strings.HasPrefix(domain, "regexp:"):
			regexpVal := domain[7:]
			if len(regexpVal) == 0 {
				return nil, errors.New("empty regexp type of rule: ", domain)
			}
			mapping := getHostMapping(m.Hosts[domain])
			mapping.Type = dns.DomainMatchingType_Regex
			mapping.Domain = regexpVal
			mappings = append(mappings, mapping)

		case strings.HasPrefix(domain, "keyword:"):
			keywordVal := domain[8:]
			if len(keywordVal) == 0 {
				return nil, errors.New("empty keyword type of rule: ", domain)
			}
			mapping := getHostMapping(m.Hosts[domain])
			mapping.Type = dns.DomainMatchingType_Keyword
			mapping.Domain = keywordVal
			mappings = append(mappings, mapping)

		case strings.HasPrefix(domain, "full:"):
			fullVal := domain[5:]
			if len(fullVal) == 0 {
				return nil, errors.New("empty full domain type of rule: ", domain)
			}
			mapping := getHostMapping(m.Hosts[domain])
			mapping.Type = dns.DomainMatchingType_Full
			mapping.Domain = fullVal
			mappings = append(mappings, mapping)

		case strings.HasPrefix(domain, "dotless:"):
			mapping := getHostMapping(m.Hosts[domain])
			mapping.Type = dns.DomainMatchingType_Regex
			switch substr := domain[8:]; {
			case substr == "":
				mapping.Domain = "^[^.]*$"
			case !strings.Contains(substr, "."):
				mapping.Domain = "^[^.]*" + substr + "[^.]*$"
			default:
				return nil, errors.New("substr in dotless rule should not contain a dot: ", substr)
			}
			mappings = append(mappings, mapping)

		case strings.HasPrefix(domain, "ext:"):
			kv := strings.Split(domain[4:], ":")
			if len(kv) != 2 {
				return nil, errors.New("invalid external resource: ", domain)
			}
			filename := kv[0]
			list := kv[1]
			geositeList, err := loadGeositeWithAttr(filename, list)
			if err != nil {
				return nil, errors.New("failed to load domain list: ", list, " from ", filename).Base(err)
			}
			for _, d := range geositeList {
				mapping := getHostMapping(m.Hosts[domain])
				mapping.Type = typeMap[d.Type]
				mapping.Domain = d.Value
				mappings = append(mappings, mapping)
			}

		default:
			mapping := getHostMapping(m.Hosts[domain])
			mapping.Type = dns.DomainMatchingType_Full
			mapping.Domain = domain
			mappings = append(mappings, mapping)
		}
	}
	return mappings, nil
}

// Build implements Buildable
func (c *DNSConfig) Build() (*dns.Config, error) {
	config := &dns.Config{
		Tag:                    c.Tag,
		DisableCache:           c.DisableCache,
		ServeStale:             c.ServeStale,
		ServeExpiredTTL:        c.ServeExpiredTTL,
		DisableFallback:        c.DisableFallback,
		DisableFallbackIfMatch: c.DisableFallbackIfMatch,
		EnableParallelQuery:    c.EnableParallelQuery,
		QueryStrategy:          resolveQueryStrategy(c.QueryStrategy),
	}

	if c.ClientIP != nil {
		if !c.ClientIP.Family().IsIP() {
			return nil, errors.New("not an IP address:", c.ClientIP.String())
		}
		config.ClientIp = []byte(c.ClientIP.IP())
	}

	// Build PolicyID
	policyMap := map[string]uint32{}
	nextPolicyID := uint32(1)
	buildPolicyID := func(nsc *NameServerConfig) uint32 {
		var sb strings.Builder

		// ClientIP
		if nsc.ClientIP != nil {
			sb.WriteString("client=")
			sb.WriteString(nsc.ClientIP.String())
			sb.WriteByte('|')
		} else {
			sb.WriteString("client=none|")
		}

		// SkipFallback
		if nsc.SkipFallback {
			sb.WriteString("skip=1|")
		} else {
			sb.WriteString("skip=0|")
		}

		// QueryStrategy
		sb.WriteString("qs=")
		sb.WriteString(strings.ToLower(strings.TrimSpace(nsc.QueryStrategy)))
		sb.WriteByte('|')

		// Tag
		sb.WriteString("tag=")
		sb.WriteString(strings.ToLower(strings.TrimSpace(nsc.Tag)))
		sb.WriteByte('|')

		// []string helper
		writeList := func(tag string, lst []string) {
			if len(lst) == 0 {
				sb.WriteString(tag)
				sb.WriteString("=[]|")
				return
			}
			cp := make([]string, len(lst))
			for i, s := range lst {
				cp[i] = strings.TrimSpace(strings.ToLower(s))
			}
			sort.Strings(cp)
			sb.WriteString(tag)
			sb.WriteByte('=')
			sb.WriteString(strings.Join(cp, ","))
			sb.WriteByte('|')
		}

		writeList("domains", nsc.Domains)
		writeList("expected", nsc.ExpectedIPs)
		writeList("expect", nsc.ExpectIPs)
		writeList("unexpected", nsc.UnexpectedIPs)

		key := sb.String()

		if id, ok := policyMap[key]; ok {
			return id
		}
		id := nextPolicyID
		nextPolicyID++
		policyMap[key] = id
		return id
	}

	for _, server := range c.Servers {
		ns, err := server.Build()
		if err != nil {
			return nil, errors.New("failed to build nameserver").Base(err)
		}
		ns.PolicyID = buildPolicyID(server)
		config.NameServer = append(config.NameServer, ns)
	}

	if c.Hosts != nil {
		staticHosts, err := c.Hosts.Build()
		if err != nil {
			return nil, errors.New("failed to build hosts").Base(err)
		}
		config.StaticHosts = append(config.StaticHosts, staticHosts...)
	}
	if c.UseSystemHosts {
		systemHosts, err := readSystemHosts()
		if err != nil {
			return nil, errors.New("failed to read system hosts").Base(err)
		}
		for domain, ips := range systemHosts {
			config.StaticHosts = append(config.StaticHosts, &dns.Config_HostMapping{Ip: ips, Domain: domain, Type: dns.DomainMatchingType_Full})
		}
	}

	return config, nil
}

func resolveQueryStrategy(queryStrategy string) dns.QueryStrategy {
	switch strings.ToLower(queryStrategy) {
	case "useip", "use_ip", "use-ip":
		return dns.QueryStrategy_USE_IP
	case "useip4", "useipv4", "use_ip4", "use_ipv4", "use_ip_v4", "use-ip4", "use-ipv4", "use-ip-v4":
		return dns.QueryStrategy_USE_IP4
	case "useip6", "useipv6", "use_ip6", "use_ipv6", "use_ip_v6", "use-ip6", "use-ipv6", "use-ip-v6":
		return dns.QueryStrategy_USE_IP6
	case "usesys", "usesystem", "use_sys", "use_system", "use-sys", "use-system":
		return dns.QueryStrategy_USE_SYS
	default:
		return dns.QueryStrategy_USE_IP
	}
}

func readSystemHosts() (map[string][][]byte, error) {
	var hostsPath string
	switch runtime.GOOS {
	case "windows":
		hostsPath = filepath.Join(os.Getenv("SystemRoot"), "System32", "drivers", "etc", "hosts")
	default:
		hostsPath = "/etc/hosts"
	}

	file, err := os.Open(hostsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hostsMap := make(map[string][][]byte)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if i := strings.IndexByte(line, '#'); i >= 0 {
			// Discard comments.
			line = line[0:i]
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		addr := net.ParseAddress(f[0])
		if addr.Family().IsDomain() {
			continue
		}
		ip := addr.IP()
		for i := 1; i < len(f); i++ {
			domain := strings.TrimSuffix(f[i], ".")
			domain = strings.ToLower(domain)
			if v, ok := hostsMap[domain]; ok {
				hostsMap[domain] = append(v, ip)
			} else {
				hostsMap[domain] = [][]byte{ip}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return hostsMap, nil
}
