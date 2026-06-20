package sub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// writeSubError translates a service-layer result into an HTTP response.
// A nil error with no rows means the subId doesn't match anything (deleted
// client, never-existed id) and becomes 404. A real error becomes 500. No
// body — VPN clients only look at the status.
func writeSubError(c *gin.Context, err error) {
	if err == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Status(http.StatusInternalServerError)
}

// cachedSubTemplate holds a parsed custom subscription template together with
// the modification time of the file it was parsed from, so the cache can be
// invalidated when an admin edits the template on disk.
type cachedSubTemplate struct {
	tmpl    *template.Template
	modTime time.Time
}

// SUBController handles HTTP requests for subscription links and JSON configurations.
type SUBController struct {
	subTitle         string
	subSupportUrl    string
	subProfileUrl    string
	subAnnounce      string
	subEnableRouting bool
	subRoutingRules  string
	subHideSettings  bool
	subPath          string
	subJsonPath      string
	subClashPath     string
	jsonEnabled      bool
	clashEnabled     bool
	subEncrypt       bool
	updateInterval   string

	subService      *SubService
	subJsonService  *SubJsonService
	subClashService *SubClashService
	settingService  service.SettingService

	subTemplateMu    sync.RWMutex
	subTemplateCache map[string]*cachedSubTemplate
}

// NewSUBController creates a new subscription controller with the given configuration.
func NewSUBController(
	g *gin.RouterGroup,
	subPath string,
	jsonPath string,
	clashPath string,
	jsonEnabled bool,
	clashEnabled bool,
	encrypt bool,
	remarkTemplate string,
	update string,
	jsonMux string,
	jsonRules string,
	jsonFinalMask string,
	clashEnableRouting bool,
	clashRules string,
	subTitle string,
	subSupportUrl string,
	subProfileUrl string,
	subAnnounce string,
	subEnableRouting bool,
	subRoutingRules string,
	subHideSettings bool,
) *SUBController {
	sub := NewSubService(remarkTemplate)
	a := &SUBController{
		subTitle:         subTitle,
		subSupportUrl:    subSupportUrl,
		subProfileUrl:    subProfileUrl,
		subAnnounce:      subAnnounce,
		subEnableRouting: subEnableRouting,
		subRoutingRules:  subRoutingRules,
		subHideSettings:  subHideSettings,
		subPath:          subPath,
		subJsonPath:      jsonPath,
		subClashPath:     clashPath,
		jsonEnabled:      jsonEnabled,
		clashEnabled:     clashEnabled,
		subEncrypt:       encrypt,
		updateInterval:   update,

		subService:      sub,
		subJsonService:  NewSubJsonService(jsonMux, jsonRules, jsonFinalMask, sub),
		subClashService: NewSubClashService(clashEnableRouting, clashRules, sub),

		subTemplateCache: map[string]*cachedSubTemplate{},
	}
	a.initRouter(g)
	return a
}

// initRouter registers HTTP routes for subscription links and JSON endpoints
// on the provided router group.
func (a *SUBController) initRouter(g *gin.RouterGroup) {
	gLink := g.Group(a.subPath)
	gLink.GET(":subid", a.subs)
	gLink.HEAD(":subid", a.subs)
	if a.jsonEnabled {
		gJson := g.Group(a.subJsonPath)
		gJson.GET(":subid", a.subJsons)
		gJson.HEAD(":subid", a.subJsons)
	}
	if a.clashEnabled {
		gClash := g.Group(a.subClashPath)
		gClash.GET(":subid", a.subClashs)
		gClash.HEAD(":subid", a.subClashs)
	}
}

// subs handles HTTP requests for subscription links, returning either HTML page or base64-encoded subscription data.
func (a *SUBController) subs(c *gin.Context) {
	subId := c.Param("subid")
	scheme, host, hostWithPort, hostHeader := a.subService.ResolveRequest(c)
	subReq := a.subService.ForRequest(host)
	// The remark template's per-client info is for the content a client app
	// imports — the raw subscription body. A browser viewing the HTML info page
	// gets clean, name-only remarks (usage is shown in the page summary).
	accept := c.GetHeader("Accept")
	wantsHTML := strings.Contains(strings.ToLower(accept), "text/html") || c.Query("html") == "1" || strings.EqualFold(c.Query("view"), "html")
	subReq.subscriptionBody = !wantsHTML
	subs, emails, lastOnline, traffic, err := subReq.getSubs(subId)
	if err != nil || len(subs) == 0 {
		writeSubError(c, err)
	} else {
		var result strings.Builder
		for _, sub := range subs {
			result.WriteString(sub)
			result.WriteString("\n")
		}

		// If the request expects HTML (e.g., browser) or explicitly asked (?html=1 or ?view=html), render the info page here
		if wantsHTML {
			subURL, subJsonURL, subClashURL := subReq.BuildURLs(a.subPath, a.subJsonPath, a.subClashPath, subId)
			if !a.jsonEnabled {
				subJsonURL = ""
			}
			if !a.clashEnabled {
				subClashURL = ""
			}
			basePath, exists := c.Get("base_path")
			if !exists {
				basePath = "/"
			}
			basePathStr := basePath.(string)
			page := subReq.BuildPageData(subId, hostHeader, traffic, lastOnline, subs, emails, subURL, subJsonURL, subClashURL, basePathStr, a.subTitle, a.subSupportUrl)
			a.serveSubPage(c, basePathStr, page)
			return
		}

		// Add headers
		header := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
		profileUrl := a.subProfileUrl
		if profileUrl == "" {
			profileUrl = fmt.Sprintf("%s://%s%s", scheme, hostWithPort, c.Request.RequestURI)
		}
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle, a.subSupportUrl, profileUrl, a.subAnnounce, a.subEnableRouting, a.subRoutingRules, a.subHideSettings)

		if a.subEncrypt {
			c.String(200, base64.StdEncoding.EncodeToString([]byte(result.String())))
		} else {
			c.String(200, result.String())
		}
	}
}

// serveSubPage renders internal/web/dist/subpage.html for the current subscription
// request. The Vite-built SPA reads window.__SUB_PAGE_DATA__ on mount —
// we inject that here, along with window.X_UI_BASE_PATH so the
// page's static asset references resolve correctly when the panel runs
// behind a URL prefix.
func (a *SUBController) serveSubPage(c *gin.Context, basePath string, page PageData) {
	var body []byte
	if diskBody, diskErr := os.ReadFile("internal/web/dist/subpage.html"); diskErr == nil {
		body = diskBody
	} else {
		readBody, err := distFS.ReadFile("dist/subpage.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "missing embedded subpage")
			return
		}
		body = readBody
	}

	// Vite emits absolute asset URLs (`/assets/...`); when the panel is
	// installed under a custom URL prefix, rewrite them so the bundle
	// loads from `<basePath>assets/...` where the static handler is
	// actually mounted.
	if basePath != "/" && basePath != "" {
		body = bytes.ReplaceAll(body, []byte(`src="/assets/`), []byte(`src="`+basePath+`assets/`))
		body = bytes.ReplaceAll(body, []byte(`href="/assets/`), []byte(`href="`+basePath+`assets/`))
	}

	// JSON-marshal the view-model so the SPA can read it as a plain
	// The panel's "Calendar Type" setting decides whether the SubPage
	// renders dates in Gregorian or Jalali — surface it here so the SPA
	// can match the rest of the panel without a round-trip.
	datepicker, _ := a.settingService.GetDatepicker()
	if datepicker == "" {
		datepicker = "gregorian"
	}

	subData := map[string]any{
		"sId":           page.SId,
		"enabled":       page.Enabled,
		"download":      page.Download,
		"upload":        page.Upload,
		"total":         page.Total,
		"used":          page.Used,
		"remained":      page.Remained,
		"expire":        page.Expire,
		"lastOnline":    page.LastOnline,
		"downloadByte":  page.DownloadByte,
		"uploadByte":    page.UploadByte,
		"totalByte":     page.TotalByte,
		"subUrl":        page.SubUrl,
		"subJsonUrl":    page.SubJsonUrl,
		"subClashUrl":   page.SubClashUrl,
		"subTitle":      page.SubTitle,
		"subSupportUrl": page.SubSupportUrl,
		"links":         page.Result,
		"emails":        page.Emails,
		"datepicker":    datepicker,
	}

	// When an admin has configured a custom subscription theme, render it
	// instead of the default SPA. We render into a buffer first so a template
	// that fails mid-execution can't leave a partially-written (corrupt)
	// response — on any error we log and fall through to the default page.
	if themeDir, _ := a.settingService.GetSubThemeDir(); themeDir != "" {
		if tmpl, err := a.loadSubTemplate(themeDir); err != nil {
			logger.Error("sub: custom template parse failed, using default page:", err)
		} else if tmpl == nil {
			logger.Warning("sub: subThemeDir set but no usable template found, using default page:", themeDir)
		} else {
			var buf bytes.Buffer
			if execErr := tmpl.Execute(&buf, subData); execErr != nil {
				logger.Error("sub: custom template execution failed, using default page:", execErr)
			} else {
				setNoCacheHeaders(c)
				c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
				return
			}
		}
	}

	subDataJSON, err := json.Marshal(subData)
	if err != nil {
		subDataJSON = []byte("{}")
	}

	// Defense-in-depth string-escape for the basePath embed — admin-
	// controlled but cheap to harden.
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

	inject := []byte(`<script>window.X_UI_BASE_PATH="` + escapedBase + `";` +
		`window.__SUB_PAGE_DATA__=` + string(subDataJSON) + `;</script></head>`)
	out := bytes.Replace(body, []byte("</head>"), inject, 1)

	setNoCacheHeaders(c)
	c.Data(http.StatusOK, "text/html; charset=utf-8", out)
}

// setNoCacheHeaders marks a subscription page response as non-cacheable so VPN
// clients and browsers always fetch fresh traffic/expiry data.
func setNoCacheHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

// loadSubTemplate returns the parsed custom subscription template located in
// themeDir, preferring sub.html over index.html. Parsed templates are cached and
// only re-parsed when the underlying file's modification time changes, so admin
// edits are picked up without paying a disk read + HTML parse on every request.
//
// It returns (nil, nil) when themeDir is not a usable directory or contains no
// template file — the caller should fall back to the default page. A non-nil
// error means a template file exists but failed to parse.
func (a *SUBController) loadSubTemplate(themeDir string) (*template.Template, error) {
	info, err := os.Stat(themeDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	templatePath := filepath.Join(themeDir, "index.html")
	if _, err := os.Stat(filepath.Join(themeDir, "sub.html")); err == nil {
		templatePath = filepath.Join(themeDir, "sub.html")
	}

	fi, err := os.Stat(templatePath)
	if err != nil {
		return nil, nil
	}
	modTime := fi.ModTime()

	a.subTemplateMu.RLock()
	cached := a.subTemplateCache[templatePath]
	a.subTemplateMu.RUnlock()
	if cached != nil && cached.modTime.Equal(modTime) {
		return cached.tmpl, nil
	}

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, err
	}

	a.subTemplateMu.Lock()
	a.subTemplateCache[templatePath] = &cachedSubTemplate{tmpl: tmpl, modTime: modTime}
	a.subTemplateMu.Unlock()
	return tmpl, nil
}

// subJsons handles HTTP requests for JSON subscription configurations.
func (a *SUBController) subJsons(c *gin.Context) {
	subId := c.Param("subid")
	scheme, host, hostWithPort, _ := a.subService.ResolveRequest(c)
	jsonSub, header, err := a.subJsonService.GetJson(subId, host)
	if err != nil || len(jsonSub) == 0 {
		writeSubError(c, err)
	} else {
		profileUrl := a.subProfileUrl
		if profileUrl == "" {
			profileUrl = fmt.Sprintf("%s://%s%s", scheme, hostWithPort, c.Request.RequestURI)
		}
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle, a.subSupportUrl, profileUrl, a.subAnnounce, a.subEnableRouting, a.subRoutingRules, a.subHideSettings)

		c.String(200, jsonSub)
	}
}

func (a *SUBController) subClashs(c *gin.Context) {
	subId := c.Param("subid")
	scheme, host, hostWithPort, _ := a.subService.ResolveRequest(c)
	clashSub, header, err := a.subClashService.GetClash(subId, host)
	if err != nil || len(clashSub) == 0 {
		writeSubError(c, err)
	} else {
		profileUrl := a.subProfileUrl
		if profileUrl == "" {
			profileUrl = fmt.Sprintf("%s://%s%s", scheme, hostWithPort, c.Request.RequestURI)
		}
		a.ApplyCommonHeaders(c, header, a.updateInterval, a.subTitle, a.subSupportUrl, profileUrl, a.subAnnounce, a.subEnableRouting, a.subRoutingRules, a.subHideSettings)
		if a.subTitle != "" {
			// Clash clients commonly use Content-Disposition to choose the imported profile name.
			c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, url.PathEscape(a.subTitle)))
		}
		c.Data(200, "application/yaml; charset=utf-8", []byte(clashSub))
	}
}

// ApplyCommonHeaders sets common HTTP headers for subscription responses including user info, update interval, and profile title.
func (a *SUBController) ApplyCommonHeaders(
	c *gin.Context,
	header,
	updateInterval,
	profileTitle string,
	profileSupportUrl string,
	profileUrl string,
	profileAnnounce string,
	profileEnableRouting bool,
	profileRoutingRules string,
	profileHideSettings bool,
) {
	c.Writer.Header().Set("Subscription-Userinfo", header)
	c.Writer.Header().Set("Profile-Update-Interval", updateInterval)

	//Basics
	if profileTitle != "" {
		c.Writer.Header().Set("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(profileTitle)))
	}
	if profileSupportUrl != "" {
		c.Writer.Header().Set("Support-Url", profileSupportUrl)
	}
	if profileUrl != "" {
		c.Writer.Header().Set("Profile-Web-Page-Url", profileUrl)
	}
	if profileAnnounce != "" {
		c.Writer.Header().Set("Announce", "base64:"+base64.StdEncoding.EncodeToString([]byte(profileAnnounce)))
	}

	//Advanced (Happ)
	c.Writer.Header().Set("Routing-Enable", strconv.FormatBool(profileEnableRouting))
	if profileRoutingRules != "" {
		c.Writer.Header().Set("Routing", profileRoutingRules)
	}
	if profileHideSettings {
		c.Writer.Header().Set("Hide-Settings", "1")
	}
}
