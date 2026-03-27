// Package hsms implements the HSMS (High-Speed SECS Message Services) transport
// layer per SEMI E37. HSMS provides TCP/IP-based communication for SECS-II messages.
package hsms

import (
	"encoding/binary"
	"fmt"
)

// Message types (SType) per SEMI E37.
type SType byte

const (
	STypeDataMessage    SType = 0  // SECS-II data message
	STypeSelectReq      SType = 1  // Select.req
	STypeSelectRsp      SType = 2  // Select.rsp
	STypeDeselectReq    SType = 3  // Deselect.req
	STypeDeselectRsp    SType = 4  // Deselect.rsp
	STypeLinktestReq    SType = 5  // Linktest.req
	STypeLinktestRsp    SType = 6  // Linktest.rsp
	STypeRejectReq      SType = 7  // Reject.req
	STypeSeparateReq    SType = 9  // Separate.req
)

func (s SType) String() string {
	switch s {
	case STypeDataMessage:
		return "DataMessage"
	case STypeSelectReq:
		return "Select.req"
	case STypeSelectRsp:
		return "Select.rsp"
	case STypeDeselectReq:
		return "Deselect.req"
	case STypeDeselectRsp:
		return "Deselect.rsp"
	case STypeLinktestReq:
		return "Linktest.req"
	case STypeLinktestRsp:
		return "Linktest.rsp"
	case STypeRejectReq:
		return "Reject.req"
	case STypeSeparateReq:
		return "Separate.req"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// SelectStatus is the status byte in Select.rsp.
type SelectStatus byte

const (
	SelectStatusSuccess              SelectStatus = 0
	SelectStatusAlreadyActive        SelectStatus = 1
	SelectStatusNotReady             SelectStatus = 2
	SelectStatusAlreadyUsed          SelectStatus = 3
)

// Header is the 10-byte HSMS message header.
type Header struct {
	SessionID uint16  // Device ID / Session ID
	WBit      bool    // Wait bit (reply expected)
	Stream    byte    // Stream number (0 for control messages)
	Function  byte    // Function number (0 for control messages)
	PType     byte    // Presentation type (always 0 for SECS-II)
	SType     SType   // Session type (0=data, 1-9=control)
	SystemID  uint32  // Transaction ID for matching req/rsp
}

// Message is a complete HSMS message (header + optional data).
type Message struct {
	Header Header
	Data   []byte // SECS-II encoded body (empty for control messages)
}

// headerLen is the fixed HSMS header size.
const headerLen = 10

// minMessageLen is the minimum HSMS wire message size (4-byte length + 10-byte header).
const minMessageLen = 14

// IsControlMessage returns true if this is a control message (non-zero SType).
func (m *Message) IsControlMessage() bool {
	return m.Header.SType != STypeDataMessage
}

// MarshalBinary serializes the HSMS message to wire format.
//
// Wire format:
//
//	[4 bytes: message length (big-endian, includes header)]
//	[10 bytes: header]
//	[N bytes: data]
func (m *Message) MarshalBinary() ([]byte, error) {
	msgLen := uint32(headerLen + len(m.Data))
	buf := make([]byte, 4+msgLen)

	// Message length (excludes the 4-byte length field itself)
	binary.BigEndian.PutUint32(buf[0:4], msgLen)

	// Session ID
	binary.BigEndian.PutUint16(buf[4:6], m.Header.SessionID)

	// Byte 6: WBit (bit 7) + Stream
	b6 := m.Header.Stream & 0x7F
	if m.Header.WBit {
		b6 |= 0x80
	}
	buf[6] = b6

	// Byte 7: Function
	buf[7] = m.Header.Function

	// Byte 8: PType (always 0)
	buf[8] = m.Header.PType

	// Byte 9: SType
	buf[9] = byte(m.Header.SType)

	// Bytes 10-13: System ID
	binary.BigEndian.PutUint32(buf[10:14], m.Header.SystemID)

	// Data
	copy(buf[14:], m.Data)

	return buf, nil
}

// UnmarshalBinary deserializes an HSMS message from wire format.
// The input must include the 4-byte length prefix.
func (m *Message) UnmarshalBinary(data []byte) error {
	if len(data) < minMessageLen {
		return fmt.Errorf("hsms: message too short: %d bytes (min %d)", len(data), minMessageLen)
	}

	msgLen := binary.BigEndian.Uint32(data[0:4])
	if int(msgLen)+4 > len(data) {
		return fmt.Errorf("hsms: message length %d exceeds data size %d", msgLen, len(data)-4)
	}
	if msgLen < headerLen {
		return fmt.Errorf("hsms: message length %d too small for header", msgLen)
	}

	m.Header.SessionID = binary.BigEndian.Uint16(data[4:6])
	m.Header.WBit = data[6]&0x80 != 0
	m.Header.Stream = data[6] & 0x7F
	m.Header.Function = data[7]
	m.Header.PType = data[8]
	m.Header.SType = SType(data[9])
	m.Header.SystemID = binary.BigEndian.Uint32(data[10:14])

	dataLen := int(msgLen) - headerLen
	if dataLen > 0 {
		m.Data = make([]byte, dataLen)
		copy(m.Data, data[14:14+dataLen])
	}

	return nil
}

// --- Control message constructors ---

// NewSelectReq creates a Select.req message.
func NewSelectReq(sessionID uint16, systemID uint32) *Message {
	return &Message{
		Header: Header{
			SessionID: sessionID,
			SType:     STypeSelectReq,
			SystemID:  systemID,
		},
	}
}

// NewSelectRsp creates a Select.rsp message.
func NewSelectRsp(sessionID uint16, systemID uint32, status SelectStatus) *Message {
	return &Message{
		Header: Header{
			SessionID: sessionID,
			Stream:    byte(status), // Status in byte 2 (stream position)
			SType:     STypeSelectRsp,
			SystemID:  systemID,
		},
	}
}

// NewDeselectReq creates a Deselect.req message.
func NewDeselectReq(sessionID uint16, systemID uint32) *Message {
	return &Message{
		Header: Header{
			SessionID: sessionID,
			SType:     STypeDeselectReq,
			SystemID:  systemID,
		},
	}
}

// NewDeselectRsp creates a Deselect.rsp message.
func NewDeselectRsp(sessionID uint16, systemID uint32, status byte) *Message {
	return &Message{
		Header: Header{
			SessionID: sessionID,
			Stream:    status,
			SType:     STypeDeselectRsp,
			SystemID:  systemID,
		},
	}
}

// NewLinktestReq creates a Linktest.req message.
func NewLinktestReq(systemID uint32) *Message {
	return &Message{
		Header: Header{
			SessionID: 0xFFFF,
			SType:     STypeLinktestReq,
			SystemID:  systemID,
		},
	}
}

// NewLinktestRsp creates a Linktest.rsp message.
func NewLinktestRsp(systemID uint32) *Message {
	return &Message{
		Header: Header{
			SessionID: 0xFFFF,
			SType:     STypeLinktestRsp,
			SystemID:  systemID,
		},
	}
}

// NewSeparateReq creates a Separate.req message.
func NewSeparateReq(sessionID uint16, systemID uint32) *Message {
	return &Message{
		Header: Header{
			SessionID: sessionID,
			SType:     STypeSeparateReq,
			SystemID:  systemID,
		},
	}
}

// NewDataMessage creates a SECS-II data message.
func NewDataMessage(sessionID uint16, stream, function byte, wbit bool, systemID uint32, data []byte) *Message {
	return &Message{
		Header: Header{
			SessionID: sessionID,
			WBit:      wbit,
			Stream:    stream,
			Function:  function,
			SType:     STypeDataMessage,
			SystemID:  systemID,
		},
		Data: data,
	}
}
