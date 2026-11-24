package service

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestValidateInboundTLSCertificates(t *testing.T) {
	tests := []struct {
		name           string
		streamSettings string
		wantErr        bool
	}{
		{"empty stream", "", false},
		{"whitespace stream", " \t\n", false},
		{"none ignores stale TLS settings", `{"security":"none","tlsSettings":{"certificates":[{}]}}`, false},
		{"reality needs no TLS certificate", `{"security":"reality","realitySettings":{}}`, false},
		{"missing TLS settings", `{"security":"tls"}`, true},
		{"uppercase TLS security", `{"security":"TLS","tlsSettings":{}}`, true},
		{"mixed-case TLS security", `{"security":"Tls","tlsSettings":{}}`, true},
		{"null TLS settings", `{"security":"tls","tlsSettings":null}`, true},
		{"missing certificates", `{"security":"tls","tlsSettings":{}}`, true},
		{"null certificates", `{"security":"tls","tlsSettings":{"certificates":null}}`, true},
		{"empty certificates", `{"security":"tls","tlsSettings":{"certificates":[]}}`, true},
		{"null certificate row", `{"security":"tls","tlsSettings":{"certificates":[null]}}`, true},
		{"empty default file fields", `{"security":"tls","tlsSettings":{"certificates":[{"certificateFile":"","keyFile":""}]}}`, true},
		{"empty default inline fields", `{"security":"tls","tlsSettings":{"certificates":[{"certificate":[],"key":[]}]}}`, true},
		{"whitespace file fields", `{"security":"tls","tlsSettings":{"certificates":[{"certificateFile":" \t","keyFile":" \n"}]}}`, true},
		{"whitespace inline certificate", `{"security":"tls","tlsSettings":{"certificates":[{"certificate":[" ","\t"],"key":["private key"]}]}}`, true},
		{"whitespace inline key", `{"security":"tls","tlsSettings":{"certificates":[{"certificate":["certificate"],"key":[" ","\n"]}]}}`, true},
		{"missing private key", `{"security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/node/cert.pem"}]}}`, true},
		{"missing certificate", `{"security":"tls","tlsSettings":{"certificates":[{"keyFile":"/node/key.pem"}]}}`, true},
		{"verify only", `{"security":"tls","tlsSettings":{"certificates":[{"usage":"verify","certificateFile":"/node/ca.pem"}]}}`, true},
		{"verify with private key still needs server certificate", `{"security":"tls","tlsSettings":{"certificates":[{"usage":"verify","certificateFile":"/node/ca.pem","keyFile":"/node/key.pem"}]}}`, true},
		{"issue needs private key", `{"security":"tls","tlsSettings":{"certificates":[{"usage":"issue","certificateFile":"/node/ca.pem"}]}}`, true},
		{"file credentials with default usage", `{"security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/node/cert.pem","keyFile":"/node/key.pem"}]}}`, false},
		{"inline credentials", `{"security":"tls","tlsSettings":{"certificates":[{"certificate":["certificate"],"key":["private key"]}]}}`, false},
		{"certificate file and inline key", `{"security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/node/cert.pem","key":["private key"]}]}}`, false},
		{"inline certificate and key file", `{"security":"tls","tlsSettings":{"certificates":[{"certificate":["certificate"],"keyFile":"/node/key.pem"}]}}`, false},
		{"encipherment usage", `{"security":"tls","tlsSettings":{"certificates":[{"usage":"encipherment","certificateFile":"/node/cert.pem","keyFile":"/node/key.pem"}]}}`, false},
		{"issue usage", `{"security":"tls","tlsSettings":{"certificates":[{"usage":"issue","certificateFile":"/node/ca.pem","keyFile":"/node/ca-key.pem"}]}}`, false},
		{"unknown usage defaults to encipherment like Xray", `{"security":"tls","tlsSettings":{"certificates":[{"usage":"custom","certificateFile":"/node/cert.pem","keyFile":"/node/key.pem"}]}}`, false},
		{"verify and server certificates", `{"security":"tls","tlsSettings":{"certificates":[{"usage":"verify","certificateFile":"/node/ca.pem"},{"certificateFile":"/node/cert.pem","keyFile":"/node/key.pem"}]}}`, false},
		{"verify usage is case insensitive", `{"security":"tls","tlsSettings":{"certificates":[{"usage":"VERIFY","certificateFile":"/node/ca.pem"},{"certificateFile":"/node/cert.pem","keyFile":"/node/key.pem"}]}}`, false},
		{"empty extra certificate row", `{"security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/node/cert.pem","keyFile":"/node/key.pem"},{}]}}`, true},
		{"empty extra verify certificate", `{"security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/node/cert.pem","keyFile":"/node/key.pem"},{"usage":"verify"}]}}`, true},
		{"whitespace certificate file overrides inline content", `{"security":"tls","tlsSettings":{"certificates":[{"certificateFile":" ","certificate":["certificate"],"key":["private key"]}]}}`, true},
		{"whitespace key file overrides inline content", `{"security":"tls","tlsSettings":{"certificates":[{"certificate":["certificate"],"keyFile":" ","key":["private key"]}]}}`, true},
		{"malformed stream", `{"security":"tls"`, true},
		{"malformed TLS settings", `{"security":"tls","tlsSettings":"invalid"}`, true},
		{"malformed certificate list", `{"security":"tls","tlsSettings":{"certificates":{}}}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInboundTLSCertificates(tt.streamSettings)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateInboundTLSCertificates() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateInboundTLSCertificatesIdentifiesIncompleteRow(t *testing.T) {
	err := validateInboundTLSCertificates(`{"security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/node/cert.pem","keyFile":"/node/key.pem"},{"certificateFile":"/node/other.pem"}]}}`)
	if err == nil || !strings.Contains(err.Error(), "TLS certificate 2") || !strings.Contains(err.Error(), "private key") {
		t.Fatalf("expected actionable error for the second certificate's private key, got %v", err)
	}
}

func TestAddInboundRejectsMissingTLSCertificates(t *testing.T) {
	setupConflictDB(t)
	mgr := useTestRuntimeManager(t)
	fake := &fakeNodeRuntime{}
	mgr.SetLocalRuntimeOverride(fake)

	inbound := &model.Inbound{
		Tag:            "tls-missing-44310",
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           44310,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"","keyFile":""}]}}`,
		Settings:       `{"clients":[]}`,
	}
	_, needRestart, err := (&InboundService{}).AddInbound(inbound)
	if err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("AddInbound: expected TLS validation error, got %v", err)
	}
	if needRestart {
		t.Fatal("AddInbound: rejected TLS configuration requested a restart")
	}
	var count int64
	if err := database.GetDB().Model(&model.Inbound{}).Count(&count).Error; err != nil {
		t.Fatalf("count inbounds: %v", err)
	}
	if count != 0 {
		t.Fatalf("AddInbound: rejected TLS configuration created %d rows", count)
	}
	if fake.addInbound.Load() != 0 || fake.updateInbound.Load() != 0 || fake.delInbound.Load() != 0 {
		t.Fatal("AddInbound: rejected TLS configuration reached the runtime")
	}
}

func TestUpdateInboundRejectsMissingTLSCertificates(t *testing.T) {
	setupConflictDB(t)
	mgr := useTestRuntimeManager(t)
	fake := &fakeNodeRuntime{}
	mgr.SetLocalRuntimeOverride(fake)

	seedInboundConflict(t, "tls-existing-44311", "0.0.0.0", 44311, model.VLESS,
		`{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/node/cert.pem","keyFile":"/node/key.pem"}]}}`, `{"clients":[]}`)
	var existing model.Inbound
	if err := database.GetDB().Where("tag = ?", "tls-existing-44311").First(&existing).Error; err != nil {
		t.Fatalf("load existing inbound: %v", err)
	}
	update := existing
	update.Remark = "must not be saved"
	update.Port = 44312
	update.StreamSettings = `{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/node/cert.pem"}]}}`
	_, needRestart, err := (&InboundService{}).UpdateInbound(&update)
	if err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("UpdateInbound: expected TLS validation error, got %v", err)
	}
	if needRestart {
		t.Fatal("UpdateInbound: rejected TLS configuration requested a restart")
	}
	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, existing.Id).Error; err != nil {
		t.Fatalf("reload existing inbound: %v", err)
	}
	if !reflect.DeepEqual(reloaded, existing) {
		t.Fatal("UpdateInbound: rejected TLS configuration changed the stored inbound")
	}
	if fake.addInbound.Load() != 0 || fake.updateInbound.Load() != 0 || fake.delInbound.Load() != 0 {
		t.Fatal("UpdateInbound: rejected TLS configuration reached the runtime")
	}
}

// The panel used to seed a TLS inbound with an all-empty certificate, so rows in
// that shape predate the guard and must stay editable — see UpdateInbound.
func TestUpdateInboundAllowsUntouchedLegacyTLSCertificates(t *testing.T) {
	const legacyStream = `{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"","keyFile":"","certificate":[],"key":[]}]}}`

	tests := []struct {
		name           string
		streamSettings string
	}{
		{"remark-only edit resends the stored block", legacyStream},
		{"node push re-encodes the same block", `{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"key":[],"certificate":[],"keyFile":"","certificateFile":""}]}}`},
		{"a partial fix to the stored credentials is tolerated", `{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/node/cert.pem"}]}}`},
		{"completing the credentials is accepted", `{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"/node/cert.pem","keyFile":"/node/key.pem"}]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupConflictDB(t)
			mgr := useTestRuntimeManager(t)
			mgr.SetLocalRuntimeOverride(&fakeNodeRuntime{})

			seedInboundConflict(t, "tls-legacy-44321", "0.0.0.0", 44321, model.VLESS, legacyStream, `{"clients":[]}`)
			var existing model.Inbound
			if err := database.GetDB().Where("tag = ?", "tls-legacy-44321").First(&existing).Error; err != nil {
				t.Fatalf("load legacy inbound: %v", err)
			}

			update := existing
			update.Remark = "renamed"
			update.StreamSettings = tt.streamSettings
			if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
				t.Fatalf("UpdateInbound: %v", err)
			}
			var reloaded model.Inbound
			if err := database.GetDB().First(&reloaded, existing.Id).Error; err != nil {
				t.Fatalf("reload inbound: %v", err)
			}
			if reloaded.Remark != "renamed" {
				t.Fatalf("UpdateInbound: remark = %q, want %q", reloaded.Remark, "renamed")
			}
		})
	}
}

// A push names any path on this host, and xray-core reads it whole in the panel process
// (a device never ends, /dev/zero never stops growing), so it is refused before storing.
func TestAddInboundRefusesACertificatePathThatIsNoRegularFile(t *testing.T) {
	setupConflictDB(t)
	mgr := useTestRuntimeManager(t)
	fake := &fakeNodeRuntime{}
	mgr.SetLocalRuntimeOverride(fake)

	dir := filepath.ToSlash(t.TempDir())
	inbound := &model.Inbound{
		Tag:            "tls-device-44311",
		Enable:         true,
		Listen:         "0.0.0.0",
		Port:           44311,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"certificates":[{"certificateFile":"` + dir + `","KeyFile":"` + dir + `"}]}}`,
		Settings:       `{"clients":[]}`,
	}
	_, _, err := (&InboundService{FromNodeSync: true}).AddInbound(inbound)
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("AddInbound = %v, want the certificate path refused", err)
	}
	var count int64
	if err := database.GetDB().Model(&model.Inbound{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("refused push stored %d row(s) (%v)", count, err)
	}
	if fake.addInbound.Load() != 0 {
		t.Fatal("refused push reached the runtime")
	}
}

// A node-sync token or the master's certificate pushes inbounds into this node's core,
// which runs as root: a push must not reach this host's files, sockets or own services.
func TestAddInboundRefusesPushedFieldsThatReachThisHost(t *testing.T) {
	setupConflictDB(t)
	mgr := useTestRuntimeManager(t)
	mgr.SetLocalRuntimeOverride(&fakeNodeRuntime{})
	push := &InboundService{FromNodeSync: true}
	if _, _, err := push.AddInbound(&model.Inbound{Tag: "web", Enable: true, Port: 44400, Protocol: model.VLESS, Settings: `{"clients":[]}`}); err != nil {
		t.Fatalf("seed a local inbound: %v", err)
	}
	vless := func(settings, stream string) *model.Inbound {
		return &model.Inbound{Enable: true, Port: 44401, Protocol: model.VLESS, Settings: settings, StreamSettings: stream}
	}
	for name, ib := range map[string]*model.Inbound{
		"file masquerade behind a repeated key": {
			Enable: true, Port: 44401, Protocol: model.Hysteria, Settings: `{"clients":[]}`,
			StreamSettings: `{"network":"hysteria","HysteriaSettings":{"masquerade":{"Type":"FILE","dir":"/"}},"hysteriaSettings":{"masquerade":{"type":"404"}}}`,
		},
		"masterKeyLog":               vless(`{"clients":[]}`, `{"network":"tcp","security":"tls","tlsSettings":{"masterKeyLog":"/root/.ssh/authorized_keys"}}`),
		"tun inbound":                {Enable: true, Port: 44401, Protocol: model.Protocol("tun"), Settings: `{}`},
		"fallback to the core API":   vless(`{"clients":[],"fallbacks":[{"dest":62789}]}`, `{"network":"tcp"}`),
		"REALITY target on loopback": vless(`{"clients":[]}`, `{"network":"tcp","security":"reality","realitySettings":{"target":"127.0.0.1:2053"}}`),
		"local xdrive storage":       vless(`{"clients":[]}`, `{"network":"xdrive","xdriveSettings":{"service":"local","remoteFolder":"/etc/x-ui"}}`),
	} {
		if _, _, err := push.AddInbound(ib); err == nil {
			t.Errorf("%s: pushed inbound accepted", name)
		}
	}
	for name, ib := range map[string]*model.Inbound{
		"fallback to another inbound here": vless(`{"clients":[],"fallbacks":[{"dest":44400}]}`, `{"network":"tcp"}`),
		"public REALITY target":            {Enable: true, Port: 44402, Protocol: model.VLESS, Settings: `{"clients":[]}`, StreamSettings: `{"network":"tcp","security":"reality","realitySettings":{"target":"www.example.com:443"}}`},
	} {
		if _, _, err := push.AddInbound(ib); err != nil && strings.Contains(err.Error(), "pushed") {
			t.Errorf("%s: refused: %v", name, err)
		}
	}
	s3 := xdriveInbound(44404, `{"service":"s3","remoteFolder":"x","secrets":["AKID","SECRET"],"s3":{"endpoint":"http://127.0.0.1:2053","region":"r","bucket":"b"}}`)
	if _, _, err := push.AddInbound(s3); err == nil || !strings.Contains(err.Error(), "may not set xdrive service") {
		t.Fatalf("pushed S3 storage on this host's own port = %v, want refused by the push guard", err)
	}
	if _, _, err := (&InboundService{}).AddInbound(&model.Inbound{Enable: true, Listen: "env:XUI_DB_DSN", Port: 44403, Protocol: model.VLESS, Settings: `{"clients":[]}`}); err == nil || !strings.Contains(err.Error(), "env:") {
		t.Fatalf("env: listen = %v, want refused for every caller", err)
	}
}

// What this node's own admin configured stays pushable: the master echoes the stored row
// back, and refusing it would stop that inbound syncing at all.
func TestUpdateInboundKeepsAHostReachTheNodeAlreadyStores(t *testing.T) {
	setupConflictDB(t)
	mgr := useTestRuntimeManager(t)
	mgr.SetLocalRuntimeOverride(&fakeNodeRuntime{})
	stream := `{"network":"hysteria","hysteriaSettings":{"masquerade":{"type":"proxy","url":"https://news.example.com"}}}`
	stored, _, err := (&InboundService{}).AddInbound(&model.Inbound{Tag: "hy", Enable: true, Port: 44410, Protocol: model.Hysteria, Settings: `{"clients":[]}`, StreamSettings: stream})
	if err != nil {
		t.Fatalf("admin add: %v", err)
	}
	push := &InboundService{FromNodeSync: true}
	echo := *stored
	if _, _, err := push.UpdateInbound(&echo); err != nil {
		t.Fatalf("pushing back the stored row: %v", err)
	}
	changed := *stored
	changed.StreamSettings = `{"network":"hysteria","hysteriaSettings":{"masquerade":{"type":"proxy","url":"http://127.0.0.1:2053"}}}`
	if _, _, err := push.UpdateInbound(&changed); err == nil {
		t.Fatal("a push moved the masquerade proxy to the panel's own port")
	}
}
