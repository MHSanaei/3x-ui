package web

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServerStopWhenXrayNotRunningSucceeds(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	s := NewServer()
	if err := s.Stop(); err != nil {
		t.Fatalf("expected nil error when stopping server with unstarted xray, got: %v", err)
	}
}

func TestServerStopPanelOnlyWithPreclosedListenerSucceeds(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	s := NewServer()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen failed: %v", err)
	}
	s.listener = ln
	s.httpServer = &http.Server{}
	go func() { _ = s.httpServer.Serve(ln) }()
	time.Sleep(10 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.httpServer.Shutdown(shutdownCtx)

	if err := s.StopPanelOnly(); err != nil {
		t.Fatalf("expected nil error when listener is already closed, got: %v", err)
	}
}
