package hsms

import "time"

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
}

// DefaultConfig returns a Config with SEMI E37 recommended defaults.
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
