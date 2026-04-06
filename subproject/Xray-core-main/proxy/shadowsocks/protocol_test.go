package shadowsocks_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	. "github.com/xtls/xray-core/proxy/shadowsocks"
)

func toAccount(a *Account) protocol.Account {
	account, err := a.AsAccount()
	common.Must(err)
	return account
}

func equalRequestHeader(x, y *protocol.RequestHeader) bool {
	return cmp.Equal(x, y, cmp.Comparer(func(x, y protocol.RequestHeader) bool {
		return x == y
	}))
}

func TestUDPEncodingDecoding(t *testing.T) {
	testRequests := []protocol.RequestHeader{
		{
			Version: Version,
			Command: protocol.RequestCommandUDP,
			Address: net.LocalHostIP,
			Port:    1234,
			User: &protocol.MemoryUser{
				Email: "love@example.com",
				Account: toAccount(&Account{
					Password:   "password",
					CipherType: CipherType_AES_128_GCM,
				}),
			},
		},
		{
			Version: Version,
			Command: protocol.RequestCommandUDP,
			Address: net.LocalHostIP,
			Port:    1234,
			User: &protocol.MemoryUser{
				Email: "love@example.com",
				Account: toAccount(&Account{
					Password:   "123",
					CipherType: CipherType_NONE,
				}),
			},
		},
	}

	for _, request := range testRequests {
		data := buf.New()
		common.Must2(data.WriteString("test string"))
		encodedData, err := EncodeUDPPacket(&request, data.Bytes())
		common.Must(err)

		validator := new(Validator)
		validator.Add(request.User)
		decodedRequest, decodedData, err := DecodeUDPPacket(validator, encodedData)
		common.Must(err)

		if r := cmp.Diff(decodedData.Bytes(), data.Bytes()); r != "" {
			t.Error("data: ", r)
		}

		if equalRequestHeader(decodedRequest, &request) == false {
			t.Error("different request")
		}
	}
}

func TestUDPDecodingWithPayloadTooShort(t *testing.T) {
	testAccounts := []protocol.Account{
		toAccount(&Account{
			Password:   "password",
			CipherType: CipherType_AES_128_GCM,
		}),
		toAccount(&Account{
			Password:   "password",
			CipherType: CipherType_NONE,
		}),
	}

	for _, account := range testAccounts {
		data := buf.New()
		data.WriteString("short payload")
		validator := new(Validator)
		validator.Add(&protocol.MemoryUser{
			Account: account,
		})
		_, _, err := DecodeUDPPacket(validator, data)
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestTCPRequest(t *testing.T) {
	cases := []struct {
		request *protocol.RequestHeader
		payload []byte
	}{
		{
			request: &protocol.RequestHeader{
				Version: Version,
				Command: protocol.RequestCommandTCP,
				Address: net.LocalHostIP,
				Port:    1234,
				User: &protocol.MemoryUser{
					Email: "love@example.com",
					Account: toAccount(&Account{
						Password:   "tcp-password",
						CipherType: CipherType_AES_128_GCM,
					}),
				},
			},
			payload: []byte("test string"),
		},
		{
			request: &protocol.RequestHeader{
				Version: Version,
				Command: protocol.RequestCommandTCP,
				Address: net.LocalHostIPv6,
				Port:    1234,
				User: &protocol.MemoryUser{
					Email: "love@example.com",
					Account: toAccount(&Account{
						Password:   "password",
						CipherType: CipherType_AES_256_GCM,
					}),
				},
			},
			payload: []byte("test string"),
		},
		{
			request: &protocol.RequestHeader{
				Version: Version,
				Command: protocol.RequestCommandTCP,
				Address: net.DomainAddress("example.com"),
				Port:    1234,
				User: &protocol.MemoryUser{
					Email: "love@example.com",
					Account: toAccount(&Account{
						Password:   "password",
						CipherType: CipherType_CHACHA20_POLY1305,
					}),
				},
			},
			payload: []byte("test string"),
		},
	}

	runTest := func(request *protocol.RequestHeader, payload []byte) {
		data := buf.New()
		common.Must2(data.Write(payload))

		cache := buf.New()
		defer cache.Release()

		writer, err := WriteTCPRequest(request, cache)
		common.Must(err)

		common.Must(writer.WriteMultiBuffer(buf.MultiBuffer{data}))

		validator := new(Validator)
		validator.Add(request.User)
		decodedRequest, reader, err := ReadTCPSession(validator, cache)
		common.Must(err)
		if equalRequestHeader(decodedRequest, request) == false {
			t.Error("different request")
		}

		decodedData, err := reader.ReadMultiBuffer()
		common.Must(err)
		if r := cmp.Diff(decodedData[0].Bytes(), payload); r != "" {
			t.Error("data: ", r)
		}
	}

	for _, test := range cases {
		runTest(test.request, test.payload)
	}
}

func TestUDPReaderWriter(t *testing.T) {
	user := &protocol.MemoryUser{
		Account: toAccount(&Account{
			Password:   "test-password",
			CipherType: CipherType_CHACHA20_POLY1305,
		}),
	}
	cache := buf.New()
	defer cache.Release()

	writer := &UDPWriter{
		Writer: cache,
		Request: &protocol.RequestHeader{
			Version: Version,
			Address: net.DomainAddress("example.com"),
			Port:    123,
			User:    user,
		},
	}

	reader := &UDPReader{
		Reader: cache,
		User:   user,
	}

	{
		b := buf.New()
		common.Must2(b.WriteString("test payload"))
		common.Must(writer.WriteMultiBuffer(buf.MultiBuffer{b}))

		payload, err := reader.ReadMultiBuffer()
		common.Must(err)
		if payload[0].String() != "test payload" {
			t.Error("unexpected output: ", payload[0].String())
		}
	}

	{
		b := buf.New()
		common.Must2(b.WriteString("test payload 2"))
		common.Must(writer.WriteMultiBuffer(buf.MultiBuffer{b}))

		payload, err := reader.ReadMultiBuffer()
		common.Must(err)
		if payload[0].String() != "test payload 2" {
			t.Error("unexpected output: ", payload[0].String())
		}
	}
}
