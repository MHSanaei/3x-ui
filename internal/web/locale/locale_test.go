package locale

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

type fixedTgLang string

func (l fixedTgLang) GetTgLang() (string, error) { return string(l), nil }

func initTestBundle(t *testing.T) {
	t.Helper()
	translations := fstest.MapFS{
		"translation/en-US.json": {Data: []byte(`{"greeting": "Hello {{ .Name }}"}`)},
		"translation/fa-IR.json": {Data: []byte(`{"greeting": "سلام {{ .Name }}"}`)},
	}
	if err := InitLocalizer(translations, fixedTgLang("en-US")); err != nil {
		t.Fatalf("InitLocalizer: %v", err)
	}
}

func localizedContext(t *testing.T, lang string) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.AddCookie(&http.Cookie{Name: "lang", Value: lang})
	LocalizerMiddleware()(c)
	return c
}

func webGreeting(t *testing.T, c *gin.Context) string {
	t.Helper()
	fn, ok := c.MustGet("I18n").(func(I18nType, string, ...string) string)
	if !ok {
		t.Fatalf("I18n context value has type %T", c.MustGet("I18n"))
	}
	return fn(Web, "greeting", "Name==Ali")
}

// The web localizer was one package global that every request overwrote, so a
// request was answered in whatever language the latest other request asked for.
func TestWebLocalizerIsPerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initTestBundle(t)

	persian := localizedContext(t, "fa-IR")
	english := localizedContext(t, "en-US")

	if got, want := webGreeting(t, persian), "سلام Ali"; got != want {
		t.Fatalf("fa-IR request localized to %q, want %q", got, want)
	}
	if got, want := webGreeting(t, english), "Hello Ali"; got != want {
		t.Fatalf("en-US request localized to %q, want %q", got, want)
	}
}
