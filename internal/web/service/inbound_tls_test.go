package service

import (
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
