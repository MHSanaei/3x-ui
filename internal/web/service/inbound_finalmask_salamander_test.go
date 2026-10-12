package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func hysteriaSalamanderStream(password string) string {
	return `{"network":"hysteria","security":"tls","hysteriaSettings":{"version":2},
		"tlsSettings":{"alpn":["h3"],"certificates":[{"certificateFile":"/etc/ssl/h.crt","keyFile":"/etc/ssl/h.key"}]},
		"finalmask":{"udp":[{"type":"salamander","settings":{"password":"` + password + `"}}]}}`
}

func hysteriaInbound(port int, stream string) *model.Inbound {
	return &model.Inbound{
		Enable: true, Listen: "127.0.0.1", Port: port, Protocol: model.Hysteria,
		Settings:       `{"version":2,"clients":[{"email":"sal@example.test","auth":"a","enable":true}]}`,
		StreamSettings: stream,
	}
}

// xray-core loads a salamander mask whose password is under 4 bytes and refuses it only when the
// socket opens: on an inbound that stops every listener, so each save path must refuse it.
func TestSavePathsRefuseShortSalamanderPassword(t *testing.T) {
	const want = "salamander finalmask password must be at least 4 bytes"
	tests := []struct {
		name string
		save func(t *testing.T) error
	}{
		{"add inbound", func(t *testing.T) error {
			_, _, err := (&InboundService{}).AddInbound(hysteriaInbound(18443, hysteriaSalamanderStream("")))
			return err
		}},
		{"update inbound", func(t *testing.T) error {
			in, _, err := (&InboundService{}).AddInbound(hysteriaInbound(18444, hysteriaSalamanderStream("long-enough")))
			if err != nil {
				t.Fatalf("AddInbound: %v", err)
			}
			in.StreamSettings = hysteriaSalamanderStream("abc")
			_, _, err = (&InboundService{}).UpdateInbound(in)
			return err
		}},
		{"add host", func(t *testing.T) error {
			ib := mkInbound(t, 18445, model.VLESS, `{"clients":[]}`)
			_, err := (&HostService{}).AddHostGroup(&entity.HostGroup{
				InboundIds: []int{ib.Id}, Remark: "h", Port: 443,
				FinalMask: `{"udp":[{"type":"salamander","settings":{"password":"€"}}]}`,
			})
			return err
		}},
		{"subscription JSON finalmask", func(t *testing.T) error {
			s := &SettingService{}
			all, err := s.GetAllSetting()
			if err != nil {
				t.Fatalf("GetAllSetting: %v", err)
			}
			all.SubJsonFinalMask = `{"udp":[{"type":"salamander","settings":{"password":"ab"}}]}`
			return s.UpdateAllSetting(all, SecretClears{})
		}},
		{"xray template inbound", func(t *testing.T) error {
			return (&XraySettingService{}).SaveXraySetting(`{"inbounds":[{"tag":"hy","port":18446,"protocol":"hysteria",
				"settings":{"version":2,"clients":[]},"streamSettings":` + hysteriaSalamanderStream("") + `}],
				"outbounds":[{"protocol":"freedom","tag":"direct"}]}`)
		}},
		{"xray template outbound", func(t *testing.T) error {
			return (&XraySettingService{}).SaveXraySetting(`{"outbounds":[{"protocol":"freedom","tag":"direct"},
				{"protocol":"hysteria","tag":"hy-out","settings":{"version":2,"address":"hy.example.com","port":443},
				"streamSettings":{"network":"hysteria","security":"tls","hysteriaSettings":{"version":2,"auth":"a"},
					"finalmask":{"udp":[{"type":"salamander","settings":{"password":"abc"}}]}}}]}`)
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
