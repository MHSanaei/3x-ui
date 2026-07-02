package xray

import (
	"bytes"
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

// HotDiff describes the gRPC API operations needed to bring a running Xray
// instance from one generated config to another without restarting the
// process. It only covers the sections Xray can reload at runtime: inbounds,
// outbounds and routing rules/balancers.
type HotDiff struct {
	RemovedInboundTags  []string
	AddedInbounds       [][]byte
	RemovedUsers        []UserOp
	AddedUsers          []UserOp
	RemovedOutboundTags []string
	AddedOutbounds      [][]byte
	RoutingConfig       []byte // full new routing section; nil when unchanged
}

// UserOp is a per-user AlterInbound operation; User is nil for removals.
type UserOp struct {
	Tag      string
	Protocol string
	Email    string
	User     map[string]any
}

// Empty reports whether the diff contains no operations.
func (d *HotDiff) Empty() bool {
	return len(d.RemovedInboundTags) == 0 &&
		len(d.AddedInbounds) == 0 &&
		len(d.RemovedUsers) == 0 &&
		len(d.AddedUsers) == 0 &&
		len(d.RemovedOutboundTags) == 0 &&
		len(d.AddedOutbounds) == 0 &&
		d.RoutingConfig == nil
}

// ComputeHotDiff compares two generated configs and returns the API operations
// that transform a running instance from oldCfg to newCfg. ok is false when
// the change touches anything that has no runtime reload API (log, dns,
// policy, ...) and therefore requires a full process restart.
func ComputeHotDiff(oldCfg, newCfg *Config) (*HotDiff, bool) {
	if oldCfg == nil || newCfg == nil {
		return nil, false
	}

	// Sections without a reload API must be semantically identical.
	// Comparison is whitespace-insensitive: a template save that merely
	// reformats the JSON (frontend textarea, API clients) must not be
	// mistaken for a real change that forces a restart.
	static := []struct {
		name     string
		old, new json_util.RawMessage
	}{
		{"log", oldCfg.LogConfig, newCfg.LogConfig},
		{"dns", oldCfg.DNSConfig, newCfg.DNSConfig},
		{"transport", oldCfg.Transport, newCfg.Transport},
		{"policy", oldCfg.Policy, newCfg.Policy},
		{"api", oldCfg.API, newCfg.API},
		{"stats", oldCfg.Stats, newCfg.Stats},
		{"reverse", oldCfg.Reverse, newCfg.Reverse},
		{"fakedns", oldCfg.FakeDNS, newCfg.FakeDNS},
		{"observatory", oldCfg.Observatory, newCfg.Observatory},
		{"burstObservatory", oldCfg.BurstObservatory, newCfg.BurstObservatory},
		{"metrics", oldCfg.Metrics, newCfg.Metrics},
		{"geodata", oldCfg.Geodata, newCfg.Geodata},
	}
	for _, section := range static {
		if !rawEqualNormalized(section.old, section.new) {
			logger.Debug("hot diff: section [", section.name, "] changed and has no reload API")
			return nil, false
		}
	}

	diff := &HotDiff{}

	if ok := diffInbounds(oldCfg, newCfg, diff); !ok {
		logger.Debug("hot diff: inbound change is not API-applicable")
		return nil, false
	}
	if ok := diffOutbounds(oldCfg, newCfg, diff); !ok {
		logger.Debug("hot diff: outbound change is not API-applicable (default outbound or tags)")
		return nil, false
	}
	if ok := diffRouting(oldCfg, newCfg, diff); !ok {
		logger.Debug("hot diff: routing change is not API-applicable (domainStrategy or section shape)")
		return nil, false
	}

	return diff, true
}

// diffInbounds fills diff with inbound removals/additions (a changed inbound
// becomes remove+add). The api inbound carries the gRPC server the panel is
// talking through, so any change touching it forces a restart.
func diffInbounds(oldCfg, newCfg *Config, diff *HotDiff) bool {
	oldByTag, ok := inboundsByTag(oldCfg.InboundConfigs)
	if !ok {
		return false
	}
	newByTag, ok := inboundsByTag(newCfg.InboundConfigs)
	if !ok {
		return false
	}

	apiTag := apiTagFromConfig(newCfg.API)

	for i := range oldCfg.InboundConfigs {
		oldIb := &oldCfg.InboundConfigs[i]
		newIb, exists := newByTag[oldIb.Tag]
		if exists && inboundEqualNormalized(oldIb, newIb) {
			continue
		}
		if oldIb.Tag == apiTag || oldIb.Tag == "api" {
			return false
		}
		if exists && (inboundHasReverseClient(oldIb) || inboundHasReverseClient(newIb)) {
			logger.Debug("hot diff: inbound [", oldIb.Tag, "] carries a reverse-tagged client, forcing a full restart instead of a hot swap")
			return false
		}
		if exists && diffInboundUsers(oldIb, newIb, diff) {
			continue
		}
		diff.RemovedInboundTags = append(diff.RemovedInboundTags, oldIb.Tag)
		if exists {
			raw, err := json.Marshal(newIb)
			if err != nil {
				return false
			}
			diff.AddedInbounds = append(diff.AddedInbounds, raw)
		}
	}
	for i := range newCfg.InboundConfigs {
		newIb := &newCfg.InboundConfigs[i]
		if _, exists := oldByTag[newIb.Tag]; exists {
			continue
		}
		if newIb.Tag == apiTag || newIb.Tag == "api" {
			return false
		}
		raw, err := json.Marshal(newIb)
		if err != nil {
			return false
		}
		diff.AddedInbounds = append(diff.AddedInbounds, raw)
	}
	return true
}

var userDiffableProtocols = map[string]struct{}{"vless": {}, "vmess": {}, "trojan": {}}

// diffInboundUsers emits per-user AlterInbound ops when two same-tag inbounds
// differ only in settings.clients, so the handler (and its listener) survives.
func diffInboundUsers(oldIb, newIb *InboundConfig, diff *HotDiff) bool {
	if oldIb.Port != newIb.Port || oldIb.Protocol != newIb.Protocol || oldIb.Tag != newIb.Tag {
		return false
	}
	if _, ok := userDiffableProtocols[oldIb.Protocol]; !ok {
		return false
	}
	if !rawEqualNormalized(oldIb.Listen, newIb.Listen) ||
		!rawEqualNormalized(oldIb.StreamSettings, newIb.StreamSettings) ||
		!rawEqualNormalized(oldIb.Sniffing, newIb.Sniffing) {
		return false
	}
	oldClients, oldRest, ok := splitSettingsClients(oldIb.Settings)
	if !ok {
		return false
	}
	newClients, newRest, ok := splitSettingsClients(newIb.Settings)
	if !ok {
		return false
	}
	if !bytes.Equal(oldRest, newRest) {
		return false
	}
	for email, oldC := range oldClients {
		newC, exists := newClients[email]
		if exists && bytes.Equal(oldC.norm, newC.norm) {
			continue
		}
		diff.RemovedUsers = append(diff.RemovedUsers, UserOp{Tag: oldIb.Tag, Protocol: oldIb.Protocol, Email: email})
		if exists {
			diff.AddedUsers = append(diff.AddedUsers, UserOp{Tag: oldIb.Tag, Protocol: oldIb.Protocol, Email: email, User: newC.user})
		}
	}
	for email, newC := range newClients {
		if _, exists := oldClients[email]; !exists {
			diff.AddedUsers = append(diff.AddedUsers, UserOp{Tag: oldIb.Tag, Protocol: oldIb.Protocol, Email: email, User: newC.user})
		}
	}
	return true
}

type clientEntry struct {
	user map[string]any
	norm []byte
}

// splitSettingsClients indexes settings.clients by email and returns the rest of
// the settings in canonical form; ok is false when a client has no unique email.
func splitSettingsClients(raw json_util.RawMessage) (map[string]clientEntry, []byte, bool) {
	if len(raw) == 0 {
		return nil, nil, false
	}
	settings := map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&settings); err != nil {
		return nil, nil, false
	}
	clientsRaw, hasClients := settings["clients"].([]any)
	if !hasClients {
		return nil, nil, false
	}
	clients := make(map[string]clientEntry, len(clientsRaw))
	for _, c := range clientsRaw {
		obj, ok := c.(map[string]any)
		if !ok {
			return nil, nil, false
		}
		email, _ := obj["email"].(string)
		if email == "" {
			return nil, nil, false
		}
		if _, dup := clients[email]; dup {
			return nil, nil, false
		}
		norm, err := json.Marshal(obj)
		if err != nil {
			return nil, nil, false
		}
		clients[email] = clientEntry{user: obj, norm: norm}
	}
	delete(settings, "clients")
	rest, err := json.Marshal(settings)
	if err != nil {
		return nil, nil, false
	}
	return clients, rest, true
}

func inboundHasReverseClient(ib *InboundConfig) bool {
	if ib == nil {
		return false
	}
	var settings struct {
		Clients []struct {
			Reverse json.RawMessage `json:"reverse"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(ib.Settings, &settings); err != nil {
		return false
	}
	for _, c := range settings.Clients {
		if len(c.Reverse) == 0 {
			continue
		}
		var tag any
		if err := json.Unmarshal(c.Reverse, &tag); err != nil || tag == nil {
			continue
		}
		return true
	}
	return false
}

// diffOutbounds fills diff with outbound removals/additions keyed by tag.
// The first outbound is xray's default handler and the API can only append,
// so any change to its identity or content forces a restart. Reordering of
// the remaining outbounds is ignored — routing addresses them by tag.
func diffOutbounds(oldCfg, newCfg *Config, diff *HotDiff) bool {
	oldOut, ok := parseOutbounds(oldCfg.OutboundConfigs)
	if !ok {
		return false
	}
	newOut, ok := parseOutbounds(newCfg.OutboundConfigs)
	if !ok {
		return false
	}

	if (len(oldOut) == 0) != (len(newOut) == 0) {
		return false
	}
	if len(oldOut) > 0 {
		if oldOut[0].tag != newOut[0].tag || !bytes.Equal(oldOut[0].norm, newOut[0].norm) {
			return false
		}
	}

	oldByTag := make(map[string]outboundEntry, len(oldOut))
	for _, e := range oldOut {
		oldByTag[e.tag] = e
	}
	newByTag := make(map[string]outboundEntry, len(newOut))
	for _, e := range newOut {
		newByTag[e.tag] = e
	}

	for _, oldE := range oldOut {
		newE, exists := newByTag[oldE.tag]
		if exists && bytes.Equal(oldE.norm, newE.norm) {
			continue
		}
		diff.RemovedOutboundTags = append(diff.RemovedOutboundTags, oldE.tag)
		if exists {
			diff.AddedOutbounds = append(diff.AddedOutbounds, newE.raw)
		}
	}
	for _, newE := range newOut {
		if _, exists := oldByTag[newE.tag]; !exists {
			diff.AddedOutbounds = append(diff.AddedOutbounds, newE.raw)
		}
	}
	return true
}

// diffRouting decides whether the routing change is limited to rules and
// balancers — the only parts RoutingService.AddRule can replace at runtime.
// domainStrategy/domainMatcher and any other key in the section are fixed at
// process start.
func diffRouting(oldCfg, newCfg *Config, diff *HotDiff) bool {
	if bytes.Equal(oldCfg.RouterConfig, newCfg.RouterConfig) {
		return true
	}
	// No routing section at start likely means no router feature (and no
	// RoutingService) in the running instance — only a restart can add it.
	if len(oldCfg.RouterConfig) == 0 || len(newCfg.RouterConfig) == 0 {
		return false
	}
	oldRest, ok := routingWithoutReloadable(oldCfg.RouterConfig)
	if !ok {
		return false
	}
	newRest, ok := routingWithoutReloadable(newCfg.RouterConfig)
	if !ok {
		return false
	}
	if !bytes.Equal(oldRest, newRest) {
		return false
	}
	diff.RoutingConfig = newCfg.RouterConfig
	return true
}

// routingWithoutReloadable returns the routing section normalized with the
// runtime-reloadable keys removed, for comparing the restart-only remainder.
func routingWithoutReloadable(raw []byte) ([]byte, bool) {
	parsed := map[string]any{}
	if len(raw) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&parsed); err != nil {
			return nil, false
		}
	}
	delete(parsed, "rules")
	delete(parsed, "balancers")
	out, err := json.Marshal(parsed)
	if err != nil {
		return nil, false
	}
	return out, true
}

// inboundEqualNormalized compares two inbounds ignoring JSON formatting in
// their raw sections, so a reformatted template does not read as a changed
// inbound.
func inboundEqualNormalized(a, b *InboundConfig) bool {
	return a.Port == b.Port &&
		a.Protocol == b.Protocol &&
		a.Tag == b.Tag &&
		rawEqualNormalized(a.Listen, b.Listen) &&
		rawEqualNormalized(a.Settings, b.Settings) &&
		rawEqualNormalized(a.StreamSettings, b.StreamSettings) &&
		rawEqualNormalized(a.Sniffing, b.Sniffing)
}

// rawEqualNormalized reports whether two raw JSON values are semantically
// equal: whitespace, object key order and an explicit `null` versus an
// absent section are all ignored. UI editors rebuild objects on save (new
// key order) and emit `null` for switched-off sections — none of that is a
// reason to restart the core. Number precision is preserved via json.Number,
// so genuinely different values never compare equal. Unparsable values only
// compare equal byte-for-byte.
func rawEqualNormalized(a, b json_util.RawMessage) bool {
	if bytes.Equal(a, b) {
		return true
	}
	na, ok := canonicalJSON(a)
	if !ok {
		return false
	}
	nb, ok := canonicalJSON(b)
	if !ok {
		return false
	}
	return bytes.Equal(na, nb)
}

// canonicalJSON renders a JSON value in canonical form: sorted object keys,
// no insignificant whitespace, exact number digits (json.Number). Empty
// input and JSON null both canonicalize to nil.
func canonicalJSON(raw json_util.RawMessage) ([]byte, bool) {
	if len(raw) == 0 {
		return nil, true
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	if value == nil {
		return nil, true
	}
	out, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	return out, true
}

// inboundsByTag indexes inbounds by tag; ok is false when a tag is empty or
// duplicated, since such handlers can't be addressed through the API.
func inboundsByTag(inbounds []InboundConfig) (map[string]*InboundConfig, bool) {
	byTag := make(map[string]*InboundConfig, len(inbounds))
	for i := range inbounds {
		tag := inbounds[i].Tag
		if tag == "" {
			return nil, false
		}
		if _, dup := byTag[tag]; dup {
			return nil, false
		}
		byTag[tag] = &inbounds[i]
	}
	return byTag, true
}

type outboundEntry struct {
	tag  string
	raw  []byte // original JSON, used for AddOutbound
	norm []byte // canonical JSON, used for change detection
}

// parseOutbounds splits the outbounds array into per-entry raw/normalized
// JSON. ok is false when the array is unparsable or an entry has an empty or
// duplicate tag — those can't be addressed through the API.
func parseOutbounds(raw json_util.RawMessage) ([]outboundEntry, bool) {
	if len(raw) == 0 {
		return nil, true
	}
	var elems []json.RawMessage
	if err := json.Unmarshal(raw, &elems); err != nil {
		return nil, false
	}
	entries := make([]outboundEntry, 0, len(elems))
	seen := make(map[string]struct{}, len(elems))
	for _, elem := range elems {
		var meta struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(elem, &meta); err != nil {
			return nil, false
		}
		if meta.Tag == "" {
			return nil, false
		}
		if _, dup := seen[meta.Tag]; dup {
			return nil, false
		}
		seen[meta.Tag] = struct{}{}
		norm, ok := canonicalJSON(json_util.RawMessage(elem))
		if !ok {
			return nil, false
		}
		entries = append(entries, outboundEntry{tag: meta.Tag, raw: elem, norm: norm})
	}
	return entries, true
}

// apiTagFromConfig extracts api.tag from the api section, defaulting to "api".
func apiTagFromConfig(api json_util.RawMessage) string {
	var parsed struct {
		Tag string `json:"tag"`
	}
	if len(api) > 0 && json.Unmarshal(api, &parsed) == nil && parsed.Tag != "" {
		return parsed.Tag
	}
	return "api"
}
