// Copyright (c) 2026 Masterain. MIT License.
// Adapted from PIA-Wireguard-Config-Generator-GUI (commit 53686fcd).
package pia

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type ServerListSource interface {
	Fetch(ctx context.Context) (ServerListSnapshot, error)
}

type ServerListSnapshot struct {
	Payload           []byte
	SchemaHint        string
	SignatureVerified bool
}

type CatalogClient struct {
	Endpoint     string
	PublicKeyPEM []byte
	HTTPClient   *http.Client
	MaxBody      int64
	UserAgent    string
}

func NewCatalogClient(endpoint string, publicKey []byte) *CatalogClient {
	return &CatalogClient{
		Endpoint:     endpoint,
		PublicKeyPEM: publicKey,
		MaxBody:      DefaultMaxServerListBody,
		UserAgent:    DefaultUserAgent,
		HTTPClient:   &http.Client{Timeout: DefaultRequestTimeout, CheckRedirect: noRedirect},
	}
}

func (c *CatalogClient) Fetch(ctx context.Context) (ServerListSnapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint, nil)
	if err != nil {
		return ServerListSnapshot{}, WrapError(CodeCatalogUnavailable, "The PIA region-list endpoint is invalid.", err)
	}
	request.Header.Set("Accept", "application/json, text/plain;q=0.9")
	request.Header.Set("User-Agent", c.UserAgent)
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return ServerListSnapshot{}, classifyNetworkError(ctx, CodeCatalogUnavailable, "The PIA region list could not be downloaded.", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ServerListSnapshot{}, NewError(CodeCatalogUnavailable, fmt.Sprintf("PIA returned HTTP %d for the region list.", response.StatusCode))
	}
	if !expectedContentType(response.Header.Get("Content-Type"), "application/json", "text/plain", "application/octet-stream") {
		return ServerListSnapshot{}, NewError(CodeCatalogSchemaUnsupported, "PIA returned an unexpected region-list content type.")
	}
	raw, err := readLimitedBody(response.Body, c.MaxBody)
	if err != nil {
		return ServerListSnapshot{}, WrapError(CodeCatalogUnavailable, "The PIA region-list response is too large or incomplete.", err)
	}
	verified, err := VerifySignedServerList(raw, c.PublicKeyPEM)
	if err != nil {
		return ServerListSnapshot{}, err
	}
	return ServerListSnapshot{Payload: verified, SchemaHint: schemaHint(c.Endpoint), SignatureVerified: true}, nil
}

func schemaHint(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	base := strings.ToLower(path.Base(parsed.Path))
	if base == "v6" || base == "v7" {
		return strings.TrimPrefix(base, "v")
	}
	return ""
}
