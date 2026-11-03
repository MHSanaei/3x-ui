package job

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"github.com/valyala/fasthttp"
)

func stallingListener(t *testing.T) (addr string, release func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				<-done
				_ = c.Close()
			}(conn)
			go func(c net.Conn) { _, _ = io.Copy(io.Discard, c) }(conn)
		}
	}()
	return ln.Addr().String(), func() {
		_ = ln.Close()
		<-done
	}
}

func TestExternalInformClient_StalledReceiverTimesOut(t *testing.T) {
	addr, release := stallingListener(t)
	defer release()

	request := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(request)
	request.Header.SetMethod("POST")
	request.Header.SetContentType("application/json; charset=UTF-8")
	request.SetBody([]byte(`{"clientTraffics":[],"inboundTraffics":[]}`))
	request.SetRequestURI("http://" + addr + "/inform")
	request.Header.SetConnectionClose()

	response := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(response)

	start := time.Now()
	err := externalInformClient.DoTimeout(request, response, externalInformTimeout)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a receiver that never answers must produce an error, not a completed request")
	}
	if !errors.Is(err, fasthttp.ErrTimeout) {
		t.Errorf("want a timeout error, got %v", err)
	}
	if elapsed > externalInformTimeout+2*time.Second {
		t.Errorf("call took %v, must be bounded by %v", elapsed, externalInformTimeout)
	}
}

func TestExternalInformClient_HasDeadlines(t *testing.T) {
	if externalInformClient.ReadTimeout <= 0 || externalInformClient.WriteTimeout <= 0 {
		t.Fatal("the traffic-notify client must carry read and write deadlines")
	}
	if externalInformTimeout >= 5*time.Second {
		t.Errorf("the call budget (%v) must stay under the 5s traffic poll cadence", externalInformTimeout)
	}
}

func TestInformSkippedWhenNothingToReport(t *testing.T) {
	j := &XrayTrafficJob{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		j.informTrafficToExternalAPI(nil, nil)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("an empty payload must return before touching settings or the network")
	}
}

func TestExternalTrafficInform_DeltaAccumulationAndClear(t *testing.T) {
	j := NewXrayTrafficJob()
	// Set an explicit lastInformTime in the distant future to ensure no premature flush during accumulation
	j.lastInformTime = time.Now().Add(10 * time.Minute)

	// Tick 1: client A and inbound 1
	inbounds1 := []*xray.Traffic{
		{Tag: "inbound-1", Up: 100, Down: 200, IsInbound: true},
	}
	clients1 := []*xray.ClientTraffic{
		{Email: "user1@example.com", Up: 100, Down: 200, LastOnline: 1000},
	}
	j.informTrafficToExternalAPI(inbounds1, clients1)

	// Tick 2: more traffic for client A, new traffic for client B
	inbounds2 := []*xray.Traffic{
		{Tag: "inbound-1", Up: 50, Down: 50, IsInbound: true},
		{Tag: "inbound-2", Up: 300, Down: 0, IsInbound: true},
	}
	clients2 := []*xray.ClientTraffic{
		{Email: "user1@example.com", Up: 50, Down: 50, LastOnline: 2000},
		{Email: "user2@example.com", Up: 300, Down: 0, LastOnline: 1500},
	}
	j.informTrafficToExternalAPI(inbounds2, clients2)

	j.informMu.Lock()
	if len(j.accumInbounds) != 2 {
		t.Fatalf("expected 2 accumulated inbounds, got %d", len(j.accumInbounds))
	}
	if in1 := j.accumInbounds["inbound-1"]; in1 == nil || in1.Up != 150 || in1.Down != 250 {
		t.Errorf("inbound-1 accumulated = %+v, want Up=150 Down=250", in1)
	}
	if in2 := j.accumInbounds["inbound-2"]; in2 == nil || in2.Up != 300 || in2.Down != 0 {
		t.Errorf("inbound-2 accumulated = %+v, want Up=300 Down=0", in2)
	}

	if len(j.accumClients) != 2 {
		t.Fatalf("expected 2 accumulated clients, got %d", len(j.accumClients))
	}
	if c1 := j.accumClients["user1@example.com"]; c1 == nil || c1.Up != 150 || c1.Down != 250 || c1.LastOnline != 2000 {
		t.Errorf("user1 accumulated = %+v, want Up=150 Down=250 LastOnline=2000", c1)
	}
	if c2 := j.accumClients["user2@example.com"]; c2 == nil || c2.Up != 300 || c2.Down != 0 {
		t.Errorf("user2 accumulated = %+v, want Up=300 Down=0", c2)
	}
	j.informMu.Unlock()

	// Clear test
	j.clearAccumulatedTraffic()
	j.informMu.Lock()
	if len(j.accumInbounds) != 0 || len(j.accumClients) != 0 {
		t.Errorf("clearAccumulatedTraffic failed to reset buffers: %d inbounds, %d clients", len(j.accumInbounds), len(j.accumClients))
	}
	j.informMu.Unlock()
}
