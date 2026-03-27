// Package transport defines the transport layer abstraction for equipment communication.
package transport

import "context"

// State represents the current transport connection state.
type State int

const (
	StateDisconnected State = iota
	StateConnecting
	StateConnected
	StateSelected // HSMS-specific: connection selected for data exchange
)

func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "DISCONNECTED"
	case StateConnecting:
		return "CONNECTING"
	case StateConnected:
		return "CONNECTED"
	case StateSelected:
		return "SELECTED"
	default:
		return "UNKNOWN"
	}
}

// Transport is the interface for protocol transport layers (HSMS, TCP, Serial, etc.).
type Transport interface {
	// Connect establishes the underlying connection.
	Connect(ctx context.Context) error

	// Send transmits raw bytes over the connection.
	Send(ctx context.Context, data []byte) error

	// Receive reads raw bytes from the connection. Blocks until data is available.
	Receive(ctx context.Context) ([]byte, error)

	// Close shuts down the connection gracefully.
	Close() error

	// State returns the current connection state.
	State() State
}
