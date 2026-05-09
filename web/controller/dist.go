package controller

import (
	"bytes"
	"embed"
	htmlpkg "html"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/session"
)

// distFS is filled in once at startup by the web package via SetDistFS.
// It holds the Vite-built frontend (the `dist/<page>.html` files) so
// the panel's HTML routes can serve them in production.
//
// We can't `go:embed` the dist directory directly from this package
// because embed.FS only accepts paths relative to the source file —
// dist/ lives one directory up. The web package owns the embed and
// hands the FS to us through this setter.
var distFS embed.FS

// SetDistFS is called once during server bootstrap by the web package
// to hand off the embedded `dist/` filesystem.
func SetDistFS(fs embed.FS) {
	distFS = fs
}

// distPageBuildTime is captured at startup so every served HTML page
// reports a stable Last-Modified header and the browser's conditional
// GETs can hit the 304 path on repeat loads.
var distPageBuildTime = time.Now()

// serveDistPage reads `dist/<name>` from the embedded FS and writes it
// to the response. Two transforms run before send:
//
//  1. `<script>window.__X_UI_BASE_PATH__ = "..."</script>` is injected
//     just before </head> so the AppSidebar's link generator sees the
//     right prefix.
//  2. Absolute Vite-emitted asset URLs (`/assets/...`) are rewritten
//     to include the panel's basePath, so installs running under a
//     custom URL prefix (e.g. `/myprefix/`) load the bundle from
//     `/myprefix/assets/...` where the static handler actually lives.
//
// The HTML responses are served with no-cache so a panel update
// reaches users on the next reload; the long-hashed JS/CSS files
// under /assets/ stay cacheable indefinitely.
func serveDistPage(c *gin.Context, name string) {
	body, err := distFS.ReadFile("dist/" + name)
	if err != nil {
		c.String(http.StatusInternalServerError, "missing embedded page: %s", name)
		return
	}

	basePath := c.GetString("base_path")
	if basePath == "" {
		basePath = "/"
	}

	// Rewrite asset URLs only when basePath isn't the root — for the
	// default `/` install, Vite's `/assets/...` already resolves
	// correctly and we save the byte churn.
	if basePath != "/" {
		// Vite emits these three attribute shapes for every entry's
		// JS / CSS / modulepreload reference. Anchoring the search to
		// the leading attribute name avoids matching unrelated /assets
		// substrings inside any inlined script.
		body = bytes.ReplaceAll(body, []byte(`src="/assets/`), []byte(`src="`+basePath+`assets/`))
		body = bytes.ReplaceAll(body, []byte(`href="/assets/`), []byte(`href="`+basePath+`assets/`))
	}

	// Escape just enough that a hostile basePath setting can't break
	// out of the JS string literal. The setting is admin-controlled
	// but defense-in-depth costs nothing here.
	jsEscape := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"<", `<`,
		">", `>`,
		"&", `&`,
	)
	escapedBase := jsEscape.Replace(basePath)
	escapedVer := jsEscape.Replace(config.GetVersion())

	// Embed a CSRF token in the served HTML the same way the legacy
	// templates did via `<meta name="csrf-token">`. Without this the
	// SPA login page has no way to acquire a token (the existing
	// /panel/csrf-token endpoint sits behind checkLogin), and POST
	// /login is rejected by CSRFMiddleware. EnsureCSRFToken creates
	// a session token on first call even for anonymous visitors.
	csrfToken, err := session.EnsureCSRFToken(c)
	if err != nil {
		logger.Warning("Unable to mint CSRF token for", name+":", err)
		csrfToken = ""
	}
	csrfMeta := []byte(`<meta name="csrf-token" content="` + htmlpkg.EscapeString(csrfToken) + `">`)

	inject := []byte(`<script>window.__X_UI_BASE_PATH__="` + escapedBase +
		`";window.__X_UI_CUR_VER__="` + escapedVer + `";</script>`)
	inject = append(inject, csrfMeta...)
	inject = append(inject, []byte(`</head>`)...)
	out := bytes.Replace(body, []byte("</head>"), inject, 1)

	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Last-Modified", distPageBuildTime.UTC().Format(http.TimeFormat))
	c.Data(http.StatusOK, "text/html; charset=utf-8", out)
}
