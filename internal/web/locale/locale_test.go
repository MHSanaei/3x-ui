package locale

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

type fixedLang string

func (l fixedLang) GetTgLang() (string, error) { return string(l), nil }

// The Telegram bot re-runs InitLocalizer while the panel serves requests; a request must
// never read the bundle while it is being filled (a concurrent map access kills the process).
func TestInitLocalizerWhileServingRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	translations := fstest.MapFS{"translation/en_US.json": {Data: []byte(`{"hello": "Hello"}`)}}
	engine := gin.New()
	engine.Use(LocalizerMiddleware())
	engine.GET("/", func(c *gin.Context) { c.String(http.StatusOK, I18nForLang("en-US", "hello")) })

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 200 {
			if err := InitLocalizer(translations, fixedLang("en-US")); err != nil {
				t.Errorf("InitLocalizer: %v", err)
				return
			}
		}
	})
	for range 4 {
		wg.Go(func() {
			for range 200 {
				engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
			}
		})
	}
	wg.Wait()
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Body.String() != "Hello" {
		t.Fatalf("after re-initialisation the bundle answers %q, want Hello", rec.Body.String())
	}
}
