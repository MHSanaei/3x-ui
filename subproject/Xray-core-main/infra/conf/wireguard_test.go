package conf_test

import (
	"testing"

	. "github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/proxy/wireguard"
)

func TestWireGuardConfig(t *testing.T) {
	creator := func() Buildable {
		return new(WireGuardConfig)
	}

	runMultiTestCase(t, []TestCase{
		{
			Input: `{
				"secretKey": "uJv5tZMDltsiYEn+kUwb0Ll/CXWhMkaSCWWhfPEZM3A=",
				"address": ["10.1.1.1", "fd59:7153:2388:b5fd:0000:0000:1234:0001"],
				"peers": [
					{
						"publicKey": "6e65ce0be17517110c17d77288ad87e7fd5252dcc7d09b95a39d61db03df832a",
						"endpoint": "127.0.0.1:1234"
					}
				],
				"mtu": 1300,
				"workers": 2,
				"domainStrategy": "ForceIPv6v4",
				"noKernelTun": false
			}`,
			Parser: loadJSON(creator),
			Output: &wireguard.DeviceConfig{
				// key converted into hex form
				SecretKey: "b89bf9b5930396db226049fe914c1bd0b97f0975a13246920965a17cf1193370",
				Endpoint:  []string{"10.1.1.1", "fd59:7153:2388:b5fd:0000:0000:1234:0001"},
				Peers: []*wireguard.PeerConfig{
					{
						// also can read from hex form directly
						PublicKey:  "6e65ce0be17517110c17d77288ad87e7fd5252dcc7d09b95a39d61db03df832a",
						Endpoint:   "127.0.0.1:1234",
						KeepAlive:  0,
						AllowedIps: []string{"0.0.0.0/0", "::0/0"},
					},
				},
				Mtu:            1300,
				NumWorkers:     2,
				DomainStrategy: wireguard.DeviceConfig_FORCE_IP64,
				NoKernelTun:    false,
			},
		},
	})
}
