// Copyright (c) 2026 Masterain. MIT License.
// Adapted from PIA-Wireguard-Config-Generator-GUI (commit 53686fcd).
package pia

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
)

func readLimitedBody(body io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return raw, nil
}

func expectedContentType(header string, accepted ...string) bool {
	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		return false
	}
	for _, candidate := range accepted {
		if strings.EqualFold(mediaType, candidate) {
			return true
		}
	}
	return false
}

func noRedirect(_ *http.Request, _ []*http.Request) error {
	return errors.New("redirects are disabled for this request")
}

func decodeSingleJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func classifyNetworkError(ctx context.Context, fallback, message string, err error) error {
	cause := redactNetErr(err)
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return WrapError(CodeCancelled, "The operation was cancelled.", cause)
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return WrapError(CodeTimeout, "The network request timed out.", cause)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return WrapError(CodeTimeout, "The network request timed out.", cause)
	}
	var unknownAuthority x509.UnknownAuthorityError
	var hostnameError x509.HostnameError
	var invalidCertificate x509.CertificateInvalidError
	if errors.As(err, &unknownAuthority) || errors.As(err, &hostnameError) || errors.As(err, &invalidCertificate) {
		return WrapError(CodeTLSValidation, "PIA's server identity could not be verified.", cause)
	}
	return WrapError(fallback, message, cause)
}

type redactedCause struct{ kind string }

func (e redactedCause) Error() string { return e.kind }

func redactNetErr(err error) error {
	if err == nil {
		return nil
	}
	return redactedCause{kind: "network error"}
}

func containsSecret(s string, secrets ...string) bool {
	for _, secret := range secrets {
		if secret != "" && strings.Contains(s, secret) {
			return true
		}
	}
	return false
}
