package http_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	. "github.com/xtls/xray-core/transport/internet/headers/http"
)

func TestReaderWriter(t *testing.T) {
	cache := buf.New()
	b := buf.New()
	common.Must2(b.WriteString("abcd" + ENDING))
	writer := NewHeaderWriter(b)
	err := writer.Write(cache)
	common.Must(err)
	if v := cache.Len(); v != 8 {
		t.Error("cache len: ", v)
	}
	_, err = cache.Write([]byte{'e', 'f', 'g'})
	common.Must(err)

	reader := &HeaderReader{}
	_, err = reader.Read(cache)
	if err != nil && !strings.HasPrefix(err.Error(), "malformed HTTP request") {
		t.Error("unknown error ", err)
	}
}

func TestRequestHeader(t *testing.T) {
	auth, err := NewAuthenticator(context.Background(), &Config{
		Request: &RequestConfig{
			Uri: []string{"/"},
			Header: []*Header{
				{
					Name:  "Test",
					Value: []string{"Value"},
				},
			},
		},
	})
	common.Must(err)

	cache := buf.New()
	err = auth.GetClientWriter().Write(cache)
	common.Must(err)

	if cache.String() != "GET / HTTP/1.1\r\nTest: Value\r\n\r\n" {
		t.Error("cache: ", cache.String())
	}
}

func TestLongRequestHeader(t *testing.T) {
	payload := make([]byte, buf.Size+2)
	common.Must2(rand.Read(payload[:buf.Size-2]))
	copy(payload[buf.Size-2:], ENDING)
	payload = append(payload, []byte("abcd")...)

	reader := HeaderReader{}
	_, err := reader.Read(bytes.NewReader(payload))

	if err != nil && !(strings.HasPrefix(err.Error(), "invalid") || strings.HasPrefix(err.Error(), "malformed")) {
		t.Error("unknown error ", err)
	}
}

func TestConnection(t *testing.T) {
	auth, err := NewAuthenticator(context.Background(), &Config{
		Request: &RequestConfig{
			Method: &Method{Value: "Post"},
			Uri:    []string{"/testpath"},
			Header: []*Header{
				{
					Name:  "Host",
					Value: []string{"www.example.com", "www.google.com"},
				},
				{
					Name:  "User-Agent",
					Value: []string{"Test-Agent"},
				},
			},
		},
		Response: &ResponseConfig{
			Version: &Version{
				Value: "1.1",
			},
			Status: &Status{
				Code:   "404",
				Reason: "Not Found",
			},
		},
	})
	common.Must(err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	common.Must(err)

	go func() {
		conn, err := listener.Accept()
		common.Must(err)
		authConn := auth.Server(conn)
		b := make([]byte, 256)
		for {
			n, err := authConn.Read(b)
			if err != nil {
				break
			}
			_, err = authConn.Write(b[:n])
			common.Must(err)
		}
	}()

	conn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	common.Must(err)

	authConn := auth.Client(conn)
	defer authConn.Close()

	authConn.Write([]byte("Test payload"))
	authConn.Write([]byte("Test payload 2"))

	expectedResponse := "Test payloadTest payload 2"
	actualResponse := make([]byte, 256)
	deadline := time.Now().Add(time.Second * 5)
	totalBytes := 0
	for {
		n, err := authConn.Read(actualResponse[totalBytes:])
		common.Must(err)
		totalBytes += n
		if totalBytes >= len(expectedResponse) || time.Now().After(deadline) {
			break
		}
	}

	if string(actualResponse[:totalBytes]) != expectedResponse {
		t.Error("response: ", string(actualResponse[:totalBytes]))
	}
}

func TestConnectionInvPath(t *testing.T) {
	auth, err := NewAuthenticator(context.Background(), &Config{
		Request: &RequestConfig{
			Method: &Method{Value: "Post"},
			Uri:    []string{"/testpath"},
			Header: []*Header{
				{
					Name:  "Host",
					Value: []string{"www.example.com", "www.google.com"},
				},
				{
					Name:  "User-Agent",
					Value: []string{"Test-Agent"},
				},
			},
		},
		Response: &ResponseConfig{
			Version: &Version{
				Value: "1.1",
			},
			Status: &Status{
				Code:   "404",
				Reason: "Not Found",
			},
		},
	})
	common.Must(err)

	authR, err := NewAuthenticator(context.Background(), &Config{
		Request: &RequestConfig{
			Method: &Method{Value: "Post"},
			Uri:    []string{"/testpathErr"},
			Header: []*Header{
				{
					Name:  "Host",
					Value: []string{"www.example.com", "www.google.com"},
				},
				{
					Name:  "User-Agent",
					Value: []string{"Test-Agent"},
				},
			},
		},
		Response: &ResponseConfig{
			Version: &Version{
				Value: "1.1",
			},
			Status: &Status{
				Code:   "404",
				Reason: "Not Found",
			},
		},
	})
	common.Must(err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	common.Must(err)

	go func() {
		conn, err := listener.Accept()
		common.Must(err)
		authConn := auth.Server(conn)
		b := make([]byte, 256)
		for {
			n, err := authConn.Read(b)
			if err != nil {
				authConn.Close()
				break
			}
			_, err = authConn.Write(b[:n])
			common.Must(err)
		}
	}()

	conn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	common.Must(err)

	authConn := authR.Client(conn)
	defer authConn.Close()

	authConn.Write([]byte("Test payload"))
	authConn.Write([]byte("Test payload 2"))

	expectedResponse := "Test payloadTest payload 2"
	actualResponse := make([]byte, 256)
	deadline := time.Now().Add(time.Second * 5)
	totalBytes := 0
	for {
		n, err := authConn.Read(actualResponse[totalBytes:])
		if err == nil {
			t.Error("Error Expected", err)
		} else {
			return
		}
		totalBytes += n
		if totalBytes >= len(expectedResponse) || time.Now().After(deadline) {
			break
		}
	}
}

func TestConnectionInvReq(t *testing.T) {
	auth, err := NewAuthenticator(context.Background(), &Config{
		Request: &RequestConfig{
			Method: &Method{Value: "Post"},
			Uri:    []string{"/testpath"},
			Header: []*Header{
				{
					Name:  "Host",
					Value: []string{"www.example.com", "www.google.com"},
				},
				{
					Name:  "User-Agent",
					Value: []string{"Test-Agent"},
				},
			},
		},
		Response: &ResponseConfig{
			Version: &Version{
				Value: "1.1",
			},
			Status: &Status{
				Code:   "404",
				Reason: "Not Found",
			},
		},
	})
	common.Must(err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	common.Must(err)

	go func() {
		conn, err := listener.Accept()
		common.Must(err)
		authConn := auth.Server(conn)
		b := make([]byte, 256)
		for {
			n, err := authConn.Read(b)
			if err != nil {
				authConn.Close()
				break
			}
			_, err = authConn.Write(b[:n])
			common.Must(err)
		}
	}()

	conn, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
	common.Must(err)

	conn.Write([]byte("ABCDEFGHIJKMLN\r\n\r\n"))
	l, _, err := bufio.NewReader(conn).ReadLine()
	common.Must(err)
	if !strings.HasPrefix(string(l), "HTTP/1.1 400 Bad Request") {
		t.Error("Resp to non http conn", string(l))
	}
}
