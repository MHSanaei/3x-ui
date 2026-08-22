package pia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAuthClientSuccessAndReject(t *testing.T) {
	successFixture, err := os.ReadFile(filepath.Join("testdata", "auth", "success.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.FormValue("username") != "p123" || r.FormValue("password") != "password" {
			t.Errorf("unexpected credentials")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(successFixture)
	}))
	defer server.Close()
	client := NewAuthClient(server.URL)
	token, err := client.Authenticate(context.Background(), "p123", []byte("password"))
	if err != nil || string(token.Value) != "test-token-value-that-is-long-enough" {
		t.Fatalf("unexpected auth result: token=%q err=%v", token.Value, err)
	}

	rejected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer rejected.Close()
	client = NewAuthClient(rejected.URL)
	_, err = client.Authenticate(context.Background(), "p123", []byte("wrong"))
	if CodeOf(err) != CodeInvalidCredentials {
		t.Fatalf("got %s, want %s", CodeOf(err), CodeInvalidCredentials)
	}
}

func TestAuthClientRejectsInvalidResponsesAndTimeout(t *testing.T) {
	htmlFixture, err := os.ReadFile(filepath.Join("testdata", "auth", "html.txt"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, contentType, body string
		status                  int
		maxBody                 int64
		wantCode                string
	}{
		{name: "forbidden", status: http.StatusForbidden, contentType: "application/json", body: `{}`, wantCode: CodeInvalidCredentials},
		{name: "html fixture", status: http.StatusOK, contentType: "text/html", body: string(htmlFixture), wantCode: CodeAuthenticationUnavailable},
		{name: "malformed JSON", status: http.StatusOK, contentType: "application/json", body: `{`, wantCode: CodeAuthenticationUnavailable},
		{name: "trailing JSON", status: http.StatusOK, contentType: "application/json", body: `{"token":"test-token-value-that-is-long-enough"}{}`, wantCode: CodeAuthenticationUnavailable},
		{name: "short token", status: http.StatusOK, contentType: "application/json", body: `{"token":"short"}`, wantCode: CodeAuthenticationUnavailable},
		{name: "oversized", status: http.StatusOK, contentType: "application/json", body: `{"token":"` + strings.Repeat("a", 100) + `"}`, maxBody: 32, wantCode: CodeAuthenticationUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			client := NewAuthClient(server.URL)
			if test.maxBody > 0 {
				client.MaxBody = test.maxBody
			}
			_, err := client.Authenticate(context.Background(), "p123", []byte("password"))
			if CodeOf(err) != test.wantCode {
				t.Fatalf("got %s, want %s: %v", CodeOf(err), test.wantCode, err)
			}
		})
	}

	timeoutServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"test-token-value-that-is-long-enough"}`))
	}))
	defer timeoutServer.Close()
	client := NewAuthClient(timeoutServer.URL)
	client.HTTPClient.Timeout = 25 * time.Millisecond
	_, err = client.Authenticate(context.Background(), "p123", []byte("password"))
	if CodeOf(err) != CodeTimeout {
		t.Fatalf("timeout returned %s, want %s: %v", CodeOf(err), CodeTimeout, err)
	}
}

func TestAuthClientDoesNotFollowRedirectWithSecrets(t *testing.T) {
	var destinationHits atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		destinationHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	client := NewAuthClient(origin.URL)
	_, _ = client.Authenticate(context.Background(), "p123", []byte("password"))
	if destinationHits.Load() != 0 {
		t.Fatal("authentication request followed a redirect and exposed credentials")
	}
}

func TestAuthClientRejectsControlCharactersBeforeNetwork(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := NewAuthClient(server.URL)
	_, err := client.Authenticate(context.Background(), "p123\r\nInjected", []byte("password"))
	if CodeOf(err) != CodeInvalidCredentials || hits.Load() != 0 {
		t.Fatalf("invalid credentials reached the network: code=%s hits=%d err=%v", CodeOf(err), hits.Load(), err)
	}
}

func TestAuthErrorsOmitPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client := NewAuthClient(server.URL)
	password := "TEST-PIA-PASSWORD-MUST-NOT-LEAK"
	_, err := client.Authenticate(context.Background(), "p123", []byte(password))
	if err == nil {
		t.Fatal("expected error")
	}
	if containsSecret(err.Error(), password) {
		t.Fatalf("password leaked in error: %v", err)
	}
}
