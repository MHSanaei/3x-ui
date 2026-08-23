package frontproxy

import (
	"embed"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// logDecoyFallback records why a configured decoy could not be used. The
// front door still serves a template, so this is a warning, never fatal.
func logDecoyFallback(mode string, err error) {
	logger.Warningf("frontproxy: %s decoy unusable, falling back to template: %v", mode, err)
}

//go:embed templates/*.html
var decoyTemplates embed.FS

// DecoyMode selects what the front door shows for every path that is not one
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

// DecoyConfig describes the decoy half of the front door.
type DecoyConfig struct {
	Mode     DecoyMode
	Template string
	Dir      string
	ProxyURL string
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
	return newTemplateDecoy(cfg.Template)
}

// newTemplateDecoy serves one embedded page for every path, so a prober sees
// a consistent site instead of a directory of guessable files.
func newTemplateDecoy(name string) http.Handler {
	body, err := readDecoyTemplate(name)
	if err != nil {
		body, _ = readDecoyTemplate(DefaultDecoyTemplate)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		_, _ = w.Write(body)
	})
}

// readDecoyTemplate loads an embedded page by name, rejecting any name that
// could reach outside the embedded template set.
func readDecoyTemplate(name string) ([]byte, error) {
	if name == "" {
		name = DefaultDecoyTemplate
	}
	if strings.ContainsAny(name, `/\.`) {
		return nil, fmt.Errorf("invalid decoy template name %q", name)
	}
	return decoyTemplates.ReadFile("templates/" + name + ".html")
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
		// Serve index.html for unknown paths so the decoy looks like a site
		// rather than exposing a 404 that differs from the real thing.
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fs.ServeHTTP(w, r)
	}), nil
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
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Present the upstream's own hostname so name-based virtual hosting on
	// the far side resolves to the site the admin actually pointed at.
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.Host = target.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusBadGateway)
	}
	return proxy, nil
}
