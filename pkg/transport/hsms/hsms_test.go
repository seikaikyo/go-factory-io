package hsms

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/transport"
)

// --- Message marshal/unmarshal tests ---

func TestMessageMarshalRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		msg  *Message
	}{
		{
			"SelectReq",
			NewSelectReq(0x0001, 1),
		},
		{
			"SelectRsp",
			NewSelectRsp(0x0001, 1, SelectStatusSuccess),
		},
		{
			"LinktestReq",
			NewLinktestReq(42),
		},
		{
			"LinktestRsp",
			NewLinktestRsp(42),
		},
		{
			"DataMessage",
			NewDataMessage(0x0001, 1, 13, true, 100, []byte{0x01, 0x01, 0x41, 0x01, 0x00}),
		},
		{
			"EmptyDataMessage",
			NewDataMessage(0x0001, 1, 2, false, 200, nil),
		},
		{
			"SeparateReq",
			NewSeparateReq(0x0001, 5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.msg.MarshalBinary()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			got := &Message{}
			if err := got.UnmarshalBinary(data); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			// Compare headers
			if got.Header.SessionID != tt.msg.Header.SessionID {
				t.Errorf("SessionID: got %d, want %d", got.Header.SessionID, tt.msg.Header.SessionID)
			}
			if got.Header.SType != tt.msg.Header.SType {
				t.Errorf("SType: got %s, want %s", got.Header.SType, tt.msg.Header.SType)
			}
			if got.Header.SystemID != tt.msg.Header.SystemID {
				t.Errorf("SystemID: got %d, want %d", got.Header.SystemID, tt.msg.Header.SystemID)
			}
			if got.Header.WBit != tt.msg.Header.WBit {
				t.Errorf("WBit: got %v, want %v", got.Header.WBit, tt.msg.Header.WBit)
			}
			if got.Header.Stream != tt.msg.Header.Stream {
				t.Errorf("Stream: got %d, want %d", got.Header.Stream, tt.msg.Header.Stream)
			}
			if got.Header.Function != tt.msg.Header.Function {
				t.Errorf("Function: got %d, want %d", got.Header.Function, tt.msg.Header.Function)
			}

			// Compare data
			if !bytes.Equal(got.Data, tt.msg.Data) {
				t.Errorf("Data: got %x, want %x", got.Data, tt.msg.Data)
			}
		})
	}
}

func TestMessageMarshalWireFormat(t *testing.T) {
	// Verify exact wire format for a Select.req
	msg := NewSelectReq(0x0001, 1)
	data, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Total: 14 bytes (4 length + 10 header + 0 data)
	if len(data) != 14 {
		t.Fatalf("expected 14 bytes, got %d", len(data))
	}

	// Length field: 10 (header only, no data)
	msgLen := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	if msgLen != 10 {
		t.Errorf("message length: got %d, want 10", msgLen)
	}

	// SType at byte 9
	if SType(data[9]) != STypeSelectReq {
		t.Errorf("SType: got %d, want %d", data[9], STypeSelectReq)
	}
}

func TestUnmarshalTooShort(t *testing.T) {
	msg := &Message{}
	err := msg.UnmarshalBinary([]byte{0x00, 0x00})
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestUnmarshalInvalidLength(t *testing.T) {
	// Length says 100 but only 10 bytes of header present
	data := []byte{0x00, 0x00, 0x00, 0x64, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	msg := &Message{}
	err := msg.UnmarshalBinary(data)
	if err == nil {
		t.Fatal("expected error for invalid length")
	}
}

// --- Session integration tests (loopback) ---

func TestSessionSelectActive(t *testing.T) {
	logger := slog.Default()

	// Start passive side
	passiveCfg := DefaultConfig("127.0.0.1:0", RolePassive, 0x0001)
	passiveCfg.LinktestInterval = 0 // disable for test
	passive := NewSession(passiveCfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}
	defer passive.Close()

	// Get the actual port
	addr := passive.Addr().String()

	// Start active side
	activeCfg := DefaultConfig(addr, RoleActive, 0x0001)
	activeCfg.LinktestInterval = 0
	active := NewSession(activeCfg, logger)

	if err := active.Connect(ctx); err != nil {
		t.Fatalf("active connect: %v", err)
	}
	defer active.Close()

	// Active sends Select.req
	if err := active.Select(ctx); err != nil {
		t.Fatalf("select: %v", err)
	}

	if active.State() != transport.StateSelected {
		t.Errorf("active state: got %s, want Selected", active.State())
	}

	// Give passive side time to process
	time.Sleep(50 * time.Millisecond)
	if passive.State() != transport.StateSelected {
		t.Errorf("passive state: got %s, want Selected", passive.State())
	}
}

func TestSessionDataExchange(t *testing.T) {
	logger := slog.Default()
	active, passive := setupConnectedPair(t, logger)
	defer active.Close()
	defer passive.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Active sends S1F1 (Are You There)
	secsData := []byte{0x01, 0x01, 0x00} // Minimal SECS-II: empty list
	msg := NewDataMessage(0x0001, 1, 1, true, 0, secsData)
	msg.Header.SystemID = 100

	if err := active.writeMessage(msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Passive receives
	received, err := passive.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}

	if received.Header.Stream != 1 || received.Header.Function != 1 {
		t.Errorf("S%dF%d, want S1F1", received.Header.Stream, received.Header.Function)
	}
	if !bytes.Equal(received.Data, secsData) {
		t.Errorf("data: got %x, want %x", received.Data, secsData)
	}
}

func TestSessionLinktest(t *testing.T) {
	logger := slog.Default()
	active, passive := setupConnectedPair(t, logger)
	defer active.Close()
	defer passive.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Manually send linktest
	systemID := active.nextSystemID.Add(1)
	req := NewLinktestReq(systemID)
	rsp, err := active.sendAndWait(ctx, req, active.config.T6)
	if err != nil {
		t.Fatalf("linktest: %v", err)
	}

	if rsp.Header.SType != STypeLinktestRsp {
		t.Errorf("response SType: got %s, want Linktest.rsp", rsp.Header.SType)
	}
}

// setupConnectedPair creates an active+passive pair that are already Selected.
func setupConnectedPair(t *testing.T, logger *slog.Logger) (*Session, *Session) {
	t.Helper()

	passiveCfg := DefaultConfig("127.0.0.1:0", RolePassive, 0x0001)
	passiveCfg.LinktestInterval = 0
	passive := NewSession(passiveCfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}

	addr := passive.Addr().String()

	activeCfg := DefaultConfig(addr, RoleActive, 0x0001)
	activeCfg.LinktestInterval = 0
	active := NewSession(activeCfg, logger)

	if err := active.Connect(ctx); err != nil {
		passive.Close()
		t.Fatalf("active connect: %v", err)
	}

	if err := active.Select(ctx); err != nil {
		active.Close()
		passive.Close()
		t.Fatalf("select: %v", err)
	}

	// Let passive side process the Select
	time.Sleep(50 * time.Millisecond)

	return active, passive
}

// --- Benchmarks ---

func BenchmarkMessageMarshal(b *testing.B) {
	msg := NewDataMessage(0x0001, 6, 11, false, 12345, make([]byte, 100))
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		_, _ = msg.MarshalBinary()
	}
}

func BenchmarkMessageUnmarshal(b *testing.B) {
	msg := NewDataMessage(0x0001, 6, 11, false, 12345, make([]byte, 100))
	data, _ := msg.MarshalBinary()
	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		m := &Message{}
		_ = m.UnmarshalBinary(data)
	}
}
