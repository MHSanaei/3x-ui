package sub

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// ShareEndpoint is one render target for a subscription link: the address/port
// to dial plus an optional set of TLS overrides. It unifies two sources behind
// one type so the per-protocol link builders don't branch on where the override
// came from:
//
//   - a legacy externalProxy entry (Phase 1): the source map is carried in `ep`
//     and applied through the unchanged applyExternalProxyTLS* helpers, so the
//     emitted link is byte-identical to the pre-refactor output;
//   - a Host row (Phase 4): leaves `ep` nil and uses typed override fields.
//
// ForceTls is the verbatim "same"/"tls"/"none"/"" value — never pre-resolved,
// because three behaviors branch on the raw string (keep-base, obj["tls"]
// rewrite, none-strip).
type ShareEndpoint struct {
	Address  string
	Port     int
	Remark   string // extra remark slot fed to genRemark, not a rendered remark
	ForceTls string

	// ep is the source externalProxy entry. nil for host/default endpoints.
	ep map[string]any
}

// externalProxyToEndpoint maps one externalProxy entry to an endpoint that
// carries the entry for delegated, provably-identical TLS application.
func externalProxyToEndpoint(ep map[string]any) ShareEndpoint {
	e := ShareEndpoint{ep: ep}
	e.Address, _ = ep["dest"].(string)
	if p, ok := ep["port"].(float64); ok {
		e.Port = int(p)
	}
	e.Remark, _ = ep["remark"].(string)
	e.ForceTls, _ = ep["forceTls"].(string)
	return e
}

// inboundDefaultEndpoint is the endpoint for an inbound's own resolved
// address/port (the no-externalProxy default). forceTls "same" keeps the base
// security; no per-endpoint TLS override.
func (s *SubService) inboundDefaultEndpoint(inbound *model.Inbound) ShareEndpoint {
	return ShareEndpoint{
		Address:  s.resolveInboundAddress(inbound),
		Port:     inbound.Port,
		ForceTls: "same",
	}
}

// applyEndpointTLSParams applies an endpoint's TLS overrides onto a URL-param
// map. External-proxy endpoints delegate to the unchanged helper; host/default
// endpoints carry no override yet (Phase 4).
func applyEndpointTLSParams(e ShareEndpoint, params map[string]string, security string) {
	if e.ep != nil {
		applyExternalProxyTLSParams(e.ep, params, security)
	}
}

// applyEndpointTLSObj is applyEndpointTLSParams for the VMess base64-JSON form.
func applyEndpointTLSObj(e ShareEndpoint, obj map[string]any, security string) {
	if e.ep != nil {
		applyExternalProxyTLSObj(e.ep, obj, security)
	}
}

// buildEndpointLinks renders one URL-param link per endpoint (vless/trojan/ss).
// securityToApply mirrors the legacy externalProxy loop: "same" keeps the base
// security, otherwise the endpoint's forceTls wins; "none" strips TLS hint
// fields at emit time.
func (s *SubService) buildEndpointLinks(
	eps []ShareEndpoint,
	params map[string]string,
	baseSecurity string,
	makeLink func(dest string, port int) string,
	makeRemark func(e ShareEndpoint) string,
) string {
	links := make([]string, 0, len(eps))
	for _, e := range eps {
		securityToApply := baseSecurity
		if e.ForceTls != "same" {
			securityToApply = e.ForceTls
		}
		nextParams := cloneStringMap(params)
		applyEndpointTLSParams(e, nextParams, securityToApply)
		applyEndpointRealityParams(e, nextParams, securityToApply)
		applyEndpointHostPath(e, nextParams)
		applyEndpointAllowInsecure(e, nextParams, securityToApply)
		links = append(links, buildLinkWithParamsAndSecurity(
			makeLink(e.Address, e.Port),
			nextParams,
			makeRemark(e),
			securityToApply,
			e.ForceTls == "none",
		))
	}
	return strings.Join(links, "\n")
}

// buildEndpointVmessLinks renders one VMess base64-JSON link per endpoint.
func (s *SubService) buildEndpointVmessLinks(eps []ShareEndpoint, baseObj map[string]any, inbound *model.Inbound, email string) string {
	var links strings.Builder
	for index, e := range eps {
		securityToApply, _ := baseObj["tls"].(string)
		if e.ForceTls != "same" {
			securityToApply = e.ForceTls
		}
		newObj := cloneVmessShareObj(baseObj, e.ForceTls)
		newObj["ps"] = s.endpointRemark(inbound, email, e.ep)
		newObj["add"] = e.Address
		newObj["port"] = e.Port
		if e.ForceTls != "same" {
			newObj["tls"] = e.ForceTls
		}
		applyEndpointTLSObj(e, newObj, securityToApply)
		applyEndpointHostPathObj(e, newObj)
		if index > 0 {
			links.WriteString("\n")
		}
		links.WriteString(buildVmessLink(newObj))
	}
	return links.String()
}
