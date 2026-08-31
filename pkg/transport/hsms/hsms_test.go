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

// --- Select precondition and T7 ---

// setupConnectedUnselectedPair creates an active+passive pair whose TCP
// handshake is complete but which never performed Select.
func setupConnectedUnselectedPair(t *testing.T, logger *slog.Logger, t7 time.Duration) (*Session, *Session) {
	t.Helper()

	passiveCfg := DefaultConfig("127.0.0.1:0", RolePassive, 0x0001)
	passiveCfg.LinktestInterval = 0
	passiveCfg.T7 = t7
	passive := NewSession(passiveCfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}

	activeCfg := DefaultConfig(passive.Addr().String(), RoleActive, 0x0001)
	activeCfg.LinktestInterval = 0
	activeCfg.T7 = 0 // Only the passive side is under test.
	active := NewSession(activeCfg, logger)

	if err := active.Connect(ctx); err != nil {
		passive.Close()
		t.Fatalf("active connect: %v", err)
	}

	// Let the passive side finish accepting.
	time.Sleep(50 * time.Millisecond)
	return active, passive
}

// TestDataMessageBeforeSelect covers the SEMI E37 precondition: a peer that
// completes the TCP handshake but skips Select must not be able to deliver
// data messages to the application.
func TestDataMessageBeforeSelect(t *testing.T) {
	logger := slog.Default()
	active, passive := setupConnectedUnselectedPair(t, logger, 0)
	defer active.Close()
	defer passive.Close()

	if got := passive.State(); got == transport.StateSelected {
		t.Fatalf("fixture is wrong: passive is already %s", got)
	}

	tests := []struct {
		name             string
		stream, function byte
	}{
		{"S1F1 are you there", 1, 1},
		{"S2F41 remote command", 2, 41},
		{"S1F17 request online", 1, 17},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := NewDataMessage(0x0001, tc.stream, tc.function, true, 0, []byte{0x01, 0x00})
			msg.Header.SystemID = 4000 + uint32(tc.function)
			if err := active.writeMessage(msg); err != nil {
				t.Fatalf("send: %v", err)
			}

			// The message must never reach the application inbound queue.
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			received, err := passive.ReceiveMessage(ctx)
			if err == nil {
				t.Fatalf("unselected data message was delivered: S%dF%d",
					received.Header.Stream, received.Header.Function)
			}
		})
	}
}

// TestDataMessageBeforeSelectReplies checks that the rejection is a Reject.req
// carrying the "entity not selected" reason, not a silent drop.
func TestDataMessageBeforeSelectReplies(t *testing.T) {
	logger := slog.Default()
	active, passive := setupConnectedUnselectedPair(t, logger, 0)
	defer active.Close()
	defer passive.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msg := NewDataMessage(0x0001, 2, 41, true, 0, []byte{0x01, 0x00})
	msg.Header.SystemID = 7777

	rsp, err := active.sendAndWait(ctx, msg, 2*time.Second)
	if err != nil {
		t.Fatalf("expected a Reject.req, got error: %v", err)
	}
	if rsp.Header.SType != STypeRejectReq {
		t.Errorf("SType: got %s, want Reject.req", rsp.Header.SType)
	}
	if rsp.Header.Stream != byte(STypeDataMessage) {
		t.Errorf("offending SType byte: got %d, want %d", rsp.Header.Stream, byte(STypeDataMessage))
	}
	if rsp.Header.Function != RejectReasonEntityNotSelected {
		t.Errorf("reason: got %d, want %d (entity not selected)",
			rsp.Header.Function, RejectReasonEntityNotSelected)
	}
}

// TestDataMessageAfterSelectAccepted guards against the precondition being so
// strict that normal traffic stops flowing.
func TestDataMessageAfterSelectAccepted(t *testing.T) {
	logger := slog.Default()
	active, passive := setupConnectedPair(t, logger)
	defer active.Close()
	defer passive.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msg := NewDataMessage(0x0001, 1, 1, true, 0, []byte{0x01, 0x00})
	msg.Header.SystemID = 8888
	if err := active.writeMessage(msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	received, err := passive.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("selected data message should be delivered: %v", err)
	}
	if received.Header.Stream != 1 || received.Header.Function != 1 {
		t.Errorf("got S%dF%d, want S1F1", received.Header.Stream, received.Header.Function)
	}
}

// TestT7NotSelectedTimeout covers the T7 not-selected timeout, which
// Config documented but never enforced.
func TestT7NotSelectedTimeout(t *testing.T) {
	logger := slog.Default()
	active, passive := setupConnectedUnselectedPair(t, logger, 200*time.Millisecond)
	defer active.Close()
	defer passive.Close()

	select {
	case <-passive.Done():
	case <-time.After(3 * time.Second):
		t.Fatalf("T7 did not close the never-selected connection (state=%s)", passive.State())
	}

	if got := passive.State(); got != transport.StateDisconnected {
		t.Errorf("state after T7: got %s, want Disconnected", got)
	}
}

// TestT7DoesNotCloseSelectedSession checks the timer stands down once Select
// completes, so a healthy session is not dropped.
func TestT7DoesNotCloseSelectedSession(t *testing.T) {
	logger := slog.Default()

	passiveCfg := DefaultConfig("127.0.0.1:0", RolePassive, 0x0001)
	passiveCfg.LinktestInterval = 0
	passiveCfg.T7 = 200 * time.Millisecond
	passive := NewSession(passiveCfg, logger)
	defer passive.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}

	activeCfg := DefaultConfig(passive.Addr().String(), RoleActive, 0x0001)
	activeCfg.LinktestInterval = 0
	activeCfg.T7 = 0
	active := NewSession(activeCfg, logger)
	defer active.Close()

	if err := active.Connect(ctx); err != nil {
		t.Fatalf("active connect: %v", err)
	}
	if err := active.Select(ctx); err != nil {
		t.Fatalf("select: %v", err)
	}

	time.Sleep(500 * time.Millisecond) // Well past T7.

	if got := passive.State(); got != transport.StateSelected {
		t.Errorf("selected session was closed by T7: state=%s", got)
	}
}

// TestT7DisabledWhenZero documents that T7 <= 0 turns the timeout off.
func TestT7DisabledWhenZero(t *testing.T) {
	logger := slog.Default()
	active, passive := setupConnectedUnselectedPair(t, logger, 0)
	defer active.Close()
	defer passive.Close()

	time.Sleep(300 * time.Millisecond)

	if got := passive.State(); got == transport.StateDisconnected {
		t.Error("T7=0 should leave the connection open")
	}
}
