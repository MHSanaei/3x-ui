package mtproto

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestInstanceFromInbound(t *testing.T) {
	aliceSecret := "ee0123456789abcdef0123456789abcdef6578616d706c652e636f6d"
	ib := &model.Inbound{
		Id:       3,
		Tag:      "inbound-3",
		Listen:   "0.0.0.0",
		Port:     8443,
		Protocol: model.MTProto,
		Settings: `{"fakeTlsDomain":"example.com",` +
			`"debug":true,"proxyProtocolListener":true,"preferIp":"prefer-ipv4",` +
			`"domainFronting":{"ip":"127.0.0.1","port":9443,"proxyProtocol":true},` +
			`"throttleMaxConnections":5000,` +
			`"routeThroughXray":true,"routeXrayPort":50000,` +
			`"adTag":" 0123456789abcdef0123456789abcdef ",` +
			`"clients":[` +
			`{"email":"alice","secret":"` + aliceSecret + `","adTag":"fedcba9876543210fedcba9876543210","enable":true,"totalGB":1073741824,"expiryTime":1893456000000},` +
			`{"email":"bob","secret":"","enable":true},` +
			`{"email":"carol","secret":"eeaa","enable":false}]}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if len(inst.Secrets) != 1 {
		t.Fatalf("only the enabled client with a secret should be served, got %d: %+v", len(inst.Secrets), inst.Secrets)
	}
	if inst.Secrets[0].Name != "alice" {
		t.Fatalf("secret name should be the client email, got %q", inst.Secrets[0].Name)
	}
	if inst.Secrets[0].Secret != aliceSecret {
		t.Fatalf("a valid secret must be preserved, got %q", inst.Secrets[0].Secret)
	}
	if inst.Secrets[0].AdTag != "fedcba9876543210fedcba9876543210" {
		t.Fatalf("the client ad-tag must be parsed, got %q", inst.Secrets[0].AdTag)
	}
	if inst.Secrets[0].QuotaBytes != 1073741824 {
		t.Fatalf("totalGB must map to the byte quota, got %d", inst.Secrets[0].QuotaBytes)
	}
	if inst.Secrets[0].ExpiresUnix != 1893456000 {
		t.Fatalf("expiryTime (ms) must map to a unix-second deadline, got %d", inst.Secrets[0].ExpiresUnix)
	}
	if inst.Port != 8443 || inst.Id != 3 {
		t.Fatalf("bad instance %+v", inst)
	}
	if !inst.Debug || !inst.ProxyProtocolListener || inst.PreferIP != "prefer-ipv4" {
		t.Fatalf("scalar options not parsed: %+v", inst)
	}
	if inst.FrontingIP != "127.0.0.1" || inst.FrontingPort != 9443 || !inst.FrontingProxyProtocol {
		t.Fatalf("domain-fronting not parsed: %+v", inst)
	}
	if inst.ThrottleMaxConnections != 5000 {
		t.Fatalf("throttle not parsed: %+v", inst)
	}
	if !inst.RouteThroughXray || inst.XrayRoutePort != 50000 {
		t.Fatalf("xray routing not parsed: %+v", inst)
	}

	if _, ok := InstanceFromInbound(&model.Inbound{Protocol: model.VLESS}); ok {
		t.Fatal("non-mtproto inbound should not produce an instance")
	}

	noSecrets := &model.Inbound{Protocol: model.MTProto, Settings: `{"clients":[{"email":"x","secret":"","enable":true}]}`}
	if _, ok := InstanceFromInbound(noSecrets); ok {
		t.Fatal("an inbound with no active secret should not produce an instance")
	}

	badTags := &model.Inbound{Protocol: model.MTProto, Settings: `{"clients":[{"email":"x","secret":"ee00","adTag":"deadbeef","enable":true}]}`}
	badInst, ok := InstanceFromInbound(badTags)
	if !ok {
		t.Fatal("expected a usable instance despite a malformed ad tag")
	}
	if badInst.Secrets[0].AdTag != "" {
		t.Fatalf("a malformed ad tag must be dropped so the generated config stays valid, got %q", badInst.Secrets[0].AdTag)
	}
}

func TestRenderConfig(t *testing.T) {
	// A bare instance emits only the required keys, api-bind-to, and the
	// [secrets] section, with no optional keys and no [domain-fronting].
	bare := renderConfig(Instance{
		Secrets: []SecretEntry{{Name: "alice", Secret: "ee00"}},
		Listen:  "0.0.0.0", Port: 8443,
	}, 5000, "")
	for _, unwanted := range []string{"debug", "proxy-protocol-listener", "prefer-ip", "[domain-fronting]", "[stats.prometheus]", "[throttle]", "[secret-ad-tags]", "api-token"} {
		if strings.Contains(bare, unwanted) {
			t.Fatalf("bare config should not contain %q:\n%s", unwanted, bare)
		}
	}
	if !strings.Contains(bare, `bind-to = "0.0.0.0:8443"`) {
		t.Fatalf("missing bind-to:\n%s", bare)
	}
	if !strings.Contains(bare, `api-bind-to = "127.0.0.1:5000"`) {
		t.Fatalf("api-bind-to must always be present:\n%s", bare)
	}
	if !strings.Contains(bare, "[secrets]") || !strings.Contains(bare, `"alice" = "ee00"`) {
		t.Fatalf("secrets block must carry the client secret:\n%s", bare)
	}

	// A fully configured instance emits every option, the fronting section (as
	// host, not the fork-deprecated ip), the throttle block, the per-client
	// ad-tag overrides, and [secrets] last.
	full := renderConfig(Instance{
		Secrets: []SecretEntry{
			{Name: "alice", Secret: "ee11"},
			{Name: "bob", Secret: "ee22", AdTag: "fedcba9876543210fedcba9876543210"},
		},
		Listen: "0.0.0.0", Port: 443,
		Debug: true, ProxyProtocolListener: true, PreferIP: "only-ipv6",
		FrontingIP: "127.0.0.1", FrontingPort: 9443, FrontingProxyProtocol: true,
		ThrottleMaxConnections: 5000,
		PublicIPv4:             "1.2.3.4",
		PublicIPv6:             "2001:db8::1",
	}, 6000, "sesame")
	for _, want := range []string{
		"debug = true\n",
		"proxy-protocol-listener = true\n",
		`prefer-ip = "only-ipv6"`,
		`api-token = "sesame"`,
		`public-ipv4 = "1.2.3.4"`,
		`public-ipv6 = "2001:db8::1"`,
		"[domain-fronting]",
		`host = "127.0.0.1"`,
		"port = 9443",
		"proxy-protocol = true\n",
		"[throttle]",
		"max-connections = 5000",
		"[secret-ad-tags]",
		`"bob" = "fedcba9876543210fedcba9876543210"`,
	} {
		if !strings.Contains(full, want) {
			t.Fatalf("full config missing %q:\n%s", want, full)
		}
	}
	if strings.Contains(full, `ip = "127.0.0.1"`) {
		t.Fatalf("domain-fronting must use host, not the deprecated ip key:\n%s", full)
	}
	if strings.Contains(full, "ad-tag =") {
		t.Fatalf("no global ad-tag must be emitted, tags are per client:\n%s", full)
	}
	if strings.Contains(full, `"alice" = "0123456789abcdef0123456789abcdef"`) || strings.Contains(full, `"alice" = ""`) {
		t.Fatalf("a client without a tag must not appear in [secret-ad-tags]:\n%s", full)
	}
	// TOML requires top-level keys before any [section] header, and [secrets]
	// must be the final section so trailing keys are not swallowed by a table.
	if strings.Index(full, "prefer-ip") > strings.Index(full, "[domain-fronting]") {
		t.Fatalf("top-level keys must precede the [domain-fronting] section:\n%s", full)
	}
	if strings.LastIndex(full, "[secrets]") < strings.Index(full, "[domain-fronting]") {
		t.Fatalf("[secrets] must be the final section:\n%s", full)
	}
	if strings.LastIndex(full, "[secrets]") < strings.Index(full, "[throttle]") {
		t.Fatalf("[throttle] must precede [secrets]:\n%s", full)
	}
	if strings.LastIndex(full, "[secrets]") < strings.Index(full, "[secret-ad-tags]") {
		t.Fatalf("[secret-ad-tags] must precede [secrets]:\n%s", full)
	}
}

func TestRenderConfigXrayEgress(t *testing.T) {
	// Routing through Xray emits a [network] proxies upstream pointing at the
	// loopback SOCKS bridge, before the [secrets] section.
	routed := renderConfig(Instance{
		Secrets: []SecretEntry{{Name: "a", Secret: "ee22"}},
		Listen:  "0.0.0.0", Port: 443,
		RouteThroughXray: true, XrayRoutePort: 50000,
	}, 7000, "")
	if !strings.Contains(routed, "[network]") ||
		!strings.Contains(routed, `proxies = ["socks5://127.0.0.1:50000"]`) {
		t.Fatalf("routed config must emit the SOCKS upstream:\n%s", routed)
	}
	if strings.Index(routed, "[network]") > strings.Index(routed, "[secrets]") {
		t.Fatalf("[network] must precede [secrets]:\n%s", routed)
	}

	// Without the flag (or without a port) the section is omitted.
	for _, inst := range []Instance{
		{Secrets: []SecretEntry{{Name: "a", Secret: "ee"}}, Listen: "0.0.0.0", Port: 443},
		{Secrets: []SecretEntry{{Name: "a", Secret: "ee"}}, Listen: "0.0.0.0", Port: 443, RouteThroughXray: true},
	} {
		if got := renderConfig(inst, 7000, ""); strings.Contains(got, "[network]") {
			t.Fatalf("unrouted config must omit [network]:\n%s", got)
		}
	}
}

func TestRenderConfigSecretLimits(t *testing.T) {
	cfg := renderConfig(Instance{
		Secrets: []SecretEntry{
			{Name: "alice", Secret: "ee11", QuotaBytes: 1073741824, ExpiresUnix: 1893456000},
			{Name: "bob", Secret: "ee22", QuotaBytes: 500},
			{Name: "carol", Secret: "ee33"},
		},
		Listen: "0.0.0.0", Port: 443,
	}, 6000, "")

	for _, want := range []string{
		"[secret-limits.\"alice\"]",
		`quota = "1073741824B"`,
		`expires = "2030-01-01T00:00:00Z"`,
		"[secret-limits.\"bob\"]",
		`quota = "500B"`,
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("limits config missing %q:\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "[secret-limits.\"carol\"]") {
		t.Fatalf("a client without a limit must not get a table:\n%s", cfg)
	}
	if strings.LastIndex(cfg, "[secrets]") < strings.Index(cfg, "[secret-limits.\"alice\"]") {
		t.Fatalf("[secret-limits] must precede the final [secrets] section:\n%s", cfg)
	}
	if strings.Contains(cfg, "usage-state-file") {
		t.Fatalf("no usage-state-file must be emitted; quota is an in-memory guard:\n%s", cfg)
	}
}

func TestSecretsPayloadLimits(t *testing.T) {
	body := secretsPayload(Instance{Secrets: []SecretEntry{
		{Name: "alice", Secret: "ee11", QuotaBytes: 1073741824, ExpiresUnix: 1893456000},
		{Name: "bob", Secret: "ee22"},
	}})
	if got := body.Secrets["alice"].Quota; got != "1073741824B" {
		t.Fatalf("quota must be sent as an exact byte count, got %q", got)
	}
	if got := body.Secrets["alice"].Expires; got != "2030-01-01T00:00:00Z" {
		t.Fatalf("expiry must be sent as RFC3339, got %q", got)
	}
	if body.Secrets["bob"].Quota != "" || body.Secrets["bob"].Expires != "" {
		t.Fatalf("an unlimited client must carry no quota/expiry: %+v", body.Secrets["bob"])
	}
}

func TestFingerprintSplit(t *testing.T) {
	base := Instance{Secrets: []SecretEntry{{Name: "a", Secret: "ee"}}, Listen: "0.0.0.0", Port: 443}

	for name, mutate := range map[string]func(*Instance){
		"debug":         func(i *Instance) { i.Debug = true },
		"listener":      func(i *Instance) { i.ProxyProtocolListener = true },
		"preferIp":      func(i *Instance) { i.PreferIP = "only-ipv4" },
		"frontingIP":    func(i *Instance) { i.FrontingIP = "127.0.0.1" },
		"frontingPort":  func(i *Instance) { i.FrontingPort = 9443 },
		"frontingProxy": func(i *Instance) { i.FrontingProxyProtocol = true },
		"throttle":      func(i *Instance) { i.ThrottleMaxConnections = 5000 },
		"routeXray":     func(i *Instance) { i.RouteThroughXray = true },
		"routeXrayPort": func(i *Instance) { i.XrayRoutePort = 50000 },
		"port":          func(i *Instance) { i.Port = 8443 },
		"listen":        func(i *Instance) { i.Listen = "127.0.0.1" },
		"publicIpv4":    func(i *Instance) { i.PublicIPv4 = "1.2.3.4" },
		"publicIpv6":    func(i *Instance) { i.PublicIPv6 = "2001:db8::1" },
	} {
		t.Run("structural/"+name, func(t *testing.T) {
			changed := base
			mutate(&changed)
			if base.structuralFingerprint() == changed.structuralFingerprint() {
				t.Fatalf("structural fingerprint must change when %s changes", name)
			}
			if base.secretsFingerprint() != changed.secretsFingerprint() {
				t.Fatalf("secrets fingerprint must stay put when %s changes", name)
			}
		})
	}

	for name, mutate := range map[string]func(*Instance){
		"add":    func(i *Instance) { i.Secrets = append(i.Secrets, SecretEntry{Name: "b", Secret: "ff"}) },
		"rekey":  func(i *Instance) { i.Secrets = []SecretEntry{{Name: "a", Secret: "ee99"}} },
		"remove": func(i *Instance) { i.Secrets = nil },
		"rename": func(i *Instance) { i.Secrets = []SecretEntry{{Name: "a2", Secret: "ee"}} },
		"clientAdTag": func(i *Instance) {
			i.Secrets = []SecretEntry{{Name: "a", Secret: "ee", AdTag: "0123456789abcdef0123456789abcdef"}}
		},
		"quota":  func(i *Instance) { i.Secrets = []SecretEntry{{Name: "a", Secret: "ee", QuotaBytes: 1 << 30}} },
		"expiry": func(i *Instance) { i.Secrets = []SecretEntry{{Name: "a", Secret: "ee", ExpiresUnix: 1893456000}} },
	} {
		t.Run("secrets/"+name, func(t *testing.T) {
			changed := base
			changed.Secrets = append([]SecretEntry(nil), base.Secrets...)
			mutate(&changed)
			if base.secretsFingerprint() == changed.secretsFingerprint() {
				t.Fatalf("secrets fingerprint must change on a %s", name)
			}
			if base.structuralFingerprint() != changed.structuralFingerprint() {
				t.Fatalf("structural fingerprint must stay put on a %s", name)
			}
		})
	}

	t.Run("orderInsensitive", func(t *testing.T) {
		forward := Instance{Secrets: []SecretEntry{{Name: "alice", Secret: "ee11"}, {Name: "bob", Secret: "ee22"}}}
		reversed := Instance{Secrets: []SecretEntry{{Name: "bob", Secret: "ee22"}, {Name: "alice", Secret: "ee11"}}}
		if got, want := forward.secretsFingerprint(), "alice=ee11;tag=;q=0;exp=0|bob=ee22;tag=;q=0;exp=0"; got != want {
			t.Fatalf("secrets fingerprint must join sorted pairs: got %q, want %q", got, want)
		}
		if forward.secretsFingerprint() != reversed.secretsFingerprint() {
			t.Fatal("secrets fingerprint must not depend on client order")
		}
	})
}
