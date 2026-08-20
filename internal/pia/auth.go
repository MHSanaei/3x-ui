// Copyright (c) 2026 Masterain. MIT License.
// Adapted from PIA-Wireguard-Config-Generator-GUI (commit 53686fcd).
package pia

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type AuthClient struct {
	Endpoint   string
	HTTPClient *http.Client
	MaxBody    int64
	UserAgent  string
	Now        func() time.Time
}

func NewAuthClient(endpoint string) *AuthClient {
	return &AuthClient{
		Endpoint:   endpoint,
		MaxBody:    DefaultMaxResponseBody,
		UserAgent:  DefaultUserAgent,
		Now:        time.Now,
		HTTPClient: &http.Client{Timeout: DefaultRequestTimeout, CheckRedirect: noRedirect},
	}
}

func (c *AuthClient) Authenticate(ctx context.Context, username string, password []byte) (Token, error) {
	username = strings.TrimSpace(username)
	if !validSecret([]byte(username), 1, 256) || !validSecret(password, 1, 1024) {
		return Token{}, NewError(CodeInvalidCredentials, "Enter a valid PIA username and password.")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("username", username); err != nil {
		return Token{}, WrapError(CodeAuthenticationUnavailable, "Could not prepare the authentication request.", err)
	}
	if err := writer.WriteField("password", string(password)); err != nil {
		return Token{}, WrapError(CodeAuthenticationUnavailable, "Could not prepare the authentication request.", err)
	}
	if err := writer.Close(); err != nil {
		return Token{}, WrapError(CodeAuthenticationUnavailable, "Could not prepare the authentication request.", err)
	}
	defer func() {
		raw := body.Bytes()
		for i := range raw {
			raw[i] = 0
		}
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, &body)
	if err != nil {
		return Token{}, WrapError(CodeAuthenticationUnavailable, "The authentication endpoint is invalid.", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.UserAgent)
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return Token{}, classifyNetworkError(ctx, CodeAuthenticationUnavailable, "PIA authentication could not be reached.", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return Token{}, NewError(CodeInvalidCredentials, "The PIA username or password was rejected.")
	}
	if response.StatusCode != http.StatusOK {
		return Token{}, NewError(CodeAuthenticationUnavailable, fmt.Sprintf("PIA authentication returned HTTP %d.", response.StatusCode))
	}
	if !expectedContentType(response.Header.Get("Content-Type"), "application/json") {
		return Token{}, NewError(CodeAuthenticationUnavailable, "PIA authentication returned an unexpected content type.")
	}
	raw, err := readLimitedBody(response.Body, c.MaxBody)
	if err != nil {
		return Token{}, WrapError(CodeAuthenticationUnavailable, "PIA authentication returned an invalid response.", err)
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := decodeSingleJSON(raw, &payload); err != nil || !validSecret([]byte(payload.Token), 16, 4096) {
		return Token{}, NewError(CodeAuthenticationUnavailable, "PIA authentication returned an invalid token response.")
	}
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	return Token{Value: []byte(payload.Token), ExpiresAt: now().Add(DefaultTokenTTL)}, nil
}
