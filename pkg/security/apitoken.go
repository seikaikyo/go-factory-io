package security

import (
	"crypto/subtle"
	"fmt"
	"net"
	"os"
	"strings"
)

// Environment variables consulted when a token flag is left empty.
const (
	// EnvAPIToken supplies the read-scope bearer token for the REST/gRPC API.
	EnvAPIToken = "SECSGEM_API_TOKEN"

	// EnvAPIWriteToken supplies the write-scope bearer token. Write endpoints
	// stay closed until this is set.
	EnvAPIWriteToken = "SECSGEM_API_WRITE_TOKEN"

	// EnvStudioToken supplies the token for the Studio web UI and its
	// WebSocket control channel.
	EnvStudioToken = "SECSGEM_STUDIO_TOKEN"

	// EnvCORSOrigins supplies a comma-separated browser origin allowlist.
	EnvCORSOrigins = "SECSGEM_CORS_ORIGINS"
)

// TokenFromEnv returns flagValue when non-empty, otherwise the value of the
// named environment variable. Surrounding whitespace is trimmed so a token
// pasted with a trailing newline still matches.
func TokenFromEnv(flagValue, envName string) string {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv(envName))
}

// CompareToken reports whether presented equals expected, in constant time.
// An empty expected value never matches, so an unconfigured token cannot be
// satisfied by sending an empty credential.
func CompareToken(expected, presented string) bool {
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(presented)) == 1
}

// BearerToken extracts the credential from an HTTP Authorization header value.
// The scheme match is case-insensitive per RFC 7235.
func BearerToken(header string) (string, bool) {
	const scheme = "bearer "
	if len(header) < len(scheme) || !strings.EqualFold(header[:len(scheme)], scheme) {
		return "", false
	}
	token := strings.TrimSpace(header[len(scheme):])
	if token == "" {
		return "", false
	}
	return token, true
}

// IsLoopbackListen reports whether a listen address binds to loopback only.
// A wildcard host (":8080", "0.0.0.0:8080", "[::]:8080") or an empty host is
// treated as reachable from the network, so it returns false.
func IsLoopbackListen(addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// RequireTokenForListen enforces the fail-closed rule that a listener reachable
// beyond loopback must carry a token. It returns an error naming envName so the
// operator knows exactly what to set.
func RequireTokenForListen(token, addr, envName string) error {
	if token != "" || IsLoopbackListen(addr) {
		return nil
	}
	return fmt.Errorf(
		"security: refusing to serve %s without a token; set --%s or %s (bind 127.0.0.1 for local-only use)",
		addr, flagNameFor(envName), envName,
	)
}

func flagNameFor(envName string) string {
	switch envName {
	case EnvAPIToken:
		return "api-token"
	case EnvAPIWriteToken:
		return "api-write-token"
	case EnvStudioToken:
		return "studio-token"
	default:
		return "token"
	}
}

// ParseOrigins splits a comma-separated browser origin allowlist. Blank entries
// are dropped, so an unset value yields nil (same-origin only).
func ParseOrigins(list string) []string {
	var out []string
	for _, part := range strings.Split(list, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// OriginAllowed reports whether origin appears in the allowlist. Matching is
// case-insensitive on the scheme and host, which is how browsers serialize an
// Origin header. An empty allowlist denies everything.
func OriginAllowed(allowlist []string, origin string) bool {
	if origin == "" {
		return false
	}
	for _, a := range allowlist {
		if strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}
