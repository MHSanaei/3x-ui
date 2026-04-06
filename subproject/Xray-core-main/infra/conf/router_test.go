package conf_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
	_ "unsafe"

	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/platform"
	"github.com/xtls/xray-core/common/platform/filesystem"
	"github.com/xtls/xray-core/common/serial"
	. "github.com/xtls/xray-core/infra/conf"
	"google.golang.org/protobuf/proto"
)

func getAssetPath(file string) (string, error) {
	path := platform.GetAssetLocation(file)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		path := filepath.Join("..", "..", "resources", file)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return "", fmt.Errorf("can't find %s in standard asset locations or {project_root}/resources", file)
		}
		if err != nil {
			return "", fmt.Errorf("can't stat %s: %v", path, err)
		}
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("can't stat %s: %v", path, err)
	}

	return path, nil
}

func TestToCidrList(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-")
	if err != nil {
		t.Fatalf("can't create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	geoipPath, err := getAssetPath("geoip.dat")
	if err != nil {
		t.Fatal(err)
	}

	common.Must(filesystem.CopyFile(filepath.Join(tempDir, "geoip.dat"), geoipPath))
	common.Must(filesystem.CopyFile(filepath.Join(tempDir, "geoiptestrouter.dat"), geoipPath))

	os.Setenv("xray.location.asset", tempDir)
	defer os.Unsetenv("xray.location.asset")

	ips := StringList([]string{
		"geoip:us",
		"geoip:cn",
		"geoip:!cn",
		"ext:geoiptestrouter.dat:!cn",
		"ext:geoiptestrouter.dat:ca",
		"ext-ip:geoiptestrouter.dat:!cn",
		"ext-ip:geoiptestrouter.dat:!ca",
	})

	_, err = ToCidrList(ips)
	if err != nil {
		t.Fatalf("Failed to parse geoip list, got %s", err)
	}
}

func TestRouterConfig(t *testing.T) {
	createParser := func() func(string) (proto.Message, error) {
		return func(s string) (proto.Message, error) {
			config := new(RouterConfig)
			if err := json.Unmarshal([]byte(s), config); err != nil {
				return nil, err
			}
			return config.Build()
		}
	}

	runMultiTestCase(t, []TestCase{
		{
			Input: `{
				"domainStrategy": "AsIs",
				"rules": [
					{
						"domain": [
							"baidu.com",
							"qq.com"
						],
						"outboundTag": "direct"
					},
					{
						"ip": [
							"10.0.0.0/8",
							"::1/128"
						],
						"outboundTag": "test"
					},{
						"port": "53, 443, 1000-2000",
						"outboundTag": "test"
					},{
						"port": 123,
						"outboundTag": "test"
					}
				],
				"balancers": [
					{
						"tag": "b1",
						"selector": ["test"],
						"fallbackTag": "fall"
					},
					{
						"tag": "b2",
						"selector": ["test"],
						"strategy": {
							"type": "leastload",
							"settings": {
								"healthCheck": {
									"interval": "5m0s",
									"sampling": 2,
									"timeout": "5s",
									"destination": "dest",
									"connectivity": "conn"
								},
								"costs": [
									{
										"regexp": true,
										"match": "\\d+(\\.\\d+)",
										"value": 5
									}
								],
								"baselines": ["400ms", "600ms"],
								"expected": 6,
								"maxRTT": "1000ms",
								"tolerance": 0.5
							}
						},
						"fallbackTag": "fall"
					}
				]
			}`,
			Parser: createParser(),
			Output: &router.Config{
				DomainStrategy: router.Config_AsIs,
				BalancingRule: []*router.BalancingRule{
					{
						Tag:              "b1",
						OutboundSelector: []string{"test"},
						Strategy:         "random",
						FallbackTag:      "fall",
					},
					{
						Tag:              "b2",
						OutboundSelector: []string{"test"},
						Strategy:         "leastload",
						StrategySettings: serial.ToTypedMessage(&router.StrategyLeastLoadConfig{
							Costs: []*router.StrategyWeight{
								{
									Regexp: true,
									Match:  "\\d+(\\.\\d+)",
									Value:  5,
								},
							},
							Baselines: []int64{
								int64(time.Duration(400) * time.Millisecond),
								int64(time.Duration(600) * time.Millisecond),
							},
							Expected:  6,
							MaxRTT:    int64(time.Duration(1000) * time.Millisecond),
							Tolerance: 0.5,
						}),
						FallbackTag: "fall",
					},
				},
				Rule: []*router.RoutingRule{
					{
						Domain: []*router.Domain{
							{
								Type:  router.Domain_Plain,
								Value: "baidu.com",
							},
							{
								Type:  router.Domain_Plain,
								Value: "qq.com",
							},
						},
						TargetTag: &router.RoutingRule_Tag{
							Tag: "direct",
						},
					},
					{
						Geoip: []*router.GeoIP{
							{
								Cidr: []*router.CIDR{
									{
										Ip:     []byte{10, 0, 0, 0},
										Prefix: 8,
									},
									{
										Ip:     []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
										Prefix: 128,
									},
								},
							},
						},
						TargetTag: &router.RoutingRule_Tag{
							Tag: "test",
						},
					},
					{
						PortList: &net.PortList{
							Range: []*net.PortRange{
								{From: 53, To: 53},
								{From: 443, To: 443},
								{From: 1000, To: 2000},
							},
						},
						TargetTag: &router.RoutingRule_Tag{
							Tag: "test",
						},
					},
					{
						PortList: &net.PortList{
							Range: []*net.PortRange{
								{From: 123, To: 123},
							},
						},
						TargetTag: &router.RoutingRule_Tag{
							Tag: "test",
						},
					},
				},
			},
		},
		{
			Input: `{
				"domainStrategy": "IPIfNonMatch",
				"rules": [
					{
						"domain": [
							"baidu.com",
							"qq.com"
						],
						"outboundTag": "direct"
					},
					{
						"ip": [
							"10.0.0.0/8",
							"::1/128"
						],
						"outboundTag": "test"
					}
				]
			}`,
			Parser: createParser(),
			Output: &router.Config{
				DomainStrategy: router.Config_IpIfNonMatch,
				Rule: []*router.RoutingRule{
					{
						Domain: []*router.Domain{
							{
								Type:  router.Domain_Plain,
								Value: "baidu.com",
							},
							{
								Type:  router.Domain_Plain,
								Value: "qq.com",
							},
						},
						TargetTag: &router.RoutingRule_Tag{
							Tag: "direct",
						},
					},
					{
						Geoip: []*router.GeoIP{
							{
								Cidr: []*router.CIDR{
									{
										Ip:     []byte{10, 0, 0, 0},
										Prefix: 8,
									},
									{
										Ip:     []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
										Prefix: 128,
									},
								},
							},
						},
						TargetTag: &router.RoutingRule_Tag{
							Tag: "test",
						},
					},
				},
			},
		},
	})
}
