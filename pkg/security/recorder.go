package security

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Direction indicates message flow for recording.
type Direction string

const (
	DirInbound  Direction = "IN"
	DirOutbound Direction = "OUT"
)

// MessageRecord is a single recorded SECS/HSMS message for forensic analysis.
// Covers NIST AU (Audit and Accountability).
type MessageRecord struct {
	Time      time.Time
	Direction Direction
	Stream    byte
	Function  byte
	SessionID uint16
	SystemID  uint32
	DataLen   int
	RawHex    string // First 256 bytes hex-encoded
}

func (r MessageRecord) String() string {
	return fmt.Sprintf("%s %s S%dF%d sess=%d sys=%d len=%d data=%s",
		r.Time.Format("2006-01-02T15:04:05.000Z07:00"),
		r.Direction,
		r.Stream, r.Function,
		r.SessionID, r.SystemID,
		r.DataLen, r.RawHex,
	)
}

// MessageRecorder records SECS/HSMS messages for forensic analysis.
// Thread-safe, writes to any io.Writer (file, buffer, etc.).
type MessageRecorder struct {
	mu      sync.Mutex
	writer  io.Writer
	maxHex  int  // Max bytes to hex-encode per message
	enabled bool
}

// NewMessageRecorder creates a recorder writing to the given writer.
// maxHexBytes limits the raw data hex dump per message (default 256).
func NewMessageRecorder(writer io.Writer, maxHexBytes int) *MessageRecorder {
	if maxHexBytes <= 0 {
		maxHexBytes = 256
	}
	return &MessageRecorder{
		writer:  writer,
		maxHex:  maxHexBytes,
		enabled: true,
	}
}

// NewFileRecorder creates a recorder that appends to a file.
func NewFileRecorder(path string) (*MessageRecorder, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("security: open recorder file: %w", err)
	}
	return NewMessageRecorder(f, 256), nil
}

// Record writes a message record.
func (mr *MessageRecorder) Record(dir Direction, stream, function byte, sessionID uint16, systemID uint32, data []byte) {
	if !mr.enabled {
		return
	}

	hexData := data
	if len(hexData) > mr.maxHex {
		hexData = hexData[:mr.maxHex]
	}

	record := MessageRecord{
		Time:      time.Now(),
		Direction: dir,
		Stream:    stream,
		Function:  function,
		SessionID: sessionID,
		SystemID:  systemID,
		DataLen:   len(data),
		RawHex:    hex.EncodeToString(hexData),
	}

	mr.mu.Lock()
	fmt.Fprintln(mr.writer, record.String())
	mr.mu.Unlock()
}

// SetEnabled toggles recording on/off.
func (mr *MessageRecorder) SetEnabled(enabled bool) {
	mr.mu.Lock()
	mr.enabled = enabled
	mr.mu.Unlock()
}

// IsEnabled returns whether recording is active.
func (mr *MessageRecorder) IsEnabled() bool {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	return mr.enabled
}
