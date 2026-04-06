package sudoku

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	cryptotls "crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	stdnet "net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/xtls/xray-core/app/dispatcher"
	"github.com/xtls/xray-core/app/log"
	"github.com/xtls/xray-core/app/proxyman"
	clog "github.com/xtls/xray-core/common/log"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/protocol/tls/cert"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/common/uuid"
	core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/proxy/dokodemo"
	"github.com/xtls/xray-core/proxy/freedom"
	hyproxy "github.com/xtls/xray-core/proxy/hysteria"
	hyaccount "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/vless"
	vin "github.com/xtls/xray-core/proxy/vless/inbound"
	vout "github.com/xtls/xray-core/proxy/vless/outbound"
	testingtcp "github.com/xtls/xray-core/testing/servers/tcp"
	"github.com/xtls/xray-core/transport/internet"
	hytransport "github.com/xtls/xray-core/transport/internet/hysteria"
	"github.com/xtls/xray-core/transport/internet/reality"
	splithttp "github.com/xtls/xray-core/transport/internet/splithttp"
	transtcp "github.com/xtls/xray-core/transport/internet/tcp"
	xtls "github.com/xtls/xray-core/transport/internet/tls"
	"google.golang.org/protobuf/proto"
)

var (
	e2eBinaryOnce sync.Once
	e2eBinaryPath string
	e2eBinaryErr  error
)

type trafficMode struct {
	name   string
	config *Config
}

type protocolCase struct {
	name      string
	transport string
	run       func(t *testing.T, bin string, mode trafficMode) caseResult
}

type caseResult struct {
	Protocol         string
	Mode             string
	TotalBytes       int
	ASCIIBytes       int
	ASCIIRatio       float64
	AvgHammingOnes   float64
	RotationSeen     int
	RotationExpected int
	DecodedUnits     int
	ClientToServer   directionResult
	ServerToClient   directionResult
}

type directionResult struct {
	RawBytes       int
	ASCIIBytes     int
	ASCIIRatio     float64
	AvgHammingOnes float64
	RotationSeen   int
	DecodedUnits   int
}

type tcpRelay struct {
	listener stdnet.Listener
	target   string

	mu       sync.Mutex
	captures []*tcpCapture
	wg       sync.WaitGroup
	stopCh   chan struct{}
}

type tcpCapture struct {
	mu  sync.Mutex
	c2s []byte
	s2c []byte
}

type udpRelay struct {
	conn      stdnet.PacketConn
	target    *stdnet.UDPAddr
	clientMu  sync.Mutex
	client    *stdnet.UDPAddr
	stopCh    chan struct{}
	wg        sync.WaitGroup
	captureMu sync.Mutex
	c2s       [][]byte
	s2c       [][]byte
}

type tlsDecoy struct {
	ln   stdnet.Listener
	done chan struct{}
	wg   sync.WaitGroup
}

func TestSudokuE2ETemp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sudoku e2e harness in short mode")
	}

	bin := buildE2EBinary(t)
	payloadSize := 192 * 1024
	modes := []trafficMode{
		{
			name: "prefer_ascii",
			config: &Config{
				Password: "sudoku-e2e-shared-secret",
				Ascii:    "prefer_ascii",
			},
		},
		{
			name: "prefer_entropy",
			config: &Config{
				Password: "sudoku-e2e-shared-secret",
				Ascii:    "prefer_entropy",
				CustomTables: []string{
					"xpxvvpvv",
					"vxpvxvvp",
					"pxvvxvvp",
					"vpxvxvpv",
					"xvpvvxpv",
					"vvxpxpvv",
				},
			},
		},
	}

	cases := []protocolCase{
		{name: "vless-reality", transport: "tcp", run: func(t *testing.T, bin string, mode trafficMode) caseResult {
			return runVLESSRealityCase(t, bin, mode, payloadSize)
		}},
		{name: "hysteria2", transport: "udp", run: func(t *testing.T, bin string, mode trafficMode) caseResult {
			return runHysteria2Case(t, bin, mode, payloadSize)
		}},
		{name: "vless-enc", transport: "tcp", run: func(t *testing.T, bin string, mode trafficMode) caseResult {
			return runVLesseEncCase(t, bin, mode, payloadSize)
		}},
		{name: "vless-xhttp", transport: "tcp", run: func(t *testing.T, bin string, mode trafficMode) caseResult {
			return runVLESSXHTTPCase(t, bin, mode, payloadSize)
		}},
	}

	results := make([]caseResult, 0, len(cases)*len(modes))
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range modes {
				mode := mode
				t.Run(mode.name, func(t *testing.T) {
					result := tc.run(t, bin, mode)
					if mode.name == "prefer_ascii" && result.ASCIIRatio < 0.97 {
						t.Fatalf("%s %s ascii ratio %.4f < 0.97", tc.name, mode.name, result.ASCIIRatio)
					}
					if mode.name == "prefer_entropy" {
						if result.RotationSeen != result.RotationExpected {
							t.Fatalf("%s %s saw %d/%d rotation tables", tc.name, mode.name, result.RotationSeen, result.RotationExpected)
						}
						if diff := result.AvgHammingOnes - 5.0; diff < -0.3 || diff > 0.3 {
							t.Fatalf("%s %s average ones %.4f too far from 5", tc.name, mode.name, result.AvgHammingOnes)
						}
					}
					t.Logf(
						"%s %s total=%d ascii=%.4f avg_ones=%.4f rotation=%d/%d c2s_ascii=%.4f s2c_ascii=%.4f",
						tc.name,
						mode.name,
						result.TotalBytes,
						result.ASCIIRatio,
						result.AvgHammingOnes,
						result.RotationSeen,
						result.RotationExpected,
						result.ClientToServer.ASCIIRatio,
						result.ServerToClient.ASCIIRatio,
					)
					results = append(results, result)
				})
			}
		})
	}

	for _, result := range results {
		t.Logf(
			"summary protocol=%s mode=%s bytes=%d ascii=%.4f avg_ones=%.4f rotation=%d/%d decoded=%d",
			result.Protocol,
			result.Mode,
			result.TotalBytes,
			result.ASCIIRatio,
			result.AvgHammingOnes,
			result.RotationSeen,
			result.RotationExpected,
			result.DecodedUnits,
		)
	}
}

func runVLESSRealityCase(t *testing.T, bin string, mode trafficMode, payloadSize int) caseResult {
	backend := startXOREchoServer(t)
	defer backend.Close()

	decoyCert, _ := cert.MustGenerate(nil, cert.CommonName("localhost"), cert.DNSNames("localhost"))
	decoy := startTLSEchoDecoy(t, decoyCert)
	defer decoy.Close()

	serverPort := testingtcp.PickPort()
	relayPort := testingtcp.PickPort()
	clientPort := testingtcp.PickPort()

	relay := startTCPRelay(t, int(relayPort), fmt.Sprintf("127.0.0.1:%d", serverPort))
	defer relay.Close()

	userID := protocol.NewID(uuid.New())
	realityPriv, realityPub := mustX25519Keypair(t)
	shortID := mustDecodeHex(t, "0123456789abcdef")

	serverConfig := defaultApps(&core.Config{
		Inbound: []*core.InboundHandlerConfig{
			{
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &xnet.PortList{Range: []*xnet.PortRange{xnet.SinglePortRange(serverPort)}},
					Listen:   xnet.NewIPOrDomain(xnet.LocalHostIP),
					StreamSettings: &internet.StreamConfig{
						ProtocolName: "tcp",
						TransportSettings: []*internet.TransportConfig{
							{
								ProtocolName: "tcp",
								Settings:     serial.ToTypedMessage(&transtcp.Config{}),
							},
						},
						SecurityType: serial.GetMessageType(&reality.Config{}),
						SecuritySettings: []*serial.TypedMessage{
							serial.ToTypedMessage(&reality.Config{
								Show:        true,
								Dest:        fmt.Sprintf("localhost:%d", decoy.Port()),
								ServerNames: []string{"localhost"},
								PrivateKey:  realityPriv,
								ShortIds:    [][]byte{shortID},
								Type:        "tcp",
							}),
						},
						Tcpmasks: []*serial.TypedMessage{serial.ToTypedMessage(cloneConfig(mode.config))},
					},
				}),
				ProxySettings: serial.ToTypedMessage(&vin.Config{
					Clients: []*protocol.User{
						{
							Account: serial.ToTypedMessage(&vless.Account{
								Id: userID.String(),
							}),
						},
					},
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{ProxySettings: serial.ToTypedMessage(&freedom.Config{})},
		},
	})

	clientConfig := defaultApps(&core.Config{
		Inbound: []*core.InboundHandlerConfig{
			{
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &xnet.PortList{Range: []*xnet.PortRange{xnet.SinglePortRange(clientPort)}},
					Listen:   xnet.NewIPOrDomain(xnet.LocalHostIP),
				}),
				ProxySettings: serial.ToTypedMessage(&dokodemo.Config{
					Address:  xnet.NewIPOrDomain(backend.Address()),
					Port:     uint32(backend.Port()),
					Networks: []xnet.Network{xnet.Network_TCP},
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&vout.Config{
					Vnext: &protocol.ServerEndpoint{
						Address: xnet.NewIPOrDomain(xnet.LocalHostIP),
						Port:    uint32(relayPort),
						User: &protocol.User{
							Account: serial.ToTypedMessage(&vless.Account{
								Id: userID.String(),
							}),
						},
					},
				}),
				SenderSettings: serial.ToTypedMessage(&proxyman.SenderConfig{
					StreamSettings: &internet.StreamConfig{
						ProtocolName: "tcp",
						TransportSettings: []*internet.TransportConfig{
							{
								ProtocolName: "tcp",
								Settings:     serial.ToTypedMessage(&transtcp.Config{}),
							},
						},
						SecurityType: serial.GetMessageType(&reality.Config{}),
						SecuritySettings: []*serial.TypedMessage{
							serial.ToTypedMessage(&reality.Config{
								Show:        true,
								Fingerprint: "chrome",
								ServerName:  "localhost",
								PublicKey:   realityPub,
								ShortId:     shortID,
								SpiderX:     "/",
							}),
						},
						Tcpmasks: []*serial.TypedMessage{serial.ToTypedMessage(cloneConfig(mode.config))},
					},
				}),
			},
		},
	})

	serverCmd, clientCmd := runXrayPair(t, bin, serverConfig, clientConfig)
	defer stopCmd(clientCmd)
	defer stopCmd(serverCmd)
	exerciseTCPClient(t, int(clientPort), payloadSize)

	return analyzeTCPRelay(t, "vless-reality", mode, relay.Snapshots())
}

func runHysteria2Case(t *testing.T, bin string, mode trafficMode, payloadSize int) caseResult {
	backend := startXOREchoServer(t)
	defer backend.Close()

	serverPort := testingtcp.PickPort()
	relayPort := testingtcp.PickPort()
	clientPort := testingtcp.PickPort()

	relay := startUDPRelay(t, int(relayPort), int(serverPort))
	defer relay.Close()

	ct, ctHash := cert.MustGenerate(nil, cert.CommonName("localhost"), cert.DNSNames("localhost"))
	auth := "hy2-auth-secret"

	serverConfig := defaultApps(&core.Config{
		Inbound: []*core.InboundHandlerConfig{
			{
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &xnet.PortList{Range: []*xnet.PortRange{xnet.SinglePortRange(serverPort)}},
					Listen:   xnet.NewIPOrDomain(xnet.LocalHostIP),
					StreamSettings: &internet.StreamConfig{
						ProtocolName: "hysteria",
						TransportSettings: []*internet.TransportConfig{
							{
								ProtocolName: "hysteria",
								Settings: serial.ToTypedMessage(&hytransport.Config{
									Version:        2,
									Auth:           auth,
									UdpIdleTimeout: 60,
								}),
							},
						},
						SecurityType: serial.GetMessageType(&xtls.Config{}),
						SecuritySettings: []*serial.TypedMessage{
							serial.ToTypedMessage(&xtls.Config{
								Certificate:  []*xtls.Certificate{xtls.ParseCertificate(ct)},
								NextProtocol: []string{"h3"},
							}),
						},
						Udpmasks: []*serial.TypedMessage{serial.ToTypedMessage(cloneConfig(mode.config))},
					},
				}),
				ProxySettings: serial.ToTypedMessage(&hyproxy.ServerConfig{
					Users: []*protocol.User{
						{
							Account: serial.ToTypedMessage(&hyaccount.Account{Auth: auth}),
						},
					},
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{ProxySettings: serial.ToTypedMessage(&freedom.Config{})},
		},
	})

	clientConfig := defaultApps(&core.Config{
		Inbound: []*core.InboundHandlerConfig{
			{
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &xnet.PortList{Range: []*xnet.PortRange{xnet.SinglePortRange(clientPort)}},
					Listen:   xnet.NewIPOrDomain(xnet.LocalHostIP),
				}),
				ProxySettings: serial.ToTypedMessage(&dokodemo.Config{
					Address:  xnet.NewIPOrDomain(backend.Address()),
					Port:     uint32(backend.Port()),
					Networks: []xnet.Network{xnet.Network_TCP},
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&hyproxy.ClientConfig{
					Version: 2,
					Server: &protocol.ServerEndpoint{
						Address: xnet.NewIPOrDomain(xnet.LocalHostIP),
						Port:    uint32(relayPort),
						User: &protocol.User{
							Account: serial.ToTypedMessage(&hyaccount.Account{Auth: auth}),
						},
					},
				}),
				SenderSettings: serial.ToTypedMessage(&proxyman.SenderConfig{
					StreamSettings: &internet.StreamConfig{
						ProtocolName: "hysteria",
						TransportSettings: []*internet.TransportConfig{
							{
								ProtocolName: "hysteria",
								Settings: serial.ToTypedMessage(&hytransport.Config{
									Version:        2,
									Auth:           auth,
									UdpIdleTimeout: 60,
								}),
							},
						},
						SecurityType: serial.GetMessageType(&xtls.Config{}),
						SecuritySettings: []*serial.TypedMessage{
							serial.ToTypedMessage(&xtls.Config{
								ServerName:           "localhost",
								PinnedPeerCertSha256: [][]byte{ctHash[:]},
								NextProtocol:         []string{"h3"},
							}),
						},
						Udpmasks: []*serial.TypedMessage{serial.ToTypedMessage(cloneConfig(mode.config))},
					},
				}),
			},
		},
	})

	serverCmd, clientCmd := runXrayPair(t, bin, serverConfig, clientConfig)
	defer stopCmd(clientCmd)
	defer stopCmd(serverCmd)
	if err := exerciseTCPClientErr(t, int(clientPort), payloadSize); err != nil {
		c2s, s2c := relay.Snapshots()
		t.Fatalf("hy2 traffic failed: %v (udp packets c2s=%d s2c=%d first_c2s=%d first_s2c=%d)", err, len(c2s), len(s2c), firstChunkLen(c2s), firstChunkLen(s2c))
	}

	c2s, s2c := relay.Snapshots()
	return analyzeUDPRelay(t, "hysteria2", mode, c2s, s2c)
}

func runVLesseEncCase(t *testing.T, bin string, mode trafficMode, payloadSize int) caseResult {
	backend := startXOREchoServer(t)
	defer backend.Close()

	serverPort := testingtcp.PickPort()
	relayPort := testingtcp.PickPort()
	clientPort := testingtcp.PickPort()

	relay := startTCPRelay(t, int(relayPort), fmt.Sprintf("127.0.0.1:%d", serverPort))
	defer relay.Close()

	userID := protocol.NewID(uuid.New())
	priv, pub := mustX25519Keypair(t)
	pubB64 := base64.RawURLEncoding.EncodeToString(pub)
	privB64 := base64.RawURLEncoding.EncodeToString(priv)

	serverConfig := defaultApps(&core.Config{
		Inbound: []*core.InboundHandlerConfig{
			{
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &xnet.PortList{Range: []*xnet.PortRange{xnet.SinglePortRange(serverPort)}},
					Listen:   xnet.NewIPOrDomain(xnet.LocalHostIP),
					StreamSettings: &internet.StreamConfig{
						ProtocolName: "tcp",
						TransportSettings: []*internet.TransportConfig{
							{ProtocolName: "tcp", Settings: serial.ToTypedMessage(&transtcp.Config{})},
						},
						Tcpmasks: []*serial.TypedMessage{serial.ToTypedMessage(cloneConfig(mode.config))},
					},
				}),
				ProxySettings: serial.ToTypedMessage(&vin.Config{
					Clients: []*protocol.User{
						{
							Account: serial.ToTypedMessage(&vless.Account{
								Id: userID.String(),
							}),
						},
					},
					Decryption:  privB64,
					XorMode:     1,
					SecondsFrom: 0,
					SecondsTo:   0,
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{ProxySettings: serial.ToTypedMessage(&freedom.Config{})},
		},
	})

	clientConfig := defaultApps(&core.Config{
		Inbound: []*core.InboundHandlerConfig{
			{
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &xnet.PortList{Range: []*xnet.PortRange{xnet.SinglePortRange(clientPort)}},
					Listen:   xnet.NewIPOrDomain(xnet.LocalHostIP),
				}),
				ProxySettings: serial.ToTypedMessage(&dokodemo.Config{
					Address:  xnet.NewIPOrDomain(backend.Address()),
					Port:     uint32(backend.Port()),
					Networks: []xnet.Network{xnet.Network_TCP},
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&vout.Config{
					Vnext: &protocol.ServerEndpoint{
						Address: xnet.NewIPOrDomain(xnet.LocalHostIP),
						Port:    uint32(relayPort),
						User: &protocol.User{
							Account: serial.ToTypedMessage(&vless.Account{
								Id:         userID.String(),
								Encryption: pubB64,
								XorMode:    1,
							}),
						},
					},
				}),
				SenderSettings: serial.ToTypedMessage(&proxyman.SenderConfig{
					StreamSettings: &internet.StreamConfig{
						ProtocolName: "tcp",
						TransportSettings: []*internet.TransportConfig{
							{ProtocolName: "tcp", Settings: serial.ToTypedMessage(&transtcp.Config{})},
						},
						Tcpmasks: []*serial.TypedMessage{serial.ToTypedMessage(cloneConfig(mode.config))},
					},
				}),
			},
		},
	})

	serverCmd, clientCmd := runXrayPair(t, bin, serverConfig, clientConfig)
	defer stopCmd(clientCmd)
	defer stopCmd(serverCmd)
	exerciseTCPClient(t, int(clientPort), payloadSize)

	return analyzeTCPRelay(t, "vless-enc", mode, relay.Snapshots())
}

func runVLESSXHTTPCase(t *testing.T, bin string, mode trafficMode, payloadSize int) caseResult {
	backend := startXOREchoServer(t)
	defer backend.Close()

	serverPort := testingtcp.PickPort()
	relayPort := testingtcp.PickPort()
	clientPort := testingtcp.PickPort()

	relay := startTCPRelay(t, int(relayPort), fmt.Sprintf("127.0.0.1:%d", serverPort))
	defer relay.Close()

	userID := protocol.NewID(uuid.New())
	xhttpConfig := &splithttp.Config{
		Host: "localhost",
		Path: "/sudoku",
		Mode: "auto",
	}

	serverConfig := defaultApps(&core.Config{
		Inbound: []*core.InboundHandlerConfig{
			{
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &xnet.PortList{Range: []*xnet.PortRange{xnet.SinglePortRange(serverPort)}},
					Listen:   xnet.NewIPOrDomain(xnet.LocalHostIP),
					StreamSettings: &internet.StreamConfig{
						ProtocolName: "splithttp",
						TransportSettings: []*internet.TransportConfig{
							{ProtocolName: "splithttp", Settings: serial.ToTypedMessage(xhttpConfig)},
						},
						Tcpmasks: []*serial.TypedMessage{serial.ToTypedMessage(cloneConfig(mode.config))},
					},
				}),
				ProxySettings: serial.ToTypedMessage(&vin.Config{
					Clients: []*protocol.User{
						{
							Account: serial.ToTypedMessage(&vless.Account{
								Id: userID.String(),
							}),
						},
					},
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{ProxySettings: serial.ToTypedMessage(&freedom.Config{})},
		},
	})

	clientConfig := defaultApps(&core.Config{
		Inbound: []*core.InboundHandlerConfig{
			{
				ReceiverSettings: serial.ToTypedMessage(&proxyman.ReceiverConfig{
					PortList: &xnet.PortList{Range: []*xnet.PortRange{xnet.SinglePortRange(clientPort)}},
					Listen:   xnet.NewIPOrDomain(xnet.LocalHostIP),
				}),
				ProxySettings: serial.ToTypedMessage(&dokodemo.Config{
					Address:  xnet.NewIPOrDomain(backend.Address()),
					Port:     uint32(backend.Port()),
					Networks: []xnet.Network{xnet.Network_TCP},
				}),
			},
		},
		Outbound: []*core.OutboundHandlerConfig{
			{
				ProxySettings: serial.ToTypedMessage(&vout.Config{
					Vnext: &protocol.ServerEndpoint{
						Address: xnet.NewIPOrDomain(xnet.LocalHostIP),
						Port:    uint32(relayPort),
						User: &protocol.User{
							Account: serial.ToTypedMessage(&vless.Account{
								Id: userID.String(),
							}),
						},
					},
				}),
				SenderSettings: serial.ToTypedMessage(&proxyman.SenderConfig{
					StreamSettings: &internet.StreamConfig{
						ProtocolName: "splithttp",
						TransportSettings: []*internet.TransportConfig{
							{ProtocolName: "splithttp", Settings: serial.ToTypedMessage(xhttpConfig)},
						},
						Tcpmasks: []*serial.TypedMessage{serial.ToTypedMessage(cloneConfig(mode.config))},
					},
				}),
			},
		},
	})

	serverCmd, clientCmd := runXrayPair(t, bin, serverConfig, clientConfig)
	defer stopCmd(clientCmd)
	defer stopCmd(serverCmd)
	exerciseTCPClient(t, int(clientPort), payloadSize)

	return analyzeTCPRelay(t, "vless-xhttp", mode, relay.Snapshots())
}

func analyzeTCPRelay(t *testing.T, protocol string, mode trafficMode, captures []*tcpCapture) caseResult {
	tables, err := getTables(mode.config)
	if err != nil {
		t.Fatal(err)
	}

	allC2S := make([][]byte, 0, len(captures))
	allS2C := make([][]byte, 0, len(captures))
	for _, capture := range captures {
		c2s, s2c := capture.snapshot()
		if len(c2s) > 0 {
			allC2S = append(allC2S, c2s)
		}
		if len(s2c) > 0 {
			allS2C = append(allS2C, s2c)
		}
	}

	c2sMetrics := metricFromBytes(flattenChunks(allC2S))
	s2cMetrics := metricFromBytes(flattenChunks(allS2C))

	c2sUsed, c2sDecoded, err := analyzePureChunks(tables, allC2S)
	if err != nil {
		t.Fatalf("%s %s pure decode failed: %v", protocol, mode.name, err)
	}
	s2cUsed, s2cDecoded, err := analyzePackedChunks(tables, allS2C)
	if err != nil {
		t.Fatalf("%s %s packed decode failed: %v", protocol, mode.name, err)
	}

	allBytes := append(append([]byte{}, flattenChunks(allC2S)...), flattenChunks(allS2C)...)
	totalMetrics := metricFromBytes(allBytes)
	rotationSeen := len(unionKeys(c2sUsed, s2cUsed))

	return caseResult{
		Protocol:         protocol,
		Mode:             mode.name,
		TotalBytes:       len(allBytes),
		ASCIIBytes:       totalMetrics.asciiBytes,
		ASCIIRatio:       totalMetrics.asciiRatio,
		AvgHammingOnes:   totalMetrics.avgOnes,
		RotationSeen:     rotationSeen,
		RotationExpected: expectedRotation(mode.config),
		DecodedUnits:     c2sDecoded + s2cDecoded,
		ClientToServer: directionResult{
			RawBytes:       len(flattenChunks(allC2S)),
			ASCIIBytes:     c2sMetrics.asciiBytes,
			ASCIIRatio:     c2sMetrics.asciiRatio,
			AvgHammingOnes: c2sMetrics.avgOnes,
			RotationSeen:   len(c2sUsed),
			DecodedUnits:   c2sDecoded,
		},
		ServerToClient: directionResult{
			RawBytes:       len(flattenChunks(allS2C)),
			ASCIIBytes:     s2cMetrics.asciiBytes,
			ASCIIRatio:     s2cMetrics.asciiRatio,
			AvgHammingOnes: s2cMetrics.avgOnes,
			RotationSeen:   len(s2cUsed),
			DecodedUnits:   s2cDecoded,
		},
	}
}

func analyzeUDPRelay(t *testing.T, protocol string, mode trafficMode, c2s [][]byte, s2c [][]byte) caseResult {
	tables, err := getTables(mode.config)
	if err != nil {
		t.Fatal(err)
	}

	c2sMetrics := metricFromBytes(flattenChunks(c2s))
	s2cMetrics := metricFromBytes(flattenChunks(s2c))

	c2sUsed, c2sDecoded, err := analyzePureChunks(tables, c2s)
	if err != nil {
		t.Fatalf("%s %s udp c2s decode failed: %v", protocol, mode.name, err)
	}
	s2cUsed, s2cDecoded, err := analyzePureChunks(tables, s2c)
	if err != nil {
		t.Fatalf("%s %s udp s2c decode failed: %v", protocol, mode.name, err)
	}

	allBytes := append(append([]byte{}, flattenChunks(c2s)...), flattenChunks(s2c)...)
	totalMetrics := metricFromBytes(allBytes)
	rotationSeen := len(unionKeys(c2sUsed, s2cUsed))

	return caseResult{
		Protocol:         protocol,
		Mode:             mode.name,
		TotalBytes:       len(allBytes),
		ASCIIBytes:       totalMetrics.asciiBytes,
		ASCIIRatio:       totalMetrics.asciiRatio,
		AvgHammingOnes:   totalMetrics.avgOnes,
		RotationSeen:     rotationSeen,
		RotationExpected: expectedRotation(mode.config),
		DecodedUnits:     c2sDecoded + s2cDecoded,
		ClientToServer: directionResult{
			RawBytes:       len(flattenChunks(c2s)),
			ASCIIBytes:     c2sMetrics.asciiBytes,
			ASCIIRatio:     c2sMetrics.asciiRatio,
			AvgHammingOnes: c2sMetrics.avgOnes,
			RotationSeen:   len(c2sUsed),
			DecodedUnits:   c2sDecoded,
		},
		ServerToClient: directionResult{
			RawBytes:       len(flattenChunks(s2c)),
			ASCIIBytes:     s2cMetrics.asciiBytes,
			ASCIIRatio:     s2cMetrics.asciiRatio,
			AvgHammingOnes: s2cMetrics.avgOnes,
			RotationSeen:   len(s2cUsed),
			DecodedUnits:   s2cDecoded,
		},
	}
}

type byteMetrics struct {
	asciiBytes int
	asciiRatio float64
	avgOnes    float64
}

func metricFromBytes(b []byte) byteMetrics {
	if len(b) == 0 {
		return byteMetrics{}
	}
	var ascii, ones int
	for _, v := range b {
		if v < 0x80 {
			ascii++
		}
		ones += bitsInByte(v)
	}
	return byteMetrics{
		asciiBytes: ascii,
		asciiRatio: float64(ascii) / float64(len(b)),
		avgOnes:    float64(ones) / float64(len(b)),
	}
}

func bitsInByte(b byte) int {
	n := 0
	for b != 0 {
		n += int(b & 1)
		b >>= 1
	}
	return n
}

func analyzePureChunks(tables []*table, chunks [][]byte) (map[int]int, int, error) {
	if len(tables) == 0 {
		return nil, 0, fmt.Errorf("no sudoku tables")
	}
	used := make(map[int]int)
	decoded := 0
	for _, chunk := range chunks {
		hintBuf := make([]byte, 0, 4)
		tableIndex := 0
		for _, b := range chunk {
			t := tables[tableIndex%len(tables)]
			if !t.layout.isHint(b) {
				continue
			}
			hintBuf = append(hintBuf, b)
			if len(hintBuf) < 4 {
				continue
			}
			keyBytes := sort4([4]byte{hintBuf[0], hintBuf[1], hintBuf[2], hintBuf[3]})
			key := packKey(keyBytes)
			if _, ok := t.decode[key]; !ok {
				return nil, 0, fmt.Errorf("invalid pure tuple at table %d", tableIndex%len(tables))
			}
			used[tableIndex%len(tables)]++
			decoded++
			tableIndex++
			hintBuf = hintBuf[:0]
		}
		if len(hintBuf) != 0 {
			return nil, 0, fmt.Errorf("leftover pure hints")
		}
	}
	return used, decoded, nil
}

func analyzePackedChunks(tables []*table, chunks [][]byte) (map[int]int, int, error) {
	layouts := tablesToLayouts(tables)
	if len(layouts) == 0 {
		return nil, 0, fmt.Errorf("no sudoku layouts")
	}
	used := make(map[int]int)
	decoded := 0
	for _, chunk := range chunks {
		var bitBuf uint64
		var bitCount int
		groupIndex := 0
		for _, b := range chunk {
			layout := layouts[groupIndex%len(layouts)]
			if !layout.isHint(b) {
				if b == layout.padMarker {
					bitBuf = 0
					bitCount = 0
				}
				continue
			}
			group, ok := layout.decodeGroup(b)
			if !ok {
				return nil, 0, fmt.Errorf("invalid packed byte %d", b)
			}
			used[groupIndex%len(layouts)]++
			groupIndex++
			bitBuf = (bitBuf << 6) | uint64(group)
			bitCount += 6
			for bitCount >= 8 {
				bitCount -= 8
				decoded++
				if bitCount > 0 {
					bitBuf &= (uint64(1) << bitCount) - 1
				} else {
					bitBuf = 0
				}
			}
		}
	}
	return used, decoded, nil
}

func expectedRotation(cfg *Config) int {
	tables, err := getTables(cfg)
	if err != nil {
		return 0
	}
	return len(tables)
}

func unionKeys(a, b map[int]int) map[int]struct{} {
	out := make(map[int]struct{}, len(a)+len(b))
	for k := range a {
		out[k] = struct{}{}
	}
	for k := range b {
		out[k] = struct{}{}
	}
	return out
}

func flattenChunks(chunks [][]byte) []byte {
	total := 0
	for _, chunk := range chunks {
		total += len(chunk)
	}
	out := make([]byte, 0, total)
	for _, chunk := range chunks {
		out = append(out, chunk...)
	}
	return out
}

func cloneConfig(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}
	out := proto.Clone(cfg).(*Config)
	return out
}

func defaultApps(cfg *core.Config) *core.Config {
	cfg.App = append(cfg.App,
		serial.ToTypedMessage(&log.Config{
			ErrorLogLevel: clog.Severity_Warning,
			ErrorLogType:  log.LogType_Console,
		}),
		serial.ToTypedMessage(&dispatcher.Config{}),
		serial.ToTypedMessage(&proxyman.InboundConfig{}),
		serial.ToTypedMessage(&proxyman.OutboundConfig{}),
	)
	return cfg
}

func buildE2EBinary(t *testing.T) string {
	t.Helper()
	e2eBinaryOnce.Do(func() {
		tempDir, err := os.MkdirTemp("", "xray-sudoku-e2e-*")
		if err != nil {
			e2eBinaryErr = err
			return
		}
		e2eBinaryPath = filepath.Join(tempDir, "xray.test")
		cmd := exec.Command("go", "build", "-o", e2eBinaryPath, "./main")
		cmd.Dir = repoRoot(t)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		e2eBinaryErr = cmd.Run()
	})
	if e2eBinaryErr != nil {
		t.Fatal(e2eBinaryErr)
	}
	return e2eBinaryPath
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("failed to locate repo root")
		}
		dir = parent
	}
}

func runXrayPair(t *testing.T, bin string, serverCfg, clientCfg *core.Config) (*exec.Cmd, *exec.Cmd) {
	t.Helper()
	serverCmd := runXray(t, bin, serverCfg)

	time.Sleep(500 * time.Millisecond)

	clientCmd := runXray(t, bin, clientCfg)

	time.Sleep(1500 * time.Millisecond)
	return serverCmd, clientCmd
}

func runXray(t *testing.T, bin string, cfg *core.Config) *exec.Cmd {
	t.Helper()
	cfgBytes, err := proto.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-config=stdin:", "-format=pb")
	cmd.Stdin = bytes.NewReader(cfgBytes)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd
}

func stopCmd(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

func startTCPRelay(t *testing.T, listenPort int, target string) *tcpRelay {
	t.Helper()
	ln, err := stdnet.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", listenPort))
	if err != nil {
		t.Fatal(err)
	}
	r := &tcpRelay{
		listener: ln,
		target:   target,
		stopCh:   make(chan struct{}),
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-r.stopCh:
					return
				default:
				}
				return
			}
			targetConn, err := stdnet.Dial("tcp", target)
			if err != nil {
				_ = conn.Close()
				continue
			}
			capture := &tcpCapture{}
			r.mu.Lock()
			r.captures = append(r.captures, capture)
			r.mu.Unlock()
			r.wg.Add(1)
			go func(client, server stdnet.Conn, cap *tcpCapture) {
				defer r.wg.Done()
				defer client.Close()
				defer server.Close()
				var inner sync.WaitGroup
				inner.Add(2)
				go func() {
					defer inner.Done()
					_, _ = io.Copy(server, io.TeeReader(client, &captureWriter{capture: cap, dir: "c2s"}))
					if tcp, ok := server.(*stdnet.TCPConn); ok {
						_ = tcp.CloseWrite()
					}
				}()
				go func() {
					defer inner.Done()
					_, _ = io.Copy(client, io.TeeReader(server, &captureWriter{capture: cap, dir: "s2c"}))
					if tcp, ok := client.(*stdnet.TCPConn); ok {
						_ = tcp.CloseWrite()
					}
				}()
				inner.Wait()
			}(conn, targetConn, capture)
		}
	}()
	return r
}

func (r *tcpRelay) Close() {
	close(r.stopCh)
	_ = r.listener.Close()
	r.wg.Wait()
}

func (r *tcpRelay) Snapshots() []*tcpCapture {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*tcpCapture, 0, len(r.captures))
	for _, capture := range r.captures {
		out = append(out, capture)
	}
	return out
}

func (c *tcpCapture) snapshot() ([]byte, []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte{}, c.c2s...), append([]byte{}, c.s2c...)
}

type captureWriter struct {
	capture *tcpCapture
	dir     string
}

func (w *captureWriter) Write(p []byte) (int, error) {
	w.capture.mu.Lock()
	defer w.capture.mu.Unlock()
	if w.dir == "c2s" {
		w.capture.c2s = append(w.capture.c2s, p...)
	} else {
		w.capture.s2c = append(w.capture.s2c, p...)
	}
	return len(p), nil
}

func startUDPRelay(t *testing.T, listenPort, targetPort int) *udpRelay {
	t.Helper()
	conn, err := stdnet.ListenPacket("udp", fmt.Sprintf("127.0.0.1:%d", listenPort))
	if err != nil {
		t.Fatal(err)
	}
	targetAddr := &stdnet.UDPAddr{IP: stdnet.IPv4(127, 0, 0, 1), Port: targetPort}
	r := &udpRelay{
		conn:   conn,
		target: targetAddr,
		stopCh: make(chan struct{}),
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		buf := make([]byte, 64*1024)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				select {
				case <-r.stopCh:
					return
				default:
				}
				return
			}
			payload := append([]byte{}, buf[:n]...)
			udpAddr := addr.(*stdnet.UDPAddr)
			if udpAddr.IP.Equal(r.target.IP) && udpAddr.Port == r.target.Port {
				r.captureMu.Lock()
				r.s2c = append(r.s2c, payload)
				r.captureMu.Unlock()
				r.clientMu.Lock()
				client := r.client
				r.clientMu.Unlock()
				if client != nil {
					_, _ = conn.WriteTo(payload, client)
				}
				continue
			}
			r.clientMu.Lock()
			r.client = udpAddr
			r.clientMu.Unlock()
			r.captureMu.Lock()
			r.c2s = append(r.c2s, payload)
			r.captureMu.Unlock()
			_, _ = conn.WriteTo(payload, r.target)
		}
	}()
	return r
}

func (r *udpRelay) Close() {
	close(r.stopCh)
	_ = r.conn.Close()
	r.wg.Wait()
}

func (r *udpRelay) Snapshots() ([][]byte, [][]byte) {
	r.captureMu.Lock()
	defer r.captureMu.Unlock()
	c2s := make([][]byte, 0, len(r.c2s))
	s2c := make([][]byte, 0, len(r.s2c))
	for _, packet := range r.c2s {
		c2s = append(c2s, append([]byte{}, packet...))
	}
	for _, packet := range r.s2c {
		s2c = append(s2c, append([]byte{}, packet...))
	}
	return c2s, s2c
}

type xorEchoServer struct {
	ln stdnet.Listener
	wg sync.WaitGroup
}

func startXOREchoServer(t *testing.T) *xorEchoServer {
	t.Helper()
	ln, err := stdnet.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &xorEchoServer{ln: ln}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			s.wg.Add(1)
			go func(c stdnet.Conn) {
				defer s.wg.Done()
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					for i := 0; i < n; i++ {
						buf[i] ^= 'c'
					}
					if _, err := c.Write(buf[:n]); err != nil {
						return
					}
					for i := 0; i < n; i++ {
						buf[i] ^= 'c'
					}
				}
			}(conn)
		}
	}()
	return s
}

func (s *xorEchoServer) Address() xnet.Address {
	return xnet.IPAddress(s.ln.Addr().(*stdnet.TCPAddr).IP)
}

func (s *xorEchoServer) Port() xnet.Port {
	return xnet.Port(s.ln.Addr().(*stdnet.TCPAddr).Port)
}

func (s *xorEchoServer) Close() {
	_ = s.ln.Close()
	s.wg.Wait()
}

func startTLSEchoDecoy(t *testing.T, c *cert.Certificate) *tlsDecoy {
	t.Helper()
	certPEM, keyPEM := c.ToPEM()
	keyPair, err := cryptotls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	config := &cryptotls.Config{
		Certificates: []cryptotls.Certificate{keyPair},
	}
	ln, err := stdnet.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsLn := cryptotls.NewListener(ln, config)
	d := &tlsDecoy{
		ln:   tlsLn,
		done: make(chan struct{}),
	}
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for {
			conn, err := tlsLn.Accept()
			if err != nil {
				return
			}
			d.wg.Add(1)
			go func(c stdnet.Conn) {
				defer d.wg.Done()
				defer c.Close()
				buf := make([]byte, 2048)
				_, _ = c.Read(buf)
				_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK"))
			}(conn)
		}
	}()
	return d
}

func (d *tlsDecoy) Port() int {
	return d.ln.Addr().(*stdnet.TCPAddr).Port
}

func (d *tlsDecoy) Close() {
	_ = d.ln.Close()
	d.wg.Wait()
}

func exerciseTCPClient(t *testing.T, port int, payloadSize int) {
	t.Helper()
	if err := exerciseTCPClientErr(t, port, payloadSize); err != nil {
		t.Fatal(err)
	}
}

func exerciseTCPClientErr(t *testing.T, port int, payloadSize int) error {
	conn := waitTCPConn(t, port, 10*time.Second)
	defer conn.Close()
	payload := make([]byte, payloadSize)
	if _, err := rand.Read(payload); err != nil {
		return err
	}
	offset := 0
	for offset < len(payload) {
		chunk := 1024
		if remain := len(payload) - offset; remain < chunk {
			chunk = remain
		}
		part := payload[offset : offset+chunk]
		if _, err := conn.Write(part); err != nil {
			return err
		}
		resp := make([]byte, chunk)
		if _, err := io.ReadFull(conn, resp); err != nil {
			return err
		}
		for i := range part {
			if resp[i] != (part[i] ^ 'c') {
				return fmt.Errorf("unexpected xor response at offset %d", offset+i)
			}
		}
		offset += chunk
	}
	return nil
}

func firstChunkLen(chunks [][]byte) int {
	if len(chunks) == 0 {
		return 0
	}
	return len(chunks[0])
}

func waitTCPConn(t *testing.T, port int, timeout time.Duration) stdnet.Conn {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		conn, err := stdnet.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
		if err == nil {
			return conn
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func mustX25519Keypair(t *testing.T) ([]byte, []byte) {
	t.Helper()
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv.Bytes(), priv.PublicKey().Bytes()
}

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	out := make([]byte, len(s)/2)
	if _, err := hex.Decode(out, []byte(s)); err != nil {
		t.Fatal(err)
	}
	return out
}

func init() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		os.Exit(130)
	}()
}
