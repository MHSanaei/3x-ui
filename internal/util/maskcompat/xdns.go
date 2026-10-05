package maskcompat

import "strings"

// legacyXdnsEDNS0 is the EDNS0 payload the pre-26.9.30 xdns always negotiated;
// the object shape makes it opt-in and caps every answer at 512 bytes without it.
const legacyXdnsEDNS0 = 1232

var legacyXdnsRecordTypes = map[string]int{"": 16, "txt": 16, "a": 1, "aaaa": 28}

// UpgradeLegacyXdns rewrites xdns masks from the string lists xray-core 26.9.30
// (#6718) no longer parses into its object lists; one legacy mask fails the whole config.
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
	if !hasLegacyXdnsEntry(rawDomains) && !hasLegacyXdnsEntry(rawResolvers) {
		return false
	}
	domains := make([]any, 0, len(rawDomains))
	listed := map[string]bool{}
	addDomain := func(domain map[string]any) {
		key, _ := domain["name"].(string)
		key = strings.ToLower(key)
		if key != "" && listed[key] {
			return
		}
		listed[key] = true
		domains = append(domains, domain)
	}
	for _, entry := range rawDomains {
		switch value := entry.(type) {
		case map[string]any:
			addDomain(value)
		case string:
			if domain, ok := legacyXdnsDomain(value); ok {
				addDomain(domain)
			}
		}
	}
	resolvers := make([]any, 0, len(rawResolvers))
	for _, entry := range rawResolvers {
		spec, ok := entry.(string)
		if !ok {
			resolvers = append(resolvers, entry)
			continue
		}
		head, addr, found := strings.Cut(spec, "+udp://")
		addr = strings.TrimSpace(addr)
		domain, valid := legacyXdnsDomain(head)
		if !found || addr == "" || !valid {
			continue
		}
		addDomain(domain)
		resolvers = append(resolvers, map[string]any{
			"type":     "udp",
			"settings": map[string]any{"addr": addr},
		})
	}
	settings["domains"] = domains
	if len(resolvers) > 0 {
		settings["resolvers"] = resolvers
	} else {
		delete(settings, "resolvers")
	}
	return true
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
	return map[string]any{"name": name, "types": []any{recordType}, "edns0": legacyXdnsEDNS0}, true
}

func hasLegacyXdnsEntry(values []any) bool {
	for _, value := range values {
		if _, ok := value.(string); ok {
			return true
		}
	}
	return false
}
