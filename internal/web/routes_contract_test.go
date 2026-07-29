package web

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/web/global"
)

/*
frontend/src/pages/api-docs/endpoints.ts is a hand-maintained registry: an
API route omitted there silently vanishes from the generated OpenAPI docs,
and an entry for a removed route documents an endpoint that 404s. Nothing
else checks the registry against the Gin router, so this test does: every
/panel/api route the server actually registers must have a matching entry,
and every documented /panel/api entry must correspond to a registered route.
*/

const apiPrefix = "/panel/api/"

func registeredAPIRoutes(t *testing.T) map[string]bool {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	s := NewServer()
	s.cron = cron.New(cron.WithLocation(time.Local), cron.WithSeconds())
	global.SetWebServer(s)
	engine, err := s.initRouter()
	if err != nil {
		t.Fatalf("init router: %v", err)
	}
	routes := make(map[string]bool)
	for _, r := range engine.Routes() {
		if len(r.Path) >= len(apiPrefix) && r.Path[:len(apiPrefix)] == apiPrefix {
			routes[r.Method+" "+r.Path] = true
		}
	}
	if len(routes) == 0 {
		t.Fatal("no /panel/api routes registered; router construction is broken")
	}
	return routes
}

func documentedAPIRoutes(t *testing.T) map[string]bool {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "pages", "api-docs", "endpoints.ts"))
	if err != nil {
		t.Fatalf("read endpoints.ts: %v", err)
	}
	entry := regexp.MustCompile(`(?s)method:\s*'(GET|POST|PUT|DELETE|PATCH)'[^}]*?path:\s*'([^']+)'`)
	docs := make(map[string]bool)
	for _, m := range entry.FindAllStringSubmatch(string(source), -1) {
		if len(m[2]) >= len(apiPrefix) && m[2][:len(apiPrefix)] == apiPrefix {
			docs[m[1]+" "+m[2]] = true
		}
	}
	if len(docs) == 0 {
		t.Fatal("no /panel/api entries parsed from endpoints.ts; the parser regex is broken")
	}
	return docs
}

func TestEveryAPIRouteIsDocumented(t *testing.T) {
	registered := registeredAPIRoutes(t)
	documented := documentedAPIRoutes(t)

	var missing []string
	for route := range registered {
		if !documented[route] {
			missing = append(missing, route)
		}
	}
	sort.Strings(missing)
	for _, route := range missing {
		t.Error(fmt.Errorf("route %s is registered but absent from endpoints.ts — add an entry or it vanishes from the API docs", route))
	}
}

func TestEveryDocumentedRouteIsRegistered(t *testing.T) {
	registered := registeredAPIRoutes(t)
	documented := documentedAPIRoutes(t)

	var stale []string
	for route := range documented {
		if !registered[route] {
			stale = append(stale, route)
		}
	}
	sort.Strings(stale)
	for _, route := range stale {
		t.Error(fmt.Errorf("endpoints.ts documents %s but the server does not register it — remove or fix the entry", route))
	}
}
