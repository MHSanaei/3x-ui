package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// configFromInbounds builds the config the way the panel does, from raw JSON:
// the probe is only worth anything if it parses what is really written.
func configFromInbounds(t *testing.T, inbounds string) *xray.Config {
	t.Helper()
	var cfg xray.Config
	if err := json.Unmarshal([]byte(`{"inbounds":[`+inbounds+`]}`), &cfg); err != nil {
		t.Fatalf("build config: %v", err)
	}
	return &cfg
}

func TestBindConflicts(t *testing.T) {
	const (
		relay = `{"listen":"127.0.0.1","port":65101,"protocol":"socks","tag":"relay","settings":{"auth":"password","udp":true,"accounts":[]}}`
		user  = `{"listen":"0.0.0.0","port":65101,"protocol":"vless","tag":"user","streamSettings":{"network":"tcp"}}`
	)

	cases := []struct {
		name     string
		inbounds string
		running  string
		want     int
	}{
		{
			"same listen, port and tcp",
			`{"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"a"},
			 {"listen":"0.0.0.0","port":443,"protocol":"vmess","tag":"b"}`,
			``, 1,
		},
		{
			"tcp and udp on one port are legal",
			`{"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"a"},
			 {"listen":"0.0.0.0","port":443,"protocol":"hysteria","tag":"b"}`,
			``, 0,
		},
		{
			"kcp moves vless to udp and frees the port",
			`{"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"a","streamSettings":{"network":"kcp"}},
			 {"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"b","streamSettings":{"network":"tcp"}}`,
			``, 0,
		},
		{
			"ipv4 and ipv6 wildcards are separate sockets",
			`{"listen":"::","port":443,"protocol":"vless","tag":"ipv6"},
			 {"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"ipv4"}`,
			``, 0,
		},
		{
			"ipv4 wildcard still overlaps an ipv4 specific listen",
			`{"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"wildcard"},
			 {"listen":"192.0.2.10","port":443,"protocol":"vless","tag":"specific"}`,
			``, 1,
		},
		{
			"ipv6 wildcard still overlaps an ipv6 specific listen",
			`{"listen":"::","port":443,"protocol":"vless","tag":"wildcard"},
			 {"listen":"2001:db8::10","port":443,"protocol":"vless","tag":"specific"}`,
			``, 1,
		},
		{
			"wildcard listen overlaps a loopback one",
			`{"listen":"0.0.0.0","port":8443,"protocol":"vless","tag":"a"},
			 {"listen":"127.0.0.1","port":8443,"protocol":"trojan","tag":"b"}`,
			``, 1,
		},
		{
			"absent listen means wildcard",
			`{"port":8443,"protocol":"vless","tag":"a"},
			 {"listen":"127.0.0.1","port":8443,"protocol":"trojan","tag":"b"}`,
			``, 1,
		},
		{
			"distinct loopback addresses do not overlap",
			`{"listen":"127.0.0.1","port":8443,"protocol":"vless","tag":"a"},
			 {"listen":"127.0.0.2","port":8443,"protocol":"vless","tag":"b"}`,
			``, 0,
		},
		{
			"port zero is not a bind",
			`{"listen":"0.0.0.0","port":0,"protocol":"tunnel","tag":"a"},
			 {"listen":"0.0.0.0","port":0,"protocol":"vless","tag":"b"}`,
			``, 0,
		},
		{
			"clean config",
			`{"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"a"},
			 {"listen":"127.0.0.1","port":62789,"protocol":"tunnel","tag":"api"},
			 ` + relay,
			``, 0,
		},
		{
			// The relay the AmneziaWG family lives on: loopback tcp+udp, on the
			// same port as a user inbound's tcp.
			"amneziawg relay against a tcp inbound",
			relay + "," + user, ``, 1,
		},
		{
			"amneziawg relay against a udp inbound",
			relay + `,{"listen":"0.0.0.0","port":65101,"protocol":"hysteria","tag":"user"}`,
			``, 1,
		},
		{
			// Proves the relay's transports come from settings.udp and not from a
			// blanket "loopback owns everything" rule.
			"socks bridge without udp coexists with a udp inbound",
			`{"listen":"127.0.0.1","port":65101,"protocol":"socks","tag":"bridge","settings":{"auth":"noauth"}},
			 {"listen":"0.0.0.0","port":65101,"protocol":"hysteria","tag":"user"}`,
			``, 0,
		},
		{
			"reserved api inbound against a user inbound",
			`{"listen":"127.0.0.1","port":62789,"protocol":"tunnel","tag":"api","settings":{"rewriteAddress":"127.0.0.1"}},
			 {"listen":"0.0.0.0","port":62789,"protocol":"vless","tag":"user"}`,
			``, 1,
		},
		{
			// The core is running this pair right now, so whatever a static read
			// says about it, it binds: an established setup is never refused.
			"collision the running config already serves",
			relay + "," + user, relay + "," + user, 0,
		},
		{
			"collision the running config does not have",
			relay + "," + user, user, 1,
		},
		{
			// `::` and `0.0.0.0` on one port is what bindv6only=1 makes legal, so the
			// running config excuses it -- but moving one onto the other is not it.
			"excused pair whose listen changed into a real collision",
			`{"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"a"},
			 {"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"b"}`,
			`{"listen":"::","port":443,"protocol":"vless","tag":"a"},
			 {"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"b"}`,
			1,
		},
		{
			// The same pair of sockets is the same evidence, whichever order the
			// generator happened to emit them in.
			"excused pair with its listens swapped stays excused",
			`{"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"a"},
			 {"listen":"::","port":443,"protocol":"vless","tag":"b"}`,
			`{"listen":"::","port":443,"protocol":"vless","tag":"a"},
			 {"listen":"0.0.0.0","port":443,"protocol":"vless","tag":"b"}`,
			0,
		},
		{
			"same pair on another port is still new",
			`{"listen":"127.0.0.1","port":65102,"protocol":"socks","tag":"relay","settings":{"auth":"password","udp":true,"accounts":[]}},
			 {"listen":"0.0.0.0","port":65102,"protocol":"vless","tag":"user","streamSettings":{"network":"tcp"}}`,
			relay + "," + user, 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var running *xray.Config
			if tc.running != "" {
				running = configFromInbounds(t, tc.running)
			}
			got := bindConflicts(configFromInbounds(t, tc.inbounds), running)
			if len(got) != tc.want {
				t.Fatalf("bindConflicts = %v, want %d conflict(s)", got, tc.want)
			}
			for _, c := range got {
				if c.tagA == "" || c.tagB == "" || c.tagA == c.tagB {
					t.Fatalf("a conflict must name both tags, got %+v", c)
				}
			}
		})
	}
}

func TestBindConflicts_MessageNamesBothSides(t *testing.T) {
	conflicts := bindConflicts(configFromInbounds(t, `
		{"listen":"127.0.0.1","port":65101,"protocol":"socks","tag":"relay","settings":{"auth":"password","udp":true,"accounts":[]}},
		{"listen":"0.0.0.0","port":65101,"protocol":"vless","tag":"user","streamSettings":{"network":"tcp"}}`), nil)
	if len(conflicts) != 1 {
		t.Fatalf("want exactly one conflict, got %v", conflicts)
	}
	msg := conflicts[0].String()
	for _, want := range []string{`"relay"`, `"user"`, "127.0.0.1", "65101"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("conflict message %q must contain %q", msg, want)
		}
	}
}

// The probe must read what the panel really emits: the AmneziaWG relay is a
// loopback "socks" inbound whose udp flag lives in settings, not streamSettings.
func TestBindConflicts_GeneratedConfig(t *testing.T) {
	cases := []struct {
		name    string
		portOff int
		running bool
		want    int
	}{
		{"user inbound on the relay port", 0, false, 1},
		{"already running config", 0, true, 0},
		{"user inbound on a free port", 1, false, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupSettingTestDB(t)
			seedInboundConflict(t, "awg-1", "0.0.0.0", 51820, model.AmneziaWG, ``, amneziawgRoutedSettings)

			var awg model.Inbound
			if err := database.GetDB().Where("tag = ?", "awg-1").First(&awg).Error; err != nil {
				t.Fatalf("read seeded row: %v", err)
			}
			relayPort := amneziawgnet.SOCKSPortForInbound(awg.Id)
			seedInboundConflict(t, "user", "0.0.0.0", relayPort+tc.portOff, model.VLESS, `{"network":"tcp"}`, `{}`)

			svc := &XrayService{}
			cfg, err := svc.GetXrayConfig()
			if err != nil {
				t.Fatalf("GetXrayConfig: %v", err)
			}
			running := (*xray.Config)(nil)
			if tc.running {
				if running, err = svc.GetXrayConfig(); err != nil {
					t.Fatalf("second GetXrayConfig: %v", err)
				}
			}

			got := bindConflicts(cfg, running)
			if len(got) != tc.want {
				t.Fatalf("bindConflicts = %v, want %d", got, tc.want)
			}
			if tc.want == 0 {
				return
			}
			msg := got[0].String()
			for _, want := range []string{`"awg-1"`, `"user"`, strconv.Itoa(relayPort)} {
				if !strings.Contains(msg, want) {
					t.Fatalf("conflict message %q must contain %q", msg, want)
				}
			}
		})
	}
}
