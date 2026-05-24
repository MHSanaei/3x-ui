package controller

import (
	"bytes"
	"embed"
	htmlpkg "html"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/config"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/session"
)

var distFS embed.FS

func SetDistFS(fs embed.FS) {
	distFS = fs
}

var distPageBuildTime = time.Now()

// ServeOpenAPISpec returns the generated OpenAPI 3.0 description of the
// panel API. Postman / Insomnia / openapi-generator consume this URL
// directly; the in-panel Swagger UI page also fetches it. The spec is
// produced at frontend build time by scripts/build-openapi.mjs and
// embedded into the binary via the dist FS.
func ServeOpenAPISpec(c *gin.Context) {
	body, err := distFS.ReadFile("dist/openapi.json")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "msg": "openapi.json not found"})
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

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

	if basePath != "/" {
		body = bytes.ReplaceAll(body, []byte(`src="/assets/`), []byte(`src="`+basePath+`assets/`))
		body = bytes.ReplaceAll(body, []byte(`href="/assets/`), []byte(`href="`+basePath+`assets/`))
	}

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
	csrfToken, err := session.EnsureCSRFToken(c)
	if err != nil {
		logger.Warning("Unable to mint CSRF token for", name+":", err)
		csrfToken = ""
	}
	csrfMeta := []byte(`<meta name="csrf-token" content="` + htmlpkg.EscapeString(csrfToken) + `">`)
	basePathMeta := []byte(`<meta name="base-path" content="` + htmlpkg.EscapeString(basePath) + `">`)

	nonceAttr := ""
	if nonce := c.GetString("csp_nonce"); nonce != "" {
		nonceAttr = ` nonce="` + htmlpkg.EscapeString(nonce) + `"`
	}
	script := `<script` + nonceAttr + `>window.X_UI_BASE_PATH="` + escapedBase + `"`
	if name != "login.html" {
		escapedVer := jsEscape.Replace(config.GetVersion())
		script += `;window.X_UI_CUR_VER="` + escapedVer + `"`
	}
	script += `;</script>`
	inject := []byte(script)
	inject = append(inject, csrfMeta...)
	inject = append(inject, basePathMeta...)
	inject = append(inject, []byte(`</head>`)...)
	out := bytes.Replace(body, []byte("</head>"), inject, 1)

	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("Last-Modified", distPageBuildTime.UTC().Format(http.TimeFormat))
	c.Data(http.StatusOK, "text/html; charset=utf-8", out)
}
