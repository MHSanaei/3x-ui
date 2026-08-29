package frontproxy

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// logDecoyFallback records why a configured decoy could not be used. The
// reverse proxy still serves a template, so this is a warning, never fatal.
func logDecoyFallback(mode string, err error) {
	logger.Warningf("frontproxy: %s decoy unusable, falling back to template: %v", mode, err)
}

//go:embed templates/*.html
var decoyTemplates embed.FS

// DecoyMode selects what the reverse proxy shows for every path that is not one
// of the secret ones.
type DecoyMode string

const (
	// DecoyTemplate serves one of the pages embedded in this package.
	DecoyTemplate DecoyMode = "template"
	// DecoyUpload serves a static site the admin uploaded.
	DecoyUpload DecoyMode = "upload"
	// DecoyProxy reverse-proxies to a site hosted elsewhere.
	DecoyProxy DecoyMode = "proxy"
)

// DefaultDecoyTemplate is used when no template was chosen, and as the
// fallback whenever a configured decoy turns out to be unusable.
const DefaultDecoyTemplate = "maintenance"

// DecoyConfig describes the decoy half of the reverse proxy.
type DecoyConfig struct {
	Mode     DecoyMode
	Template string
	Dir      string
	ProxyURL string
	// Seed makes this install's templates render unlike anyone else's. See
	// Variant for why identical bytes across installs would be a giveaway.
	Seed string
}

// newDecoyHandler builds the handler for non-secret paths. It never returns
// an error: a broken decoy must still show something rather than leak a 500.
func newDecoyHandler(cfg DecoyConfig) http.Handler {
	switch cfg.Mode {
	case DecoyUpload:
		if h, err := newUploadDecoy(cfg.Dir); err == nil {
			return h
		} else {
			logDecoyFallback("upload", err)
		}
	case DecoyProxy:
		if h, err := newProxyDecoy(cfg.ProxyURL); err == nil {
			return h
		} else {
			logDecoyFallback("proxy", err)
		}
	}
	return withLoginMock(cfg.Template, newTemplateDecoy(cfg.Template, cfg.Seed))
}

// newTemplateDecoy serves one embedded page for every path, so a prober sees
// a consistent site instead of a directory of guessable files.
func newTemplateDecoy(name, seed string) http.Handler {
	body, err := renderDecoyTemplate(name, seed)
	if err != nil {
		logDecoyFallback("template "+name, err)
		body, _ = renderDecoyTemplate(DefaultDecoyTemplate, seed)
	}
	// The body is fixed for this install, so the validator can be computed
	// once. Without one a browser has nothing to revalidate against and
	// caches the page heuristically -- which is how a decoy keeps showing
	// the previous theme long after the admin picked a new one.
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256(body))
	length := strconv.Itoa(len(body))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache")
		if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Length", length)
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(body)
	})
}

// renderDecoyTemplate loads an embedded page by name and renders it for this
// install, rejecting any name that could reach outside the template set.
func renderDecoyTemplate(name, seed string) ([]byte, error) {
	if name == "" {
		name = DefaultDecoyTemplate
	}
	if strings.ContainsAny(name, `/\.`) {
		return nil, fmt.Errorf("invalid decoy template name %q", name)
	}
	raw, err := decoyTemplates.ReadFile("templates/" + name + ".html")
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parsing decoy template %q: %w", name, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, NewVariant(seed, name)); err != nil {
		return nil, fmt.Errorf("rendering decoy template %q: %w", name, err)
	}
	return out.Bytes(), nil
}

// DecoyTemplateNames lists the embedded pages, for the settings UI to offer.
func DecoyTemplateNames() []string {
	entries, err := decoyTemplates.ReadDir("templates")
	if err != nil {
		return []string{DefaultDecoyTemplate}
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".html"))
	}
	return names
}

// decoyOriginToken is a placeholder an uploaded static site's own files can
// use in place of hardcoding their own domain -- substituted with this
// request's real scheme+host at serve time. Without it, an absolute URL a
// site generator bakes into sitemap.xml/rss.xml/robots.txt at upload time
// goes silently stale the moment the reverse proxy's domain changes, and has
// to be re-uploaded just to fix a URL.
const decoyOriginToken = "{{DECOY_ORIGIN}}"

// decoySubstitutedExt lists the extensions read into memory for
// {{DECOY_ORIGIN}} substitution -- the small text formats a static site
// generator actually emits absolute URLs into. Anything else (images, fonts,
// ...) is served as-is by http.FileServer, unmodified and with its normal
// caching headers.
var decoySubstitutedExt = map[string]bool{
	".html": true, ".htm": true, ".css": true, ".js": true,
	".xml": true, ".txt": true, ".json": true,
}

// newUploadDecoy serves the admin's uploaded site. An empty or missing
// directory is an error so the caller falls back to a template.
func newUploadDecoy(dir string) (http.Handler, error) {
	if dir == "" {
		return nil, fmt.Errorf("no decoy directory configured")
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		return nil, fmt.Errorf("decoy directory has no index.html: %w", err)
	}
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean before touching the filesystem. An unclean path would stat
		// outside dir, and the hit/miss shows up in the status a prober sees.
		target := path.Clean("/" + r.URL.Path)
		rel := strings.TrimPrefix(target, "/")
		fsPath := filepath.Join(dir, filepath.FromSlash(rel))
		// Serve index.html for unknown paths so the decoy looks like a site
		// rather than exposing a 404 that differs from the real thing.
		if rel == "" {
			rel, fsPath = "index.html", filepath.Join(dir, "index.html")
		} else if _, err := os.Stat(fsPath); err != nil {
			target, rel, fsPath = "/", "index.html", filepath.Join(dir, "index.html")
		}
		if decoySubstitutedExt[strings.ToLower(filepath.Ext(rel))] && serveDecoyWithOrigin(w, r, fsPath, rel) {
			return
		}
		r = r.Clone(r.Context())
		r.URL.Path, r.URL.RawPath = target, ""
		fs.ServeHTTP(w, r)
	}), nil
}

// serveDecoyWithOrigin serves one text file with every {{DECOY_ORIGIN}}
// replaced by this request's own scheme+host. Reports false on any read
// error so the caller falls through to the plain http.FileServer path rather
// than surface a broken response for what is otherwise a transient problem.
func serveDecoyWithOrigin(w http.ResponseWriter, r *http.Request, fsPath, rel string) bool {
	body, err := os.ReadFile(fsPath)
	if err != nil {
		return false
	}
	if bytes.Contains(body, []byte(decoyOriginToken)) {
		body = bytes.ReplaceAll(body, []byte(decoyOriginToken), []byte("https://"+r.Host))
	}
	contentType := mime.TypeByExtension(filepath.Ext(rel))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
	return true
}

// newProxyDecoy reverse-proxies to a site the admin already hosts elsewhere.
func newProxyDecoy(raw string) (http.Handler, error) {
	if raw == "" {
		return nil, fmt.Errorf("no decoy proxy URL configured")
	}
	target, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid decoy proxy URL: %w", err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("decoy proxy URL must be http or https, got %q", target.Scheme)
	}
	if target.Host == "" {
		return nil, fmt.Errorf("decoy proxy URL has no host")
	}
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// SetURL leaves Host derived from the target, which is exactly
			// what name-based virtual hosting on the far side needs.
			pr.SetURL(target)
			pr.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			w.WriteHeader(http.StatusBadGateway)
		},
	}, nil
}
