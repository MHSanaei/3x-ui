package pia

import (
	"errors"
	"fmt"
)

const (
	CodeInvalidInput              = "pia_invalid_input"
	CodeInvalidCredentials        = "pia_invalid_credentials"
	CodeAuthenticationUnavailable = "pia_authentication_unavailable"
	CodeTokenRejected             = "pia_token_rejected"
	CodeCatalogUnavailable        = "pia_catalog_unavailable"
	CodeCatalogSignatureInvalid   = "pia_catalog_signature_invalid"
	CodeCatalogSchemaUnsupported  = "pia_catalog_schema_unsupported"
	CodeServerNotFound            = "pia_server_not_found"
	CodeTLSValidation             = "pia_tls_validation"
	CodeRegistrationRejected      = "pia_registration_rejected"
	CodeRegistrationInvalid       = "pia_registration_response_invalid"
	CodeTimeout                   = "pia_timeout"
	CodeCancelled                 = "pia_cancelled"
	CodeNetworkUnavailable        = "pia_network_unavailable"
)

type Error struct {
	Code    string
	Message string
	cause   error
}

func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func WrapError(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, cause: cause}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func CodeOf(err error) string {
	if err == nil {
		return ""
	}
	var pe *Error
	if errors.As(err, &pe) && pe != nil {
		return pe.Code
	}
	return CodeNetworkUnavailable
}

func MessageOf(err error) string {
	var pe *Error
	if errors.As(err, &pe) && pe != nil {
		return pe.Message
	}
	if err == nil {
		return ""
	}
	return "An unexpected error occurred."
}
