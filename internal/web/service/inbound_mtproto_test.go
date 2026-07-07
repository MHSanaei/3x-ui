package service

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestMtprotoRoutesThroughXray(t *testing.T) {
	cases := map[string]struct {
		ib   *model.Inbound
		want bool
	}{
		"routed":      {&model.Inbound{Protocol: model.MTProto, Settings: `{"routeThroughXray":true}`}, true},
		"off":         {&model.Inbound{Protocol: model.MTProto, Settings: `{"routeThroughXray":false}`}, false},
		"absent":      {&model.Inbound{Protocol: model.MTProto, Settings: `{}`}, false},
		"non-mtproto": {&model.Inbound{Protocol: model.VLESS, Settings: `{"routeThroughXray":true}`}, false},
		"bad json":    {&model.Inbound{Protocol: model.MTProto, Settings: `{nope`}, false},
		"nil":         {nil, false},
	}
	for name, c := range cases {
		if got := mtprotoRoutesThroughXray(c.ib); got != c.want {
			t.Fatalf("%s: got %v want %v", name, got, c.want)
		}
	}
}

func routeXrayPortOf(t *testing.T, settings string) int {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		t.Fatalf("settings not valid JSON: %v\n%s", err, settings)
	}
	return settingsRouteXrayPort(parsed)
}

func TestNormalizeMtprotoXrayPort(t *testing.T) {
	s := &InboundService{}

	// Non-mtproto inbounds are left alone.
	ib := &model.Inbound{Protocol: model.VLESS, Settings: `{"x":1}`}
	if err := s.normalizeMtprotoXrayPort(ib, ""); err != nil {
		t.Fatal(err)
	}
	if ib.Settings != `{"x":1}` {
		t.Fatalf("non-mtproto settings must be untouched, got %s", ib.Settings)
	}

	// Routing on with no existing port allocates a fresh one.
	ib = &model.Inbound{Protocol: model.MTProto, Settings: `{"routeThroughXray":true}`}
	if err := s.normalizeMtprotoXrayPort(ib, ""); err != nil {
		t.Fatal(err)
	}
	if p := routeXrayPortOf(t, ib.Settings); p <= 0 {
		t.Fatalf("a routed inbound must get a port, got %d", p)
	}

	// On update, the stored port wins over both a missing and a client-echoed
	// value — the backend owns it, so no churn and no client override.
	ib = &model.Inbound{Protocol: model.MTProto, Settings: `{"routeThroughXray":true,"routeXrayPort":99999}`}
	if err := s.normalizeMtprotoXrayPort(ib, `{"routeThroughXray":true,"routeXrayPort":51000}`); err != nil {
		t.Fatal(err)
	}
	if p := routeXrayPortOf(t, ib.Settings); p != 51000 {
		t.Fatalf("stored port must win, got %d", p)
	}

	// An already-present port (no old settings) is stable and not re-marshaled.
	const stable = `{"routeThroughXray":true,"routeXrayPort":52000}`
	ib = &model.Inbound{Protocol: model.MTProto, Settings: stable}
	if err := s.normalizeMtprotoXrayPort(ib, ""); err != nil {
		t.Fatal(err)
	}
	if ib.Settings != stable {
		t.Fatalf("stable settings must pass through untouched, got %s", ib.Settings)
	}

	// Turning routing off strips both the bridge port and the inert outbound.
	ib = &model.Inbound{Protocol: model.MTProto, Settings: `{"routeThroughXray":false,"routeXrayPort":53000,"outboundTag":"warp"}`}
	if err := s.normalizeMtprotoXrayPort(ib, ""); err != nil {
		t.Fatal(err)
	}
	if p := routeXrayPortOf(t, ib.Settings); p != 0 {
		t.Fatalf("disabling routing must drop the port, got %d", p)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["outboundTag"]; ok {
		t.Fatalf("disabling routing must drop the inert outbound tag, got %s", ib.Settings)
	}
}

func TestFillProtocolDefaultsMtproto(t *testing.T) {
	cs := &ClientService{}
	ib := &model.Inbound{Protocol: model.MTProto, Settings: `{"fakeTlsDomain":"example.com"}`}

	c := &model.Client{Email: "u"}
	if err := cs.fillProtocolDefaults(c, ib); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.Secret, "ee") || !strings.HasSuffix(c.Secret, hex.EncodeToString([]byte("example.com"))) {
		t.Fatalf("mtproto client should get a FakeTLS secret fronting the inbound domain, got %q", c.Secret)
	}

	// An existing secret is not overwritten.
	pre := &model.Client{Email: "v", Secret: "eepreset"}
	if err := cs.fillProtocolDefaults(pre, ib); err != nil {
		t.Fatal(err)
	}
	if pre.Secret != "eepreset" {
		t.Fatalf("an existing secret must be preserved, got %q", pre.Secret)
	}

	// With no inbound domain the default fronting host is used.
	c2 := &model.Client{Email: "w"}
	if err := cs.fillProtocolDefaults(c2, &model.Inbound{Protocol: model.MTProto, Settings: `{}`}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(c2.Secret, hex.EncodeToString([]byte(defaultMtprotoDomain))) {
		t.Fatalf("a domainless inbound should front the default host, got %q", c2.Secret)
	}
}

func TestNormalizeMtprotoSecretHealsClients(t *testing.T) {
	s := &InboundService{}
	ib := &model.Inbound{Protocol: model.MTProto, Settings: `{"fakeTlsDomain":"a.com","secret":"eedeadbeef","clients":[{"email":"x","secret":""}]}`}
	s.normalizeMtprotoSecret(ib)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil {
		t.Fatalf("healed settings not valid json: %v", err)
	}
	if _, ok := parsed["secret"]; ok {
		t.Fatalf("the vestigial inbound-level secret must be stripped, got %q", ib.Settings)
	}
	clients := parsed["clients"].([]any)
	got := clients[0].(map[string]any)["secret"].(string)
	if !strings.HasPrefix(got, "ee") || !strings.HasSuffix(got, hex.EncodeToString([]byte("a.com"))) {
		t.Fatalf("client secret should be healed to front the inbound domain, got %q", got)
	}
}
