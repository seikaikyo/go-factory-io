package simulator

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"

	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

// FaultType enumerates injectable faults.
type FaultType string

const (
	FaultDisconnect    FaultType = "disconnect"     // Drop TCP connection
	FaultCorruptData   FaultType = "corrupt_data"    // Send garbled bytes
	FaultBadSType      FaultType = "bad_stype"       // Send invalid SType
	FaultBadLength     FaultType = "bad_length"       // Send incorrect length header
	FaultDupSystemByte FaultType = "dup_systembyte"   // Reuse system byte
)

// AllFaultTypes returns all supported fault types.
func AllFaultTypes() []FaultType {
	return []FaultType{
		FaultDisconnect,
		FaultCorruptData,
		FaultBadSType,
		FaultBadLength,
		FaultDupSystemByte,
	}
}

// FaultInjector introduces controlled failures into a session.
type FaultInjector struct {
	session *hsms.Session
	logger  *slog.Logger
}

// NewFaultInjector creates a fault injector for the given session.
func NewFaultInjector(session *hsms.Session, logger *slog.Logger) *FaultInjector {
	return &FaultInjector{session: session, logger: logger}
}

// Inject performs the specified fault. Some faults use the rawConn directly.
func (fi *FaultInjector) Inject(fault FaultType, rawConn net.Conn) error {
	fi.logger.Info("Injecting fault", "type", fault)

	switch fault {
	case FaultDisconnect:
		return fi.injectDisconnect()
	case FaultCorruptData:
		return fi.injectCorruptData(rawConn)
	case FaultBadSType:
		return fi.injectBadSType(rawConn)
	case FaultBadLength:
		return fi.injectBadLength(rawConn)
	case FaultDupSystemByte:
		return fi.injectDupSystemByte(rawConn)
	default:
		return fmt.Errorf("unknown fault type: %s", fault)
	}
}

func (fi *FaultInjector) injectDisconnect() error {
	return fi.session.Close()
}

func (fi *FaultInjector) injectCorruptData(conn net.Conn) error {
	if conn == nil {
		return fmt.Errorf("no raw connection available")
	}
	// Send 64 random bytes
	garbage := make([]byte, 64)
	for i := range garbage {
		garbage[i] = byte(rand.IntN(256))
	}
	_, err := conn.Write(garbage)
	return err
}

func (fi *FaultInjector) injectBadSType(conn net.Conn) error {
	if conn == nil {
		return fmt.Errorf("no raw connection available")
	}
	// Valid HSMS header with invalid SType (0xFF)
	// Length = 10 (header only), then 10-byte header with bad SType
	msg := []byte{
		0x00, 0x00, 0x00, 0x0A, // length: 10
		0x00, 0x01, // session ID
		0x00, 0x00, // stream/function (don't care)
		0xFF, 0x00, // SType = 0xFF (invalid), PType = 0
		0x00, 0x00, 0x00, 0x01, // system ID
	}
	_, err := conn.Write(msg)
	return err
}

func (fi *FaultInjector) injectBadLength(conn net.Conn) error {
	if conn == nil {
		return fmt.Errorf("no raw connection available")
	}
	// Claim length of 1000 bytes but only send 10
	msg := []byte{
		0x00, 0x00, 0x03, 0xE8, // length: 1000
		0x00, 0x01, // session ID
		0x01, 0x01, // S1F1
		0x00, 0x00, // SType/PType
		0x00, 0x00, 0x00, 0x01, // system ID
	}
	_, err := conn.Write(msg)
	return err
}

func (fi *FaultInjector) injectDupSystemByte(conn net.Conn) error {
	if conn == nil {
		return fmt.Errorf("no raw connection available")
	}
	// Send two S1F1 requests with the same system byte
	msg := []byte{
		0x00, 0x00, 0x00, 0x0A, // length: 10
		0x00, 0x01, // session ID
		0x81, 0x01, // S1F1 W-bit
		0x00, 0x00, // data message
		0xDE, 0xAD, 0xBE, 0xEF, // system byte (same for both)
	}
	if _, err := conn.Write(msg); err != nil {
		return err
	}
	_, err := conn.Write(msg)
	return err
}
