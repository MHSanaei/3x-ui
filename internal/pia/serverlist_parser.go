package pia

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"regexp"
	"sort"
	"strings"
)

var (
	regionIDPattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	countryCodePattern = regexp.MustCompile(`^[A-Za-z]{2}$`)
)

type ServerListParser interface {
	Schema() string
	CanParse(raw []byte) bool
	Parse(raw []byte) ([]Region, error)
}

type (
	V6Parser struct{}
	V7Parser struct{}
)

func (V6Parser) Schema() string { return "v6" }
func (V7Parser) Schema() string { return "v7" }

func (V6Parser) CanParse(raw []byte) bool { return schemaVersion(raw) == 0 || schemaVersion(raw) == 6 }
func (V7Parser) CanParse(raw []byte) bool { return schemaVersion(raw) == 7 }

func (V6Parser) Parse(raw []byte) ([]Region, error) { return parseCatalog(raw, false) }
func (V7Parser) Parse(raw []byte) ([]Region, error) { return parseCatalog(raw, true) }

type catalogEnvelope struct {
	Version json.RawMessage            `json:"version"`
	Groups  map[string]json.RawMessage `json:"groups"`
	Regions []rawRegion                `json:"regions"`
}

type rawRegion struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Country        string     `json:"country"`
	Geo            *bool      `json:"geo"`
	Offline        *bool      `json:"offline"`
	PortForward    *bool      `json:"port_forward"`
	PortForwarding *bool      `json:"port_forwarding"`
	Servers        rawServers `json:"servers"`
}

type rawServers struct {
	WireGuard []rawServer `json:"wg"`
}

type rawServer struct {
	IP       string `json:"ip"`
	CN       string `json:"cn"`
	Hostname string `json:"hostname"`
}

func ParseServerList(raw []byte, schemaHint string) ([]Region, string, error) {
	parsers := []ServerListParser{V7Parser{}, V6Parser{}}
	version, present, err := detectSchemaVersion(raw)
	if err != nil {
		return nil, "", WrapError(CodeCatalogSchemaUnsupported, "PIA returned an invalid server-list version.", err)
	}
	if present {
		for _, parser := range parsers {
			if strings.TrimPrefix(parser.Schema(), "v") == fmt.Sprint(version) {
				regions, parseErr := parser.Parse(raw)
				return regions, parser.Schema(), parseErr
			}
		}
		return nil, "", NewError(CodeCatalogSchemaUnsupported, "This PIA server-list schema is not supported.")
	}

	hint := strings.ToLower(strings.TrimPrefix(schemaHint, "v"))
	if hint != "" {
		for _, parser := range parsers {
			if strings.TrimPrefix(parser.Schema(), "v") != hint {
				continue
			}
			regions, err := parser.Parse(raw)
			return regions, parser.Schema(), err
		}
	}
	for _, parser := range parsers {
		if parser.CanParse(raw) {
			regions, err := parser.Parse(raw)
			return regions, parser.Schema(), err
		}
	}
	return nil, "", NewError(CodeCatalogSchemaUnsupported, "This PIA server-list schema is not supported.")
}

func schemaVersion(raw []byte) int {
	version, present, err := detectSchemaVersion(raw)
	if err != nil || !present {
		return 0
	}
	return version
}

func detectSchemaVersion(raw []byte) (int, bool, error) {
	var envelope struct {
		Version json.RawMessage `json:"version"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, false, err
	}
	if len(envelope.Version) == 0 || string(envelope.Version) == "null" {
		return 0, false, nil
	}
	var number int
	if json.Unmarshal(envelope.Version, &number) == nil {
		if number < 1 {
			return 0, true, fmt.Errorf("version must be positive")
		}
		return number, true, nil
	}
	var text string
	if json.Unmarshal(envelope.Version, &text) == nil {
		text = strings.TrimPrefix(strings.ToLower(text), "v")
		if _, err := fmt.Sscanf(text, "%d", &number); err == nil && fmt.Sprint(number) == text && number > 0 {
			return number, true, nil
		}
	}
	return 0, true, fmt.Errorf("version has an unsupported type or value")
}

func parseCatalog(raw []byte, allowV7Aliases bool) ([]Region, error) {
	var envelope catalogEnvelope
	if err := decodeSingleJSON(raw, &envelope); err != nil {
		return nil, WrapError(CodeCatalogSchemaUnsupported, "PIA returned an invalid region list.", err)
	}
	if len(envelope.Groups) == 0 || len(envelope.Regions) == 0 {
		return nil, NewError(CodeCatalogSchemaUnsupported, "The PIA region list is missing required fields.")
	}

	seen := make(map[string]struct{}, len(envelope.Regions))
	regions := make([]Region, 0, len(envelope.Regions))
	for _, rawRegion := range envelope.Regions {
		if !regionIDPattern.MatchString(rawRegion.ID) || strings.TrimSpace(rawRegion.Name) == "" || len(rawRegion.Name) > 128 {
			continue
		}
		idKey := strings.ToLower(rawRegion.ID)
		if _, duplicate := seen[idKey]; duplicate {
			continue
		}
		seen[idKey] = struct{}{}
		if !countryCodePattern.MatchString(rawRegion.Country) || rawRegion.Geo == nil || rawRegion.Offline == nil {
			continue
		}
		if *rawRegion.Offline {
			continue
		}
		portForwarding := false
		if rawRegion.PortForward != nil {
			portForwarding = *rawRegion.PortForward
		} else if allowV7Aliases && rawRegion.PortForwarding != nil {
			portForwarding = *rawRegion.PortForwarding
		}
		servers := make([]WireGuardServer, 0, len(rawRegion.Servers.WireGuard))
		for _, rawServer := range rawRegion.Servers.WireGuard {
			hostname := rawServer.CN
			if hostname == "" && allowV7Aliases {
				hostname = rawServer.Hostname
			}
			ip, err := netip.ParseAddr(rawServer.IP)
			if err != nil || !ip.Is4() || ip.IsUnspecified() || !validHostname(hostname) {
				continue
			}
			servers = append(servers, WireGuardServer{Hostname: hostname, IP: ip})
		}
		if len(servers) == 0 {
			continue
		}
		regions = append(regions, Region{
			ID: rawRegion.ID, Name: rawRegion.Name, CountryCode: strings.ToUpper(rawRegion.Country), Geo: *rawRegion.Geo,
			PortForwarding: portForwarding, WireGuard: servers,
		})
	}
	if len(regions) == 0 {
		return nil, NewError(CodeCatalogSchemaUnsupported, "The PIA region list contains no available WireGuard regions.")
	}
	sort.Slice(regions, func(i, j int) bool {
		if regions[i].CountryCode == regions[j].CountryCode {
			return regions[i].Name < regions[j].Name
		}
		return regions[i].CountryCode < regions[j].CountryCode
	})
	return regions, nil
}
