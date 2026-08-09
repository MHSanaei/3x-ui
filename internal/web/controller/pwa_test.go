package controller

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestServePWAAssets(t *testing.T) {
	oldDistFS := distFS
	distFS = fstest.MapFS{
		"dist/manifest.webmanifest": &fstest.MapFile{Data: []byte(`{"name":"3x-ui"}`)},
		"dist/pwa-register.js":      &fstest.MapFile{Data: []byte("register")},
		"dist/service-worker.js":    &fstest.MapFile{Data: []byte("worker")},
		"dist/icons/3x-ui-192.svg":  &fstest.MapFile{Data: []byte("icon-192")},
		"dist/icons/3x-ui-512.svg":  &fstest.MapFile{Data: []byte("icon-512")},
	}
	t.Cleanup(func() { distFS = oldDistFS })

	tests := []struct {
		name        string
		handler     gin.HandlerFunc
		contentType string
		body        string
	}{
		{name: "manifest", handler: ServePWAManifest, contentType: "application/manifest+json; charset=utf-8", body: `{"name":"3x-ui"}`},
		{name: "registration", handler: ServePWARegister, contentType: "application/javascript; charset=utf-8", body: "register"},
		{name: "worker", handler: ServePWAServiceWorker, contentType: "application/javascript; charset=utf-8", body: "worker"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			test.handler(context)

			if response.Code != 200 {
				t.Fatalf("status = %d, want 200", response.Code)
			}
			if response.Header().Get("Content-Type") != test.contentType {
				t.Errorf("content type = %q, want %q", response.Header().Get("Content-Type"), test.contentType)
			}
			if response.Header().Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
				t.Errorf("cache control = %q", response.Header().Get("Cache-Control"))
			}
			if strings.TrimSpace(response.Body.String()) != test.body {
				t.Errorf("body = %q, want %q", response.Body.String(), test.body)
			}
		})
	}
}

func TestServePWAIconRejectsUnknownName(t *testing.T) {
	oldDistFS := distFS
	distFS = fstest.MapFS{}
	t.Cleanup(func() { distFS = oldDistFS })

	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Params = gin.Params{{Key: "name", Value: "../../etc/passwd"}}
	ServePWAIcon(context)

	if response.Code != 404 {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}
