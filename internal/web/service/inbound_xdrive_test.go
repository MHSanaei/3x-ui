package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func xdriveInbound(port int, xdriveSettings string) *model.Inbound {
	return &model.Inbound{
		Enable: true, Listen: "127.0.0.1", Port: port, Protocol: model.VLESS,
		Settings:       `{"clients":[],"decryption":"none"}`,
		StreamSettings: `{"network":"xdrive","security":"none","xdriveSettings":` + xdriveSettings + `}`,
	}
}

// xray-core opens XDRIVE storage when the listener starts; a storage it refuses there stops every inbound.
func TestAddInboundRefusesXDriveStorageTheCoreCannotOpen(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		want     string
	}{
		{"local without folder", `{"service":"local","remoteFolder":""}`, `XDRIVE needs a "remoteFolder"`},
		{"google drive without folder", `{"service":"Google Drive","remoteFolder":" ","secrets":["id","secret","token"]}`, `XDRIVE needs a "remoteFolder"`},
		{"google drive with two secrets", `{"service":"Google Drive","remoteFolder":"f","secrets":["id","secret"]}`, "Google Drive needs 3 secrets"},
		{"google drive with an empty secret", `{"service":"Google Drive","remoteFolder":"f","secrets":["id","","token"]}`, "Google Drive needs 3 secrets"},
		{"template without template", `{"service":"template","remoteFolder":"f"}`, `XDRIVE service "template" needs a "template" object`},
		{"template that is empty", `{"service":"template","remoteFolder":"f","template":{}}`, "XDRIVE template needs put, get, list and delete operations"},
		{"template that is not an object", `{"service":"template","remoteFolder":"f","template":"put"}`, "XDRIVE template is not a valid storage template"},
		{"template without a delete op", `{"service":"template","template":{"put":{"url":"u"},"get":{"url":"u"},"list":{"url":"u","namesRegex":"(.+)"}}}`, "XDRIVE template needs put, get, list and delete operations"},
		{"template without namesRegex", `{"service":"template","template":{"put":{"url":"u"},"get":{"url":"u"},"list":{"url":"u"},"delete":{"url":"u"}}}`, "XDRIVE template list needs a namesRegex"},
		{"template namesRegex without a group", `{"service":"template","template":{"put":{"url":"u"},"get":{"url":"u"},"list":{"url":"u","namesRegex":"name"},"delete":{"url":"u"}}}`, "XDRIVE template namesRegex needs one capture group"},
		{"unknown service", `{"service":"Dropbox","remoteFolder":"f"}`, `unsupported XDRIVE service "Dropbox"`},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			_, _, err := (&InboundService{}).AddInbound(xdriveInbound(18460+i, tc.settings))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("AddInbound err = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestAddInboundAcceptsACompleteXDriveTemplate(t *testing.T) {
	setupBulkDB(t)
	tmpl := `{"service":"template","template":{"put":{"url":"u"},"get":{"url":"u"},"list":{"url":"u","namesRegex":"\"name\":\"([^\"]+)\""},"delete":{"url":"u"}}}`
	if _, _, err := (&InboundService{}).AddInbound(xdriveInbound(18480, tmpl)); err != nil {
		t.Fatalf("AddInbound of a template xray-core can open: %v", err)
	}
}

func TestSavePathsRefuseXDriveWithoutStorage(t *testing.T) {
	const want = `XDRIVE needs a "remoteFolder"`
	const empty = `{"service":"local","remoteFolder":""}`
	tests := []struct {
		name string
		save func(t *testing.T) error
	}{
		{"update inbound", func(t *testing.T) error {
			in, _, err := (&InboundService{}).AddInbound(xdriveInbound(18470, `{"service":"local","remoteFolder":"/tmp/drive"}`))
			if err != nil {
				t.Fatalf("AddInbound of a complete XDRIVE inbound: %v", err)
			}
			in.StreamSettings = xdriveInbound(18470, empty).StreamSettings
			_, _, err = (&InboundService{}).UpdateInbound(in)
			return err
		}},
		{"xray template inbound", func(t *testing.T) error {
			return (&XraySettingService{}).SaveXraySetting(`{"inbounds":[{"tag":"drive","port":18471,"protocol":"vless",
				"settings":{"clients":[],"decryption":"none"},"streamSettings":` + xdriveInbound(18471, empty).StreamSettings + `}],
				"outbounds":[{"protocol":"freedom","tag":"direct"}]}`)
		}},
		{"xray template outbound", func(t *testing.T) error {
			return (&XraySettingService{}).SaveXraySetting(`{"outbounds":[{"protocol":"freedom","tag":"direct"},
				{"protocol":"vless","tag":"drive-out","settings":{"address":"drive.example","port":443,"id":"11111111-2222-4333-8444-555555555555","encryption":"none"},
				"streamSettings":` + xdriveInbound(0, empty).StreamSettings + `}]}`)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			if err := tc.save(t); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("save err = %v, want one containing %q", err, want)
			}
		})
	}
}
