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

// expectedRoutes lists every route documented in frontend/src/pages/api-docs/endpoints.js.
// Keep this in sync when adding new endpoints to the docs.
var expectedRoutes = []routeDef{
	// Authentication — no prefix
	{Method: "POST", Path: "/login"},
	{Method: "GET", Path: "/logout"},
	{Method: "GET", Path: "/csrf-token"},
	{Method: "POST", Path: "/getTwoFactorEnable"},

	// Inbounds API — prefix /panel/api/inbounds
	{Method: "GET", Path: "/panel/api/inbounds/list"},
	{Method: "GET", Path: "/panel/api/inbounds/get/:id"},
	{Method: "GET", Path: "/panel/api/inbounds/getClientTraffics/:email"},
	{Method: "GET", Path: "/panel/api/inbounds/getClientTrafficsById/:id"},
	{Method: "GET", Path: "/panel/api/inbounds/getSubLinks/:subId"},
	{Method: "GET", Path: "/panel/api/inbounds/getClientLinks/:id/:email"},
	{Method: "POST", Path: "/panel/api/inbounds/add"},
	{Method: "POST", Path: "/panel/api/inbounds/del/:id"},
	{Method: "POST", Path: "/panel/api/inbounds/update/:id"},
	{Method: "POST", Path: "/panel/api/inbounds/setEnable/:id"},
	{Method: "POST", Path: "/panel/api/inbounds/clientIps/:email"},
	{Method: "POST", Path: "/panel/api/inbounds/clearClientIps/:email"},
	{Method: "POST", Path: "/panel/api/inbounds/addClient"},
	{Method: "POST", Path: "/panel/api/inbounds/:id/copyClients"},
	{Method: "POST", Path: "/panel/api/inbounds/:id/delClient/:clientId"},
	{Method: "POST", Path: "/panel/api/inbounds/updateClient/:clientId"},
	{Method: "POST", Path: "/panel/api/inbounds/:id/resetClientTraffic/:email"},
	{Method: "POST", Path: "/panel/api/inbounds/resetAllTraffics"},
	{Method: "POST", Path: "/panel/api/inbounds/resetAllClientTraffics/:id"},
	{Method: "POST", Path: "/panel/api/inbounds/delDepletedClients/:id"},
	{Method: "POST", Path: "/panel/api/inbounds/import"},
	{Method: "POST", Path: "/panel/api/inbounds/onlines"},
	{Method: "POST", Path: "/panel/api/inbounds/lastOnline"},
	{Method: "POST", Path: "/panel/api/inbounds/updateClientTraffic/:email"},
	{Method: "POST", Path: "/panel/api/inbounds/:id/delClientByEmail/:email"},

	// Server API — prefix /panel/api/server
	{Method: "GET", Path: "/panel/api/server/status"},
	{Method: "GET", Path: "/panel/api/server/cpuHistory/:bucket"},
	{Method: "GET", Path: "/panel/api/server/history/:metric/:bucket"},
	{Method: "GET", Path: "/panel/api/server/xrayMetricsState"},
	{Method: "GET", Path: "/panel/api/server/xrayMetricsHistory/:metric/:bucket"},
	{Method: "GET", Path: "/panel/api/server/xrayObservatory"},
	{Method: "GET", Path: "/panel/api/server/xrayObservatoryHistory/:tag/:bucket"},
	{Method: "GET", Path: "/panel/api/server/getXrayVersion"},
	{Method: "GET", Path: "/panel/api/server/getPanelUpdateInfo"},
	{Method: "GET", Path: "/panel/api/server/getConfigJson"},
	{Method: "GET", Path: "/panel/api/server/getDb"},
	{Method: "GET", Path: "/panel/api/server/getNewUUID"},
	{Method: "GET", Path: "/panel/api/server/getNewX25519Cert"},
	{Method: "GET", Path: "/panel/api/server/getNewmldsa65"},
	{Method: "GET", Path: "/panel/api/server/getNewmlkem768"},
	{Method: "GET", Path: "/panel/api/server/getNewVlessEnc"},
	{Method: "POST", Path: "/panel/api/server/stopXrayService"},
	{Method: "POST", Path: "/panel/api/server/restartXrayService"},
	{Method: "POST", Path: "/panel/api/server/installXray/:version"},
	{Method: "POST", Path: "/panel/api/server/updatePanel"},
	{Method: "POST", Path: "/panel/api/server/updateGeofile"},
	{Method: "POST", Path: "/panel/api/server/updateGeofile/:fileName"},
	{Method: "POST", Path: "/panel/api/server/logs/:count"},
	{Method: "POST", Path: "/panel/api/server/xraylogs/:count"},
	{Method: "POST", Path: "/panel/api/server/importDB"},
	{Method: "POST", Path: "/panel/api/server/getNewEchCert"},

	// Nodes API — prefix /panel/api/nodes
	{Method: "GET", Path: "/panel/api/nodes/list"},
	{Method: "GET", Path: "/panel/api/nodes/get/:id"},
	{Method: "POST", Path: "/panel/api/nodes/add"},
	{Method: "POST", Path: "/panel/api/nodes/update/:id"},
	{Method: "POST", Path: "/panel/api/nodes/del/:id"},
	{Method: "POST", Path: "/panel/api/nodes/setEnable/:id"},
	{Method: "POST", Path: "/panel/api/nodes/test"},
	{Method: "POST", Path: "/panel/api/nodes/probe/:id"},
	{Method: "GET", Path: "/panel/api/nodes/history/:id/:metric/:bucket"},

	// Custom Geo API — prefix /panel/api/custom-geo
	{Method: "GET", Path: "/panel/api/custom-geo/list"},
	{Method: "GET", Path: "/panel/api/custom-geo/aliases"},
	{Method: "POST", Path: "/panel/api/custom-geo/add"},
	{Method: "POST", Path: "/panel/api/custom-geo/update/:id"},
	{Method: "POST", Path: "/panel/api/custom-geo/delete/:id"},
	{Method: "POST", Path: "/panel/api/custom-geo/download/:id"},
	{Method: "POST", Path: "/panel/api/custom-geo/update-all"},

	// Backup — prefix /panel/api
	{Method: "GET", Path: "/panel/api/backuptotgbot"},

	// Settings API — prefix /panel/setting
	{Method: "POST", Path: "/panel/setting/all"},
	{Method: "POST", Path: "/panel/setting/defaultSettings"},
	{Method: "POST", Path: "/panel/setting/update"},
	{Method: "POST", Path: "/panel/setting/updateUser"},
	{Method: "POST", Path: "/panel/setting/restartPanel"},
	{Method: "GET", Path: "/panel/setting/getDefaultJsonConfig"},
	{Method: "GET", Path: "/panel/setting/getApiToken"},
	{Method: "POST", Path: "/panel/setting/regenerateApiToken"},

	// Xray Settings API — prefix /panel/xray
	{Method: "POST", Path: "/panel/xray/"},
	{Method: "GET", Path: "/panel/xray/getDefaultJsonConfig"},
	{Method: "GET", Path: "/panel/xray/getOutboundsTraffic"},
	{Method: "GET", Path: "/panel/xray/getXrayResult"},
	{Method: "POST", Path: "/panel/xray/update"},
	{Method: "POST", Path: "/panel/xray/warp/:action"},
	{Method: "POST", Path: "/panel/xray/nord/:action"},
	{Method: "POST", Path: "/panel/xray/resetOutboundsTraffic"},
	{Method: "POST", Path: "/panel/xray/testOutbound"},

	// WebSocket
	{Method: "GET", Path: "/ws"},

	// Subscription server — separate server (not on main Gin engine)
	// Documented in Subscription Server section but not tested here
	// because the sub server is a separate Gin engine.
	// {Method: "GET", Path: "/sub/:subid"},
	// {Method: "GET", Path: "/json/:subid"},
	// {Method: "GET", Path: "/clash/:subid"},
}

// routePattern matches route registrations like g.GET("/path", handler) or api.GET("/path", handler)
var routePattern = regexp.MustCompile(`\b(g|api)\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\("([^"]+)"`)

func TestAPIRoutesDocumented(t *testing.T) {
	// Build a set of documented routes for fast lookup
	docSet := make(map[string]bool)
	for _, r := range expectedRoutes {
		key := r.Method + " " + r.Path
		if docSet[key] {
			t.Errorf("Duplicate documented route: %s", key)
		}
		docSet[key] = true
	}

	// Walk the web directory to find all Go files with route definitions
	controllerDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}

	// Collect all routes from the controller files
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

	// Check each source route against the documented set
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
		// Skip /panel/csrf-token (documented under auth)
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

	// Report undocumented documented routes
	extraInDocs := 0
	for _, r := range expectedRoutes {
		key := r.Method + " " + r.Path
		if !sourceSet[key] {
			extraInDocs++
			t.Logf("Documented route not found in source (perhaps deleted or moved): %s %s", r.Method, r.Path)
		}
	}

	t.Logf("Routes found in source: %d, documented: %d, matching: %d, missing: %d, extra in docs: %d",
		len(sourceSet), len(docSet), foundInDoc, missingFromDocs, extraInDocs)

	if missingFromDocs > 0 {
		t.Errorf("Found %d undocumented route(s). Update endpoints.js to match.", missingFromDocs)
	}
}
