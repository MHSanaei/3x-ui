package maskcompat

import (
	"slices"
	"strings"
)

// legacyXdnsEDNS0 is the EDNS0 payload the pre-26.9.30 xdns always negotiated;
// the object shape makes it opt-in and caps every answer at 512 bytes without it.
const legacyXdnsEDNS0 = 1232

var legacyXdnsRecordTypes = map[string]int{"": 16, "txt": 16, "a": 1, "aaaa": 28}

// UpgradeLegacyXdns rewrites xdns masks into the names/addrs lists xray-core 26.10.10
// (#7090) parses: it ignores 26.9.30's name/type+settings keys and fails the string lists.
func UpgradeLegacyXdns(finalmask any) bool {
	fm, _ := finalmask.(map[string]any)
	masks, _ := fm["udp"].([]any)
	changed := false
	for _, entry := range masks {
		mask, _ := entry.(map[string]any)
		if maskType, _ := mask["type"].(string); !strings.EqualFold(maskType, "xdns") {
			continue
		}
		if settings, ok := mask["settings"].(map[string]any); ok && upgradeLegacyXdnsSettings(settings) {
			changed = true
		}
	}
	return changed
}

// upgradeLegacyXdnsSettings maps a bare name to TXT, the type legacy clients queried by
// default, and drops entries the old core refused instead of keeping them to fail again.
func upgradeLegacyXdnsSettings(settings map[string]any) bool {
	rawDomains, _ := settings["domains"].([]any)
	rawResolvers, _ := settings["resolvers"].([]any)
	if !slices.ContainsFunc(rawDomains, isLegacyXdnsDomain) && !slices.ContainsFunc(rawResolvers, isLegacyXdnsResolver) {
		return false
	}
	domains := make([]any, 0, len(rawDomains))
	listed := map[string]bool{}
	addDomain := func(domain map[string]any) {
		names, _ := domain["names"].([]any)
		for _, name := range names {
			if s, ok := name.(string); ok {
				listed[strings.ToLower(s)] = true
			}
		}
		domains = append(domains, domain)
	}
	addLegacyDomain := func(domain map[string]any) {
		if name, _ := domain["names"].([]any)[0].(string); !listed[strings.ToLower(name)] {
			addDomain(domain)
		}
	}
	for _, entry := range rawDomains {
		switch value := entry.(type) {
		case map[string]any:
			addDomain(upgradeXdnsDomainObject(value))
		case string:
			if domain, ok := legacyXdnsDomain(value); ok {
				addLegacyDomain(domain)
			}
		}
	}
	resolvers := make([]any, 0, len(rawResolvers))
	for _, entry := range rawResolvers {
		switch value := entry.(type) {
		case string:
			head, addr, found := strings.Cut(value, "+udp://")
			addr = strings.TrimSpace(addr)
			domain, valid := legacyXdnsDomain(head)
			if !found || addr == "" || !valid {
				continue
			}
			addLegacyDomain(domain)
			resolvers = append(resolvers, map[string]any{"addrs": []any{"udp://" + addr}})
		case map[string]any:
			if !isLegacyXdnsResolver(value) {
				resolvers = append(resolvers, value)
			} else if addr, ok := legacyXdnsResolverAddr(value); ok {
				resolvers = append(resolvers, map[string]any{"addrs": []any{addr}})
			}
		default:
			resolvers = append(resolvers, entry)
		}
	}
	settings["domains"] = domains
	if len(resolvers) > 0 {
		settings["resolvers"] = resolvers
	} else {
		delete(settings, "resolvers")
	}
	return true
}

// upgradeXdnsDomainObject folds a 26.9.30 domain's single "name" into the "names" list.
func upgradeXdnsDomainObject(domain map[string]any) map[string]any {
	raw, legacy := domain["name"]
	if !legacy {
		return domain
	}
	delete(domain, "name")
	name, _ := raw.(string)
	name = strings.TrimSpace(name)
	names, _ := domain["names"].([]any)
	if name != "" && !slices.Contains(names, any(name)) {
		names = append([]any{name}, names...)
	}
	if names == nil {
		names = []any{}
	}
	domain["names"] = names
	return domain
}

// legacyXdnsResolverAddr turns a 26.9.30 {type, settings:{addr}} resolver into the URL
// the addrs list takes; that loader refused every type but udp and tcp.
func legacyXdnsResolverAddr(resolver map[string]any) (string, bool) {
	kind, _ := resolver["type"].(string)
	kind = strings.ToLower(strings.TrimSpace(kind))
	settings, _ := resolver["settings"].(map[string]any)
	addr, _ := settings["addr"].(string)
	addr = strings.TrimSpace(addr)
	if (kind != "udp" && kind != "tcp") || addr == "" {
		return "", false
	}
	return kind + "://" + addr, true
}

// legacyXdnsDomain parses the "name[:txt|a|aaaa]" spec both legacy lists used.
func legacyXdnsDomain(spec string) (map[string]any, bool) {
	name, method := strings.TrimSpace(spec), ""
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name, method = name[:i], strings.ToLower(name[i+1:])
	}
	name = strings.Trim(name, ".")
	recordType, known := legacyXdnsRecordTypes[method]
	if name == "" || !known {
		return nil, false
	}
	return map[string]any{"names": []any{name}, "types": []any{recordType}, "edns0": legacyXdnsEDNS0}, true
}

func isLegacyXdnsDomain(value any) bool {
	switch domain := value.(type) {
	case string:
		return true
	case map[string]any:
		_, legacy := domain["name"]
		return legacy
	}
	return false
}

func isLegacyXdnsResolver(value any) bool {
	switch resolver := value.(type) {
	case string:
		return true
	case map[string]any:
		_, current := resolver["addrs"]
		return !current
	}
	return false
}
