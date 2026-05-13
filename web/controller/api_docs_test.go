package controller

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type routeDef struct {
	Method string
	Path   string
}

// routePattern matches route registrations like g.GET("/path", handler) or api.GET("/path", handler)
var routePattern = regexp.MustCompile(`\b(g|api)\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\("([^"]+)"`)

// docRoutePattern matches { method: 'X', path: 'Y' ... } entries in endpoints.js.
var docRoutePattern = regexp.MustCompile(`method:\s*'([A-Z]+)'\s*,\s*path:\s*'([^']+)'`)

// buildDocSet parses frontend/src/pages/api-docs/endpoints.js and returns the
// set of documented "METHOD PATH" keys. WS pseudo-routes and subscription
// placeholders (paths starting with /{...}) are skipped because they aren't
// registered on the main Gin engine.
func buildDocSet(t *testing.T) map[string]bool {
	t.Helper()
	controllerDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	endpointsPath := filepath.Join(controllerDir, "..", "..", "frontend", "src", "pages", "api-docs", "endpoints.js")
	data, err := os.ReadFile(endpointsPath)
	if err != nil {
		t.Fatalf("failed to read endpoints.js at %s: %v", endpointsPath, err)
	}
	docSet := make(map[string]bool)
	for _, m := range docRoutePattern.FindAllStringSubmatch(string(data), -1) {
		method, path := m[1], m[2]
		if method == "WS" {
			continue
		}
		if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "/{") {
			continue
		}
		docSet[method+" "+path] = true
	}
	if len(docSet) == 0 {
		t.Fatalf("no documented routes parsed from %s — regex or file format may have changed", endpointsPath)
	}
	return docSet
}

func TestAPIRoutesDocumented(t *testing.T) {
	docSet := buildDocSet(t)

	controllerDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}

	var allRoutes []routeDef

	entries, err := os.ReadDir(controllerDir)
	if err != nil {
		t.Fatalf("failed to read controller dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(controllerDir, entry.Name()))
		if err != nil {
			t.Fatalf("failed to read %s: %v", entry.Name(), err)
		}
		src := string(data)

		// Determine the base path for this file based on its initRouter patterns
		basePath := ""
		switch entry.Name() {
		case "index.go":
			basePath = ""
		case "xui.go":
			basePath = "/panel"
		case "api.go":
			basePath = "/panel/api"
		case "inbound.go":
			basePath = "/panel/api/inbounds"
		case "server.go":
			basePath = "/panel/api/server"
		case "node.go":
			basePath = "/panel/api/nodes"
		case "setting.go":
			basePath = "/panel/setting"
		case "xray_setting.go":
			basePath = "/panel/xray"
		case "custom_geo.go":
			basePath = "/panel/api/custom-geo"
		case "websocket.go":
			basePath = ""
		}

		// Find all route registrations
		matches := routePattern.FindAllStringSubmatch(src, -1)
		for _, m := range matches {
			method := m[2]
			path := strings.TrimSpace(m[3])
			if basePath == "" {
				allRoutes = append(allRoutes, routeDef{Method: method, Path: path})
			} else {
				fullPath := basePath + path
				allRoutes = append(allRoutes, routeDef{Method: method, Path: fullPath})
			}
		}
	}

	// The WebSocket route /ws is registered in web/web.go (not a controller file)
	allRoutes = append(allRoutes, routeDef{Method: "GET", Path: "/ws"})

	missingFromDocs := 0
	foundInDoc := 0
	sourceSet := make(map[string]bool)

	for _, r := range allRoutes {
		key := r.Method + " " + r.Path
		// Skip SPA page routes (these are UI pages, not API endpoints)
		spaPages := map[string]bool{
			"/": true, "/panel/": true, "/panel/inbounds": true,
			"/panel/nodes": true, "/panel/settings": true,
			"/panel/xray": true, "/panel/api-docs": true,
		}
		if spaPages[r.Path] {
			continue
		}
		// Skip /panel/csrf-token (documented under auth as /csrf-token)
		if r.Path == "/panel/csrf-token" {
			continue
		}
		// Skip Chrome DevTools route
		if strings.Contains(r.Path, ".well-known") {
			continue
		}

		sourceSet[key] = true
		if docSet[key] {
			foundInDoc++
		} else {
			missingFromDocs++
			t.Errorf("Route not documented in endpoints.js: %s %s", r.Method, r.Path)
		}
	}

	t.Logf("Routes found in source: %d, documented: %d, matching: %d, missing: %d",
		len(sourceSet), len(docSet), foundInDoc, missingFromDocs)

	if missingFromDocs > 0 {
		t.Errorf("Found %d undocumented route(s). Update endpoints.js to match.", missingFromDocs)
	}
}
