package reflect_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/xtls/xray-core/common/protocol"
	. "github.com/xtls/xray-core/common/reflect"
	cserial "github.com/xtls/xray-core/common/serial"
	iserial "github.com/xtls/xray-core/infra/conf/serial"
	"github.com/xtls/xray-core/proxy/shadowsocks"
)

func TestMashalAccount(t *testing.T) {
	account := &shadowsocks.Account{
		Password:   "shadowsocks-password",
		CipherType: shadowsocks.CipherType_CHACHA20_POLY1305,
	}

	user := &protocol.User{
		Level:   0,
		Email:   "love@v2ray.com",
		Account: cserial.ToTypedMessage(account),
	}

	j, ok := MarshalToJson(user, false)
	if !ok || strings.Contains(j, "_TypedMessage_") {

		t.Error("marshal account failed")
	}

	kws := []string{"CHACHA20_POLY1305", "cipherType", "shadowsocks-password"}
	for _, kw := range kws {
		if !strings.Contains(j, kw) {
			t.Error("marshal account failed")
		}
	}
	// t.Log(j)
}

func TestMashalStruct(t *testing.T) {
	type Foo = struct {
		N   int                             `json:"n"`
		Np  *int                            `json:"np"`
		S   string                          `json:"s"`
		Arr *[]map[string]map[string]string `json:"arr"`
	}

	n := 1
	np := &n
	arr := make([]map[string]map[string]string, 0)
	m1 := make(map[string]map[string]string, 0)
	m2 := make(map[string]string, 0)
	m2["hello"] = "world"
	m1["foo"] = m2

	arr = append(arr, m1)

	f1 := Foo{
		N:   n,
		Np:  np,
		S:   "hello",
		Arr: &arr,
	}

	s, ok1 := MarshalToJson(f1, true)
	sp, ok2 := MarshalToJson(&f1, true)

	if !ok1 || !ok2 || s != sp {
		t.Error("marshal failed")
	}

	f2 := Foo{}
	if json.Unmarshal([]byte(s), &f2) != nil {
		t.Error("json unmarshal failed")
	}

	v := (*f2.Arr)[0]["foo"]["hello"]

	if f1.N != f2.N || *(f1.Np) != *(f2.Np) || f1.S != f2.S || v != "world" {
		t.Error("f1 not equal to f2")
	}
}

func TestMarshalConfigJson(t *testing.T) {

	buf := bytes.NewBufferString(getConfig())
	config, err := iserial.DecodeJSONConfig(buf)
	if err != nil {
		t.Error("decode JSON config failed")
	}

	bc, err := config.Build()
	if err != nil {
		t.Error("build core config failed")
	}

	tmsg := cserial.ToTypedMessage(bc)
	tc, ok := MarshalToJson(tmsg, true)
	if !ok {
		t.Error("marshal config failed")
	}

	// t.Log(tc)

	keywords := []string{
		"4784f9b8-a879-4fec-9718-ebddefa47750",
		"bing.com",
		"inboundTag",
		"level",
		"stats",
		"userDownlink",
		"userUplink",
		"system",
		"inboundDownlink",
		"outboundUplink",
		"XHTTP_IN",
		"\"host\": \"bing.com\"",
		"scMaxEachPostBytes",
		"\"from\": 100",
		"\"to\": 1000",
		"\"from\": 1000000",
		"\"to\": 1000000",
	}
	for _, kw := range keywords {
		if !strings.Contains(tc, kw) {
			t.Log("config.json:", tc)
			t.Error("keyword not found:", kw)
			break
		}
	}
}

func getConfig() string {
	return `{
  "log": {
    "loglevel": "debug"
  },
  "stats": {},
  "policy": {
    "levels": {
      "0": {
        "statsUserUplink": true,
        "statsUserDownlink": true
      }
    },
    "system": {
      "statsInboundUplink": true,
      "statsInboundDownlink": true,
      "statsOutboundUplink": true,
      "statsOutboundDownlink": true
    }
  },
  "inbounds": [
    {
      "tag": "agentin",
      "protocol": "http",
      "port": 18080,
      "listen": "127.0.0.1",
      "settings": {}
    },
    {
      "listen": "127.0.0.1",
      "port": 10085,
      "protocol": "dokodemo-door",
      "settings": {
        "address": "127.0.0.1"
      },
      "tag": "api-in"
    }
  ],
  "api": {
    "tag": "api",
    "services": [
      "HandlerService",
      "StatsService"
    ]
  },
  "routing": {
    "rules": [
      {
        "inboundTag": [
          "api-in"
        ],
        "outboundTag": "api"
      }
    ],
    "domainStrategy": "AsIs"
  },
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "1.2.3.4",
            "port": 1234,
            "users": [
              {
                "id": "4784f9b8-a879-4fec-9718-ebddefa47750",
                "encryption": "none"
              }
            ]
          }
        ]
      },
      "tag": "XHTTP_IN",
      "streamSettings": {
        "network": "xhttp",
        "xhttpSettings": {
          "host": "bing.com",
          "path": "/xhttp_client_upload",
          "mode": "auto",
          "extra": {
            "noSSEHeader": false,
            "scMaxEachPostBytes": 1000000,
            "scMaxBufferedPosts": 30,
            "xPaddingBytes": "100-1000"
          }
        },
        "sockopt": {
          "tcpFastOpen": true,
          "acceptProxyProtocol": false,
          "tcpcongestion": "bbr",
          "tcpMptcp": true
        }
      },
      "sniffing": {
        "enabled": true,
        "destOverride": [
          "http",
          "tls",
          "quic"
        ],
        "metadataOnly": false,
        "routeOnly": true
      }
    }
  ]
}`
}
