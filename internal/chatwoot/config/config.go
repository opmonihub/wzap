// Package config validates the per-instance Chatwoot connector
// configuration. Disabled configurations always pass; enabled ones must
// carry the credentials the mirror needs. Field defaults are applied by the
// caller through DefaultInbox and DefaultDelimiter, never inside Validate.
package config

import (
	"net"
	"net/url"
	"strings"

	"wzap/internal/model"
)

// ErrField reports which configuration field rejected the write. Handlers map
// every error wrapping it to 422 without persisting anything.
type ErrField struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e ErrField) Error() string {
	return e.Field + ": " + e.Message
}

// Validate checks a Chatwoot connector configuration with set semantics. A
// disabled config passes with empty fields. An enabled config requires url,
// account_id and token; a malformed URL is rejected. SignMsg is a Go bool so
// non-boolean payloads are rejected at the REST boundary, not here.
func Validate(c model.ChatwootConfig) error {
	if !c.Enabled {
		return nil
	}
	raw := strings.TrimSpace(c.URL)
	if raw == "" {
		return &ErrField{Field: "url", Message: "url is required"}
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return &ErrField{Field: "url", Message: "malformed url"}
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		if parsed.Hostname() == "" {
			return &ErrField{Field: "url", Message: "url has no host"}
		}
		// The token travels as api_access_token on every call: plain http
		// would expose it on the wire, so only loopback (local dev) may
		// stay unencrypted.
		if strings.EqualFold(parsed.Scheme, "http") && !isLoopbackHost(parsed.Hostname()) {
			return &ErrField{Field: "url", Message: "http requires a loopback host, use https"}
		}
	default:
		return &ErrField{Field: "url", Message: "scheme must be http or https"}
	}
	if strings.TrimSpace(c.AccountID) == "" {
		return &ErrField{Field: "account_id", Message: "account_id is required"}
	}
	if strings.TrimSpace(c.Token) == "" {
		return &ErrField{Field: "token", Message: "token is required"}
	}
	return nil
}

// isLoopbackHost reports whether host is a loopback destination without DNS:
// "localhost" (case-insensitive, optional trailing dot) or a literal IP in
// 127.0.0.0/8 or ::1. It mirrors the webhook gate so both boundaries agree on
// the local-dev exception.
func isLoopbackHost(host string) bool {
	trimmed := strings.TrimSuffix(host, ".")
	if strings.EqualFold(trimmed, "localhost") {
		return true
	}
	if ip := net.ParseIP(trimmed); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// DefaultInbox returns the inbox name used when name_inbox is empty: the
// instance identifier.
func DefaultInbox(instanceName string) string {
	return instanceName
}

// DefaultDelimiter returns the signature delimiter used when sign_delimiter
// is empty: a line break.
func DefaultDelimiter() string {
	return "\n"
}
