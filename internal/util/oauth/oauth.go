// Package oauth wraps go-oidc + x/oauth2 into the small surface the panel needs:
// discovery, an Authorization-Code + PKCE login URL, and a code exchange that
// verifies the ID token and extracts the caller's username and groups.
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// Provider is a configured OIDC relying party. Construct it once per login flow
// (discovery hits the network) via NewProvider.
type Provider struct {
	oauth2Config  oauth2.Config
	verifier      *oidc.IDTokenVerifier
	usernameClaim string
	groupsClaim   string
}

// Identity is the verified caller extracted from an ID token.
type Identity struct {
	Subject  string
	Username string
	Email    string
	Groups   []string
}

// FlowState is the per-login CSRF/replay material. State and Nonce must be
// persisted (session) and checked on callback; Verifier is the PKCE secret.
type FlowState struct {
	State    string
	Nonce    string
	Verifier string
}

// NewProvider runs OIDC discovery against cfg.Issuer and builds the relying
// party. It is the only constructor that touches the network.
func NewProvider(ctx context.Context, cfg config.OAuthConfig) (*Provider, error) {
	if cfg.Issuer == "" || cfg.ClientID == "" {
		return nil, errors.New("oauth: issuer and client id are required")
	}
	oidcProvider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oauth: discovery failed: %w", err)
	}
	return &Provider{
		oauth2Config: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     oidcProvider.Endpoint(),
			Scopes:       cfg.Scopes,
		},
		verifier:      oidcProvider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		usernameClaim: cfg.UsernameClaim,
		groupsClaim:   cfg.GroupsClaim,
	}, nil
}

// NewFlowState mints fresh state, nonce, and a PKCE verifier for one login.
func NewFlowState() (FlowState, error) {
	state, err := randToken()
	if err != nil {
		return FlowState{}, err
	}
	nonce, err := randToken()
	if err != nil {
		return FlowState{}, err
	}
	return FlowState{State: state, Nonce: nonce, Verifier: oauth2.GenerateVerifier()}, nil
}

// AuthCodeURL builds the provider authorization URL carrying the state, nonce,
// and PKCE S256 challenge from fs.
func (p *Provider) AuthCodeURL(fs FlowState) string {
	return p.oauth2Config.AuthCodeURL(fs.State,
		oidc.Nonce(fs.Nonce),
		oauth2.S256ChallengeOption(fs.Verifier),
	)
}

// Exchange trades the callback code for tokens, verifies the ID token signature
// and audience, checks the nonce, and returns the extracted identity.
func (p *Provider) Exchange(ctx context.Context, code, verifier, expectedNonce string) (*Identity, error) {
	token, err := p.oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("oauth: token exchange failed: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("oauth: response missing id_token")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oauth: id token verification failed: %w", err)
	}
	if idToken.Nonce != expectedNonce {
		return nil, errors.New("oauth: nonce mismatch")
	}
	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oauth: claim decode failed: %w", err)
	}
	return extractIdentity(idToken.Subject, claims, p.usernameClaim, p.groupsClaim), nil
}

// InGroup reports whether the identity belongs to the named group (exact match).
func (id *Identity) InGroup(name string) bool {
	if name == "" {
		return false
	}
	for _, g := range id.Groups {
		if g == name {
			return true
		}
	}
	return false
}

// InAnyGroup reports whether the identity belongs to any of the named groups.
func (id *Identity) InAnyGroup(names []string) bool {
	for _, n := range names {
		if id.InGroup(n) {
			return true
		}
	}
	return false
}

// extractIdentity pulls the username, email, and groups out of the verified ID
// token claims. Username falls back to email then subject so it is never empty.
func extractIdentity(subject string, claims map[string]any, usernameClaim, groupsClaim string) *Identity {
	email := claimString(claims["email"])
	username := claimString(claims[usernameClaim])
	if username == "" {
		username = email
	}
	if username == "" {
		username = subject
	}
	return &Identity{
		Subject:  subject,
		Username: username,
		Email:    email,
		Groups:   toStringSlice(claims[groupsClaim]),
	}
}

func claimString(v any) string {
	s, _ := v.(string)
	return s
}

// toStringSlice normalizes a groups claim, which providers emit as a JSON array
// of strings or, occasionally, a single string.
func toStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func randToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth: read random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
