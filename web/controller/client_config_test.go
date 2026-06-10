package controller

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

var validRealityPK = "fgGMxyC1JykjctWckExFm8ve3b1DE9aRTsCC6LNVdSo"
var validSSKey2022 = "86BmkQkd2Ef0daHnQ4suGQ=="
var validSSKey2022_256 = "3TwHn1DHyrnF+kzr2TsXv2XKvgLgKuESdAH0jxNP56o="

func writeConfigAndRunXrayTest(t *testing.T, name string, config map[string]any) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("[%s] marshal: %v", name, err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("[%s] write: %v", name, err)
	}

	t.Logf("[%s] config:\n%s", name, string(data))

	cmd := exec.Command("xray", "-test", "-c", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("[%s] xray -test FAILED:\n%s\n---\nerr: %v", name, string(output), err)
	}
	t.Logf("[%s] xray -test OK: %s", name, string(output))
}

func TestBuildClientConfig_VMess(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.VMESS,
		Listen:   "example.com",
		StreamSettings: `{
			"network": "tcp",
			"security": "none",
			"tcpSettings": {"header": {"type": "none"}}
		}`,
	}
	client := &model.ClientRecord{
		UUID:     "550e8400-e29b-41d4-a716-446655440000",
		Security: "auto",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vmess-plain-tcp", config)
}

func TestBuildClientConfig_VMess_TLS_WS(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.VMESS,
		Listen:   "example.com",
		StreamSettings: `{
			"network": "ws",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2","http/1.1"],
				"allowInsecure": false,
				"settings": {
					"fingerprint": "chrome"
				}
			},
			"wsSettings": {
				"path": "/websocket",
				"headers": {"Host": "example.com"}
			}
		}`,
	}
	client := &model.ClientRecord{
		UUID:     "550e8400-e29b-41d4-a716-446655440000",
		Security: "auto",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vmess-tls-ws", config)
}

func TestBuildClientConfig_VMess_TCP_TLS(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.VMESS,
		Listen:   "example.com",
		StreamSettings: `{
			"network": "tcp",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": "h2,http/1.1",
				"allowInsecure": false,
				"settings": {
					"fingerprint": "chrome"
				}
			},
			"tcpSettings": {
				"header": {"type": "none"}
			}
		}`,
	}
	client := &model.ClientRecord{
		UUID:     "550e8400-e29b-41d4-a716-446655440000",
		Security: "auto",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vmess-tcp-tls", config)
}

func TestBuildClientConfig_VMess_grpc(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.VMESS,
		Listen:   "example.com",
		StreamSettings: `{
			"network": "grpc",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2"],
				"allowInsecure": false
			},
			"grpcSettings": {
				"serviceName": "my-service"
			}
		}`,
	}
	client := &model.ClientRecord{
		UUID:     "550e8400-e29b-41d4-a716-446655440000",
		Security: "auto",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vmess-grpc-tls", config)
}

func TestBuildClientConfig_VLESS_Reality(t *testing.T) {
	for _, tc := range []struct {
		name     string
		network  string
		grpcSvc  string
		grpcMM   bool
	}{
		{"grpc", "grpc", "my-grpc-service", true},
		{"tcp", "tcp", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			streamSettings := map[string]any{
				"network": tc.network,
				"security": "reality",
				"realitySettings": map[string]any{
					"show":       false,
					"xver":       0,
					"publicKey":  validRealityPK,
					"fingerprint": "chrome",
					"serverNames": []string{"sub.example.com"},
					"shortIds":    []string{"6ba85179e30d4fc2"},
					"settings": map[string]any{
						"publicKey":  validRealityPK,
						"fingerprint": "chrome",
					},
				},
			}
			if tc.network == "tcp" {
				streamSettings["tcpSettings"] = map[string]any{
					"header": map[string]any{"type": "none"},
				}
			} else {
				streamSettings["grpcSettings"] = map[string]any{
					"serviceName": tc.grpcSvc,
					"multiMode":   tc.grpcMM,
				}
			}
			ssJSON, _ := json.Marshal(streamSettings)

			inbound := &model.Inbound{
				Port:           443,
				Protocol:       model.VLESS,
				Listen:         "85.190.96.240",
				Settings:       `{"encryption": "none"}`,
				StreamSettings: string(ssJSON),
			}
			client := &model.ClientRecord{
				UUID: "550e8400-e29b-41d4-a716-446655440000",
				Flow: "xtls-rprx-vision",
			}

			config, err := buildClientConfig(inbound, client, "")
			if err != nil {
				t.Fatal(err)
			}
			writeConfigAndRunXrayTest(t, "vless-reality-"+tc.name, config)
		})
	}
}

func TestBuildClientConfig_VLESS_TLS_WS(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.VLESS,
		Listen:   "example.com",
		Settings: `{"encryption": "none"}`,
		StreamSettings: `{
			"network": "ws",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2","http/1.1"],
				"allowInsecure": false,
				"settings": {
					"fingerprint": "chrome"
				}
			},
			"wsSettings": {
				"path": "/vl-websocket",
				"headers": {"Host": "example.com"}
			}
		}`,
	}
	client := &model.ClientRecord{
		UUID: "550e8400-e29b-41d4-a716-446655440000",
		Flow: "xtls-rprx-vision",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vless-tls-ws", config)
}

func TestBuildClientConfig_Trojan_TCP_TLS(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.Trojan,
		Listen:   "example.com",
		StreamSettings: `{
			"network": "tcp",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2","http/1.1"],
				"allowInsecure": false
			},
			"tcpSettings": {
				"header": {"type": "none"}
			}
		}`,
	}
	client := &model.ClientRecord{
		Password: "my-trojan-password",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "trojan-tcp-tls", config)
}

func TestBuildClientConfig_Trojan_WS_TLS(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.Trojan,
		Listen:   "example.com",
		StreamSettings: `{
			"network": "ws",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2","http/1.1"],
				"allowInsecure": false
			},
			"wsSettings": {
				"path": "/tro-ws",
				"headers": {"Host": "example.com"}
			}
		}`,
	}
	client := &model.ClientRecord{
		Password: "my-trojan-password",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "trojan-ws-tls", config)
}

func TestBuildClientConfig_Shadowsocks(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.Shadowsocks,
		Listen:   "example.com",
		Settings: `{"method": "aes-256-gcm"}`,
		StreamSettings: `{
			"network": "tcp",
			"security": "none",
			"tcpSettings": {"header": {"type": "none"}}
		}`,
	}
	client := &model.ClientRecord{
		Password: "ss-password",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "shadowsocks-plain", config)
}

func TestBuildClientConfig_Shadowsocks2022(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.Shadowsocks,
		Listen:   "example.com",
		Settings: `{"method": "2022-blake3-aes-128-gcm", "password": "` + validSSKey2022 + `"}`,
		StreamSettings: `{
			"network": "tcp",
			"security": "none",
			"tcpSettings": {"header": {"type": "none"}}
		}`,
	}
	client := &model.ClientRecord{
		Password: validSSKey2022,
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "shadowsocks2022-plain", config)
}

func TestBuildClientConfig_Hysteria2(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.Hysteria,
		Listen:   "example.com",
		Settings: `{"version": 2}`,
		StreamSettings: `{
			"network": "hysteria",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h3"],
				"allowInsecure": false,
				"settings": {
					"fingerprint": "chrome"
				}
			},
			"hysteriaSettings": {
				"version": 2,
				"auth": "hy2-auth"
			}
		}`,
	}
	client := &model.ClientRecord{
		Auth: "hy2-auth",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "hysteria2", config)
}

func TestBuildClientConfig_Hysteria2_full(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.Hysteria,
		Listen:   "example.com",
		Settings: `{"version": 2}`,
		StreamSettings: `{
			"network": "hysteria",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h3"],
				"allowInsecure": false,
				"settings": {
					"fingerprint": "chrome"
				}
			},
			"hysteriaSettings": {
				"version": 2,
				"auth": "hy2-auth",
				"upMbps": 100,
				"downMbps": 100
			}
		}`,
	}
	client := &model.ClientRecord{
		Auth: "hy2-auth",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "hysteria2-full", config)
}

func TestBuildClientConfig_httpupgrade(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.VLESS,
		Listen:   "example.com",
		Settings: `{"encryption": "none"}`,
		StreamSettings: `{
			"network": "httpupgrade",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2","http/1.1"],
				"allowInsecure": false,
				"settings": {
					"fingerprint": "chrome"
				}
			},
			"httpupgradeSettings": {
				"path": "/httpupgrade",
				"host": "example.com"
			}
		}`,
	}
	client := &model.ClientRecord{
		UUID: "550e8400-e29b-41d4-a716-446655440000",
		Flow: "xtls-rprx-vision",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vless-httpupgrade-tls", config)
}

func TestBuildClientConfig_VMess_security_variants(t *testing.T) {
	for _, sec := range []string{"aes-128-gcm", "chacha20-poly1305", "none", "zero"} {
		t.Run(sec, func(t *testing.T) {
			inbound := &model.Inbound{
				Port:     443,
				Protocol: model.VMESS,
				Listen:   "example.com",
				StreamSettings: `{
					"network": "tcp",
					"security": "none",
					"tcpSettings": {"header": {"type": "none"}}
				}`,
			}
			client := &model.ClientRecord{
				UUID:     "550e8400-e29b-41d4-a716-446655440000",
				Security: sec,
			}
			config, err := buildClientConfig(inbound, client, "")
			if err != nil {
				t.Fatal(err)
			}
			writeConfigAndRunXrayTest(t, "vmess-"+sec, config)
		})
	}
}

func TestBuildClientConfig_VMess_TLS_variants(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.VMESS,
		Listen:   "example.com",
		StreamSettings: `{
			"network": "tcp",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2","http/1.1"],
				"allowInsecure": false,
				"settings": {
					"fingerprint": "chrome",
					"echConfigList": "AAEBAQUBAgMEBQYHCAkKCwwNDg8Q",
					"pinnedPeerCertSha256": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
				}
			},
			"tcpSettings": {"header": {"type": "none"}}
		}`,
	}
	client := &model.ClientRecord{
		UUID:     "550e8400-e29b-41d4-a716-446655440000",
		Security: "auto",
	}
	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vmess-tls-ech-pinned", config)
}

func TestBuildClientConfig_VMess_KCP(t *testing.T) {
	streamSettings := map[string]any{
		"network": "kcp",
		"security": "none",
		"kcpSettings": map[string]any{
			"mtu": 1350,
			"tti": 50,
		},
		"finalmask": map[string]any{
			"udp": []any{
				map[string]any{"type": "mkcp-original"},
			},
		},
	}
	ssJSON, _ := json.Marshal(streamSettings)
	inbound := &model.Inbound{
		Port:           443,
		Protocol:       model.VMESS,
		Listen:         "example.com",
		StreamSettings: string(ssJSON),
	}
	client := &model.ClientRecord{
		UUID:     "550e8400-e29b-41d4-a716-446655440000",
		Security: "auto",
	}
	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vmess-kcp", config)
}

func TestBuildClientConfig_VLESS_TCP_TLS_NoFlow(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.VLESS,
		Listen:   "example.com",
		Settings: `{"encryption": "none"}`,
		StreamSettings: `{
			"network": "tcp",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2","http/1.1"],
				"allowInsecure": false,
				"settings": {"fingerprint": "chrome"}
			},
			"tcpSettings": {"header": {"type": "none"}}
		}`,
	}
	client := &model.ClientRecord{
		UUID: "550e8400-e29b-41d4-a716-446655440000",
	}
	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vless-tcp-tls-noflow", config)
}

func TestBuildClientConfig_VLESS_gRPC_TLS(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.VLESS,
		Listen:   "example.com",
		Settings: `{"encryption": "none"}`,
		StreamSettings: `{
			"network": "grpc",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2"],
				"allowInsecure": false,
				"settings": {"fingerprint": "chrome"}
			},
			"grpcSettings": {
				"serviceName": "vless-grpc-svc"
			}
		}`,
	}
	client := &model.ClientRecord{
		UUID: "550e8400-e29b-41d4-a716-446655440000",
		Flow: "xtls-rprx-vision",
	}
	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vless-grpc-tls", config)
}

func TestBuildClientConfig_VLESS_KCP(t *testing.T) {
	streamSettings := map[string]any{
		"network": "kcp",
		"security": "none",
		"kcpSettings": map[string]any{
			"mtu": 1350,
			"tti": 50,
		},
		"finalmask": map[string]any{
			"udp": []any{
				map[string]any{"type": "mkcp-original"},
			},
		},
	}
	ssJSON, _ := json.Marshal(streamSettings)
	inbound := &model.Inbound{
		Port:           443,
		Protocol:       model.VLESS,
		Listen:         "example.com",
		Settings:       `{"encryption": "none"}`,
		StreamSettings: string(ssJSON),
	}
	client := &model.ClientRecord{
		UUID: "550e8400-e29b-41d4-a716-446655440000",
		Flow: "xtls-rprx-vision",
	}
	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vless-kcp", config)
}

func TestBuildClientConfig_VLESS_KCP_TLS(t *testing.T) {
	streamSettings := map[string]any{
		"network": "kcp",
		"security": "tls",
		"kcpSettings": map[string]any{
			"mtu": 1350,
			"tti": 50,
		},
		"finalmask": map[string]any{
			"udp": []any{
				map[string]any{"type": "mkcp-original"},
			},
		},
		"tlsSettings": map[string]any{
			"serverName": "example.com",
			"alpn":       []string{"h2", "http/1.1"},
			"allowInsecure": false,
			"settings": map[string]any{
				"fingerprint": "chrome",
			},
		},
	}
	ssJSON, _ := json.Marshal(streamSettings)
	inbound := &model.Inbound{
		Port:           443,
		Protocol:       model.VLESS,
		Listen:         "example.com",
		Settings:       `{"encryption": "none"}`,
		StreamSettings: string(ssJSON),
	}
	client := &model.ClientRecord{
		UUID: "550e8400-e29b-41d4-a716-446655440000",
		Flow: "xtls-rprx-vision",
	}
	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vless-kcp-tls", config)
}

func TestBuildClientConfig_Trojan_gRPC(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.Trojan,
		Listen:   "example.com",
		StreamSettings: `{
			"network": "grpc",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2"],
				"allowInsecure": false
			},
			"grpcSettings": {
				"serviceName": "tro-grpc",
				"multiMode": true
			}
		}`,
	}
	client := &model.ClientRecord{
		Password: "trojan-grpc-pw",
	}
	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "trojan-grpc-tls", config)
}

func TestBuildClientConfig_Shadowsocks_ciphers(t *testing.T) {
	for _, cipher := range []string{"chacha20-ietf-poly1305", "aes-128-gcm"} {
		t.Run(cipher, func(t *testing.T) {
			inbound := &model.Inbound{
				Port:     443,
				Protocol: model.Shadowsocks,
				Listen:   "example.com",
				Settings: `{"method": "` + cipher + `"}`,
				StreamSettings: `{
					"network": "tcp",
					"security": "none",
					"tcpSettings": {"header": {"type": "none"}}
				}`,
			}
			client := &model.ClientRecord{
				Password: "ss-password",
			}
			config, err := buildClientConfig(inbound, client, "")
			if err != nil {
				t.Fatal(err)
			}
			writeConfigAndRunXrayTest(t, "ss-"+cipher, config)
		})
	}
}

func TestBuildClientConfig_Shadowsocks2022_256(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.Shadowsocks,
		Listen:   "example.com",
		Settings: `{"method": "2022-blake3-aes-256-gcm", "password": "` + validSSKey2022_256 + `"}`,
		StreamSettings: `{
			"network": "tcp",
			"security": "none",
			"tcpSettings": {"header": {"type": "none"}}
		}`,
	}
	client := &model.ClientRecord{
		Password: validSSKey2022_256,
	}
	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "shadowsocks2022-256", config)
}

func TestBuildClientConfig_Hysteria2_full_plus(t *testing.T) {
	streamSettings := map[string]any{
		"network": "hysteria",
		"security": "tls",
		"tlsSettings": map[string]any{
			"serverName":   "example.com",
			"alpn":         []string{"h3"},
			"allowInsecure": false,
			"settings": map[string]any{
				"fingerprint": "chrome",
			},
		},
		"hysteriaSettings": map[string]any{
			"version": 2,
			"auth":    "hy2-auth",
			"up":      "100 Mbps",
			"down":    "100 Mbps",
		},
		"finalmask": map[string]any{
			"quicParams": map[string]any{
				"congestion": "brutal",
				"brutalUp":   "100M",
				"brutalDown": "100M",
				"udpHop": map[string]any{
					"ports":    "4000-5000",
					"interval": "30",
				},
			},
			"udp": []any{
				map[string]any{
					"type": "salamander",
					"settings": map[string]any{
						"password": "obfs-secret",
					},
				},
			},
		},
	}
	ssJSON, _ := json.Marshal(streamSettings)
	inbound := &model.Inbound{
		Port:           443,
		Protocol:       model.Hysteria,
		Listen:         "example.com",
		Settings:       `{"version": 2}`,
		StreamSettings: string(ssJSON),
	}
	client := &model.ClientRecord{
		Auth: "hy2-auth",
	}
	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "hysteria2-full-plus", config)
}

func TestBuildClientConfig_xhttp(t *testing.T) {
	inbound := &model.Inbound{
		Port:     443,
		Protocol: model.VLESS,
		Listen:   "example.com",
		Settings: `{"encryption": "none"}`,
		StreamSettings: `{
			"network": "xhttp",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"alpn": ["h2","http/1.1"],
				"allowInsecure": false
			},
			"xhttpSettings": {
				"path": "/xhttp",
				"host": "example.com",
				"mode": "auto",
				"noSSEHeader": true,
				"scMaxBufferedPosts": 32,
				"scStreamUpServerSecs": 5,
				"serverMaxHeaderBytes": 4096
			}
		}`,
	}
	client := &model.ClientRecord{
		UUID: "550e8400-e29b-41d4-a716-446655440000",
		Flow: "xtls-rprx-vision",
	}

	config, err := buildClientConfig(inbound, client, "")
	if err != nil {
		t.Fatal(err)
	}
	writeConfigAndRunXrayTest(t, "vless-xhttp-tls", config)
}
