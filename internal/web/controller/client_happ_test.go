package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type fakeHappLinkGenerator struct {
	calls    int
	clientID int
	host     string
	result   service.HappLinkResult
	err      error
}

func (f *fakeHappLinkGenerator) Generate(_ context.Context, clientID int, host string) (service.HappLinkResult, error) {
	f.calls++
	f.clientID = clientID
	f.host = host
	return f.result, f.err
}

func newHappClientTestRouter(generator service.HappLinkGenerator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("I18n", func(_ locale.I18nType, key string, _ ...string) string { return key })
		c.Next()
	})
	(&ClientController{happGenerator: generator}).initRouter(router.Group("/clients"))
	return router
}

func TestGenerateHappLinkForwardsCurrentRequestAndReturnsOnlyLink(t *testing.T) {
	fake := &fakeHappLinkGenerator{result: service.HappLinkResult{EncryptedLink: "happ://crypt5/fresh"}}
	router := newHappClientTestRouter(fake)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/clients/happLink/42", nil)
	req.Host = "panel.example.com:2053"
	router.ServeHTTP(rec, req)

	if fake.clientID != 42 || fake.host != "panel.example.com:2053" {
		t.Fatalf("Generate args = %d, %q", fake.clientID, fake.host)
	}
	if fake.calls != 1 {
		t.Fatalf("Generate calls = %d", fake.calls)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Success bool            `json:"success"`
		Msg     string          `json:"msg"`
		Obj     json.RawMessage `json:"obj"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !response.Success || response.Msg != "" {
		t.Fatalf("response envelope = success:%t msg:%q", response.Success, response.Msg)
	}
	var link map[string]string
	if err := json.Unmarshal(response.Obj, &link); err != nil {
		t.Fatalf("unmarshal link result: %v", err)
	}
	if len(link) != 1 || link["encryptedLink"] != "happ://crypt5/fresh" {
		t.Fatalf("response obj = %#v", link)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response envelope: %v", err)
	}
	if len(envelope) != 3 || envelope["success"] == nil || envelope["msg"] == nil || envelope["obj"] == nil {
		t.Fatalf("response envelope fields = %#v", envelope)
	}
}

func TestGenerateHappLinkRejectsInvalidIDWithoutCallingGenerator(t *testing.T) {
	fake := &fakeHappLinkGenerator{}
	router := newHappClientTestRouter(fake)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/clients/happLink/0", nil))

	if fake.calls != 0 || fake.clientID != 0 || fake.host != "" {
		t.Fatalf("Generate called %d times with = %d, %q", fake.calls, fake.clientID, fake.host)
	}
	assertHappFailureWithoutSecret(t, rec, "fake-provider-secret")
}

func TestGenerateHappLinkDoesNotExposeProviderFailure(t *testing.T) {
	fake := &fakeHappLinkGenerator{err: errors.New("fake-provider-secret")}
	router := newHappClientTestRouter(fake)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/clients/happLink/42", nil))

	if fake.calls != 1 {
		t.Fatalf("Generate calls = %d", fake.calls)
	}
	assertHappFailureWithoutSecret(t, rec, "fake-provider-secret")
}

func assertHappFailureWithoutSecret(t *testing.T, rec *httptest.ResponseRecorder, secret string) {
	t.Helper()
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"success":false`) {
		t.Fatalf("failure response = %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("failure leaked provider secret: %s", rec.Body.String())
	}
}
