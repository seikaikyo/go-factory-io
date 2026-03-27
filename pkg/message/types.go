// Package message defines the common message types for factory communication protocols.
package message

// Message represents a protocol-agnostic equipment message.
type Message struct {
	Stream   byte
	Function byte
	WBit     bool // Wait bit: true = reply expected
	SystemID uint32
	Body     interface{} // Protocol-specific body (e.g., *secs2.Item)
}

// Direction indicates message flow direction.
type Direction int

const (
	HostToEquipment Direction = iota
	EquipmentToHost
)

// IsReply returns true if this is a reply message (even function number).
func (m *Message) IsReply() bool {
	return m.Function%2 == 0
}

// IsPrimary returns true if this is a primary message (odd function number).
func (m *Message) IsPrimary() bool {
	return m.Function%2 == 1
}
