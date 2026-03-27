// Package driver defines the equipment driver abstraction.
package driver

import "context"

// Config holds driver configuration.
type Config struct {
	// SessionID for HSMS communication.
	SessionID uint16

	// Address for the equipment connection.
	Address string

	// ModelName is the equipment model identifier (MDLN).
	ModelName string

	// SoftwareRevision is the software version (SOFTREV).
	SoftwareRevision string
}

// EventHandler processes collection events from equipment.
type EventHandler func(ctx context.Context, event Event)

// Event represents a collection event with associated report data.
type Event struct {
	CEID    uint32
	Reports []Report
}

// Report holds report data associated with an event.
type Report struct {
	RPTID     uint32
	Variables []Variable
}

// Variable holds a status variable value.
type Variable struct {
	VID   uint32
	Value interface{}
}
