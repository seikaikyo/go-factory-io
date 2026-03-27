package hsms

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/security"
)

// Role determines whether this endpoint initiates (Active) or accepts (Passive) connections.
type Role int

const (
	RoleActive  Role = iota // Host: initiates TCP connection
	RolePassive             // Equipment: listens for TCP connections
)

func (r Role) String() string {
	if r == RoleActive {
		return "Active"
	}
	return "Passive"
}

// Config holds HSMS connection parameters per SEMI E37.
type Config struct {
	// Address is the TCP address. For Active: host:port to connect to.
	// For Passive: :port to listen on.
	Address string

	// Role determines active (connect) or passive (listen) mode.
	Role Role

	// SessionID is the device/session ID used in HSMS headers.
	SessionID uint16

	// T3 is the reply timeout. If a reply is not received within T3 after
	// sending a primary message, the transaction is considered failed.
	// SEMI E37 default: 45 seconds. Range: 1-120 seconds.
	T3 time.Duration

	// T5 is the connect separation timeout. After a connection failure,
	// wait at least T5 before attempting to reconnect.
	// SEMI E37 default: 10 seconds. Range: 1-240 seconds.
	T5 time.Duration

	// T6 is the control transaction timeout. Time allowed for a control
	// message response (Select.rsp, Deselect.rsp, Linktest.rsp).
	// SEMI E37 default: 5 seconds. Range: 1-240 seconds.
	T6 time.Duration

	// T7 is the not-selected timeout. After TCP connection is established,
	// if Select.req is not received within T7, the connection is dropped.
	// SEMI E37 default: 10 seconds. Range: 1-240 seconds.
	T7 time.Duration

	// T8 is the network intercharacter timeout. Maximum time between
	// successive bytes of a single HSMS message.
	// SEMI E37 default: 5 seconds. Range: 1-120 seconds.
	T8 time.Duration

	// LinktestInterval is how often to send Linktest.req to verify the connection.
	// Set to 0 to disable periodic linktest.
	LinktestInterval time.Duration

	// --- Security (IEC 62443 / SEMI E187) ---

	// TLSConfig enables TLS encryption for the HSMS connection.
	// nil = plaintext (for development/testing only).
	// Per IEC 62443 SR 4.1 (Data Confidentiality).
	TLSConfig *tls.Config

	// AllowedPeers restricts connections to these IPs (passive mode).
	// nil = accept all. Per IEC 62443 FR5 (Restricted Data Flow).
	AllowedPeers []net.IP

	// MaxConnections limits concurrent connections in passive mode.
	// 0 = default (10). Per IEC 62443 FR7 (Resource Availability).
	MaxConnections int

	// MaxMessageRate limits messages per second per connection.
	// 0 = unlimited. Per IEC 62443 FR7.
	MaxMessageRate int

	// MaxMessageSize limits the maximum HSMS message size in bytes.
	// 0 = default (16MB). Prevents OOM attacks.
	MaxMessageSize int

	// SessionTTL is the maximum session duration. 0 = no limit.
	// Per NIST SP 800-82 (vendor access time-bounding).
	SessionTTL time.Duration

	// Auditor handles security event logging. nil = no security logging.
	Auditor *security.Auditor
}

// DefaultConfig returns a Config with SEMI E37 recommended defaults.
// Security features are disabled by default for backward compatibility.
func DefaultConfig(address string, role Role, sessionID uint16) Config {
	return Config{
		Address:          address,
		Role:             role,
		SessionID:        sessionID,
		T3:               45 * time.Second,
		T5:               10 * time.Second,
		T6:               5 * time.Second,
		T7:               10 * time.Second,
		T8:               5 * time.Second,
		LinktestInterval: 30 * time.Second,
	}
}

// SecureConfig returns a Config with IEC 62443 SL2 security defaults enabled.
// Requires a TLS config to be provided.
func SecureConfig(address string, role Role, sessionID uint16, tlsConfig *tls.Config) Config {
	cfg := DefaultConfig(address, role, sessionID)
	cfg.TLSConfig = tlsConfig
	cfg.MaxConnections = 10
	cfg.MaxMessageRate = 1000
	cfg.MaxMessageSize = 16 * 1024 * 1024 // 16MB
	cfg.SessionTTL = 24 * time.Hour
	return cfg
}

const defaultMaxMessageSize = 16 * 1024 * 1024 // 16MB

func (c *Config) maxMessageSize() int {
	if c.MaxMessageSize > 0 {
		return c.MaxMessageSize
	}
	return defaultMaxMessageSize
}
