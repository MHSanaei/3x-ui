package oauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

const testKeyID = "test-key"

// fakeIDP is a minimal OIDC provider: discovery, JWKS, and a token endpoint
// that returns a signed ID token carrying whatever claims the test sets.
type fakeIDP struct {
	t          *testing.T
	priv       *rsa.PrivateKey
	issuer     string
	idTokenAud string
	nonce      string
	email      string
	groups     []string
	server     *httptest.Server
}

func newFakeIDP(t *testing.T) *fakeIDP {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &fakeIDP{t: t, priv: priv, idTokenAud: "test-client"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                 idp.issuer,
			"authorization_endpoint": idp.issuer + "/auth",
			"token_endpoint":         idp.issuer + "/token",
			"jwks_uri":               idp.issuer + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key:       &priv.PublicKey,
			KeyID:     testKeyID,
			Algorithm: "RS256",
			Use:       "sig",
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"access_token": "at",
			"token_type":   "Bearer",
			"id_token":     idp.signIDToken(),
		})
	})

	idp.server = httptest.NewServer(mux)
	idp.issuer = idp.server.URL
	t.Cleanup(idp.server.Close)
	return idp
}

func (idp *fakeIDP) signIDToken() string {
	idp.t.Helper()
	claims := map[string]any{
		"iss": idp.issuer,
		"sub": "subject-123",
		"aud": idp.idTokenAud,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	if idp.nonce != "" {
		claims["nonce"] = idp.nonce
	}
	if idp.email != "" {
		claims["email"] = idp.email
	}
	if idp.groups != nil {
		claims["groups"] = idp.groups
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: idp.priv},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", testKeyID),
	)
	if err != nil {
		idp.t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		idp.t.Fatal(err)
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		idp.t.Fatal(err)
	}
	serialized, err := jws.CompactSerialize()
	if err != nil {
		idp.t.Fatal(err)
	}
	return serialized
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (idp *fakeIDP) provider(t *testing.T) *Provider {
	t.Helper()
	p, err := NewProvider(context.Background(), config.OAuthConfig{
		Issuer:        idp.issuer,
		ClientID:      "test-client",
		ClientSecret:  "secret",
		RedirectURL:   "https://panel.example/oauth/callback",
		Scopes:        []string{"openid", "email", "groups"},
		UsernameClaim: "email",
		GroupsClaim:   "groups",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return p
}

func TestAuthCodeURLCarriesPKCEAndNonce(t *testing.T) {
	idp := newFakeIDP(t)
	p := idp.provider(t)
	fs := FlowState{State: "state-abc", Nonce: "nonce-xyz", Verifier: "verifier-1234567890123456789012345678901234567890"}

	raw := p.AuthCodeURL(fs)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if got := q.Get("state"); got != "state-abc" {
		t.Errorf("state = %q, want state-abc", got)
	}
	if got := q.Get("nonce"); got != "nonce-xyz" {
		t.Errorf("nonce = %q, want nonce-xyz", got)
	}
	if got := q.Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}
	if q.Get("code_challenge") == "" {
		t.Error("code_challenge is empty")
	}
	if got := q.Get("client_id"); got != "test-client" {
		t.Errorf("client_id = %q, want test-client", got)
	}
	if got := q.Get("scope"); !strings.Contains(got, "openid") {
		t.Errorf("scope = %q, want it to contain openid", got)
	}
}

func TestExchangeHappyPath(t *testing.T) {
	idp := newFakeIDP(t)
	idp.nonce = "expected-nonce"
	idp.email = "alice@example.io"
	idp.groups = []string{"admins", "staff"}
	p := idp.provider(t)

	id, err := p.Exchange(context.Background(), "any-code", "verifier", "expected-nonce")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if id.Subject != "subject-123" {
		t.Errorf("Subject = %q, want subject-123", id.Subject)
	}
	if id.Username != "alice@example.io" || id.Email != "alice@example.io" {
		t.Errorf("Username/Email = %q/%q, want alice@example.io", id.Username, id.Email)
	}
	if !id.InGroup("admins") {
		t.Errorf("groups = %v, want to contain admins", id.Groups)
	}
}

func TestExchangeRejectsNonceMismatch(t *testing.T) {
	idp := newFakeIDP(t)
	idp.nonce = "token-nonce"
	p := idp.provider(t)

	_, err := p.Exchange(context.Background(), "any-code", "verifier", "different-nonce")
	if err == nil || !strings.Contains(err.Error(), "nonce mismatch") {
		t.Fatalf("err = %v, want nonce mismatch", err)
	}
}

func TestExchangeRejectsWrongAudience(t *testing.T) {
	idp := newFakeIDP(t)
	idp.idTokenAud = "some-other-client"
	idp.nonce = "n"
	p := idp.provider(t)

	_, err := p.Exchange(context.Background(), "any-code", "verifier", "n")
	if err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("err = %v, want verification failure on audience", err)
	}
}
