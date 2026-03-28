package security

import (
	"sync"
	"sync/atomic"
	"time"
)

// SecurityStatus represents the cybersecurity status per SEMI E191.
// Exposes counters and state for monitoring by fab security systems.
type SecurityStatus struct {
	mu sync.RWMutex

	// Connection security
	TLSEnabled     bool   `json:"tls_enabled"`
	TLSVersion     string `json:"tls_version,omitempty"`
	MutualTLS      bool   `json:"mutual_tls"`
	CipherSuite    string `json:"cipher_suite,omitempty"`

	// Policy
	RBACEnabled    bool   `json:"rbac_enabled"`
	IPAllowlist    bool   `json:"ip_allowlist_enabled"`
	RateLimited    bool   `json:"rate_limit_enabled"`
	EncryptionEnabled bool `json:"encryption_enabled"`

	// Counters (atomic for concurrent access)
	AuthFailures     atomic.Int64 `json:"-"`
	ConnectionsTotal atomic.Int64 `json:"-"`
	RejectedConns    atomic.Int64 `json:"-"`
	RateLimitHits    atomic.Int64 `json:"-"`
	MalformedMsgs    atomic.Int64 `json:"-"`
	UnauthorizedMsgs atomic.Int64 `json:"-"`

	// Timestamps
	StartTime        time.Time `json:"start_time"`
	LastAuthFailure  time.Time `json:"last_auth_failure,omitempty"`
	LastRateLimit    time.Time `json:"last_rate_limit,omitempty"`
}

// NewSecurityStatus creates a new E191 security status tracker.
func NewSecurityStatus() *SecurityStatus {
	return &SecurityStatus{
		StartTime: time.Now(),
	}
}

// Report returns a snapshot of the security status as a map.
// Suitable for JSON serialization via REST API.
func (ss *SecurityStatus) Report() map[string]interface{} {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	return map[string]interface{}{
		"tls_enabled":         ss.TLSEnabled,
		"tls_version":         ss.TLSVersion,
		"mutual_tls":          ss.MutualTLS,
		"cipher_suite":        ss.CipherSuite,
		"rbac_enabled":        ss.RBACEnabled,
		"ip_allowlist_enabled": ss.IPAllowlist,
		"rate_limit_enabled":  ss.RateLimited,
		"encryption_enabled":  ss.EncryptionEnabled,
		"counters": map[string]interface{}{
			"auth_failures":     ss.AuthFailures.Load(),
			"connections_total": ss.ConnectionsTotal.Load(),
			"rejected_conns":    ss.RejectedConns.Load(),
			"rate_limit_hits":   ss.RateLimitHits.Load(),
			"malformed_msgs":    ss.MalformedMsgs.Load(),
			"unauthorized_msgs": ss.UnauthorizedMsgs.Load(),
		},
		"start_time":        ss.StartTime.UTC().Format(time.RFC3339),
		"uptime_seconds":    int(time.Since(ss.StartTime).Seconds()),
		"last_auth_failure": formatTimeOrEmpty(ss.LastAuthFailure),
		"last_rate_limit":   formatTimeOrEmpty(ss.LastRateLimit),
	}
}

// RecordAuthFailure increments the auth failure counter.
func (ss *SecurityStatus) RecordAuthFailure() {
	ss.AuthFailures.Add(1)
	ss.mu.Lock()
	ss.LastAuthFailure = time.Now()
	ss.mu.Unlock()
}

// RecordConnection increments the total connection counter.
func (ss *SecurityStatus) RecordConnection() {
	ss.ConnectionsTotal.Add(1)
}

// RecordRejectedConnection increments the rejected connection counter.
func (ss *SecurityStatus) RecordRejectedConnection() {
	ss.RejectedConns.Add(1)
}

// RecordRateLimit increments the rate limit hit counter.
func (ss *SecurityStatus) RecordRateLimit() {
	ss.RateLimitHits.Add(1)
	ss.mu.Lock()
	ss.LastRateLimit = time.Now()
	ss.mu.Unlock()
}

// RecordMalformed increments the malformed message counter.
func (ss *SecurityStatus) RecordMalformed() {
	ss.MalformedMsgs.Add(1)
}

// RecordUnauthorized increments the unauthorized message counter.
func (ss *SecurityStatus) RecordUnauthorized() {
	ss.UnauthorizedMsgs.Add(1)
}

// SetTLS updates TLS configuration status.
func (ss *SecurityStatus) SetTLS(enabled bool, version, cipher string, mutual bool) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.TLSEnabled = enabled
	ss.TLSVersion = version
	ss.CipherSuite = cipher
	ss.MutualTLS = mutual
}

// AsEventHandler returns an EventHandler that auto-updates counters from security events.
func (ss *SecurityStatus) AsEventHandler() EventHandler {
	return func(event Event) {
		switch event.Type {
		case "auth_failed":
			ss.RecordAuthFailure()
		case "connection_rejected":
			ss.RecordRejectedConnection()
		case "rate_limited":
			ss.RecordRateLimit()
		case "malformed_message":
			ss.RecordMalformed()
		case "unauthorized_message":
			ss.RecordUnauthorized()
		}
	}
}

func formatTimeOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
