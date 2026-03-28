package modbus

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// mockModbusServer is a simple Modbus TCP server for testing.
type mockModbusServer struct {
	listener net.Listener
	handler  func(fc byte, data []byte) ([]byte, error)
}

func newMockServer(t *testing.T, handler func(fc byte, data []byte) ([]byte, error)) *mockModbusServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	s := &mockModbusServer{listener: ln, handler: handler}
	go s.serve(t)
	return s
}

func (s *mockModbusServer) serve(t *testing.T) {
	t.Helper()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(t, conn)
	}
}

func (s *mockModbusServer) handleConn(t *testing.T, conn net.Conn) {
	t.Helper()
	defer conn.Close()
	for {
		txnID, unitID, pdu, err := readResponse(conn)
		if err != nil {
			return
		}
		if len(pdu) == 0 {
			return
		}

		fc := pdu[0]
		respPDU, err := s.handler(fc, pdu[1:])
		if err != nil {
			// Send exception
			excPDU := []byte{fc | 0x80, 0x02} // Illegal data address
			header := encodeMBAP(txnID, len(excPDU), unitID)
			conn.Write(append(header, excPDU...))
			continue
		}

		fullPDU := append([]byte{fc}, respPDU...)
		header := encodeMBAP(txnID, len(fullPDU), unitID)
		conn.Write(append(header, fullPDU...))
	}
}

func (s *mockModbusServer) addr() string {
	return s.listener.Addr().String()
}

func (s *mockModbusServer) close() {
	s.listener.Close()
}

func TestReadHoldingRegisters(t *testing.T) {
	srv := newMockServer(t, func(fc byte, data []byte) ([]byte, error) {
		if fc != fcReadHoldingRegisters {
			t.Errorf("fc = 0x%02X, want 0x03", fc)
		}
		// Return 2 registers: 100, 200
		resp := []byte{4} // byte count
		resp = append(resp, 0, 100, 0, 200)
		return resp, nil
	})
	defer srv.close()

	client := NewClient(Config{Address: srv.addr(), Timeout: 2 * time.Second}, nil)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	regs, err := client.ReadHoldingRegisters(context.Background(), 0, 2)
	if err != nil {
		t.Fatalf("ReadHoldingRegisters: %v", err)
	}
	if len(regs) != 2 {
		t.Fatalf("got %d registers, want 2", len(regs))
	}
	if regs[0] != 100 || regs[1] != 200 {
		t.Errorf("registers = %v, want [100 200]", regs)
	}
}

func TestReadCoils(t *testing.T) {
	srv := newMockServer(t, func(fc byte, data []byte) ([]byte, error) {
		// Return 8 coils: 10101010
		return []byte{1, 0xAA}, nil
	})
	defer srv.close()

	client := NewClient(Config{Address: srv.addr(), Timeout: 2 * time.Second}, nil)
	client.Connect(context.Background())
	defer client.Close()

	coils, err := client.ReadCoils(context.Background(), 0, 8)
	if err != nil {
		t.Fatalf("ReadCoils: %v", err)
	}
	if len(coils) != 8 {
		t.Fatalf("got %d coils, want 8", len(coils))
	}
	// 0xAA = 10101010 -> bit0=0, bit1=1, bit2=0, bit3=1, ...
	expected := []bool{false, true, false, true, false, true, false, true}
	for i, v := range coils {
		if v != expected[i] {
			t.Errorf("coil[%d] = %v, want %v", i, v, expected[i])
		}
	}
}

func TestWriteSingleRegister(t *testing.T) {
	var gotAddr, gotValue uint16

	srv := newMockServer(t, func(fc byte, data []byte) ([]byte, error) {
		if fc != fcWriteSingleRegister {
			t.Errorf("fc = 0x%02X, want 0x06", fc)
		}
		gotAddr = binary.BigEndian.Uint16(data[0:2])
		gotValue = binary.BigEndian.Uint16(data[2:4])
		return data, nil // Echo back
	})
	defer srv.close()

	client := NewClient(Config{Address: srv.addr(), Timeout: 2 * time.Second}, nil)
	client.Connect(context.Background())
	defer client.Close()

	err := client.WriteSingleRegister(context.Background(), 10, 42)
	if err != nil {
		t.Fatalf("WriteSingleRegister: %v", err)
	}
	if gotAddr != 10 {
		t.Errorf("addr = %d, want 10", gotAddr)
	}
	if gotValue != 42 {
		t.Errorf("value = %d, want 42", gotValue)
	}
}

func TestWriteSingleCoil(t *testing.T) {
	srv := newMockServer(t, func(fc byte, data []byte) ([]byte, error) {
		// data = addr(2) + value(2), check value bytes at offset 2-3
		if len(data) < 4 {
			t.Fatalf("data too short: %d", len(data))
		}
		if data[2] != 0xFF || data[3] != 0x00 {
			t.Errorf("coil value = 0x%02X%02X, want 0xFF00", data[2], data[3])
		}
		return data, nil
	})
	defer srv.close()

	client := NewClient(Config{Address: srv.addr(), Timeout: 2 * time.Second}, nil)
	client.Connect(context.Background())
	defer client.Close()

	if err := client.WriteSingleCoil(context.Background(), 5, true); err != nil {
		t.Fatalf("WriteSingleCoil: %v", err)
	}
}

func TestExceptionResponse(t *testing.T) {
	srv := newMockServer(t, func(fc byte, data []byte) ([]byte, error) {
		return nil, &ModbusError{FunctionCode: fc, ExceptionCode: ExcIllegalDataAddress}
	})
	defer srv.close()

	client := NewClient(Config{Address: srv.addr(), Timeout: 2 * time.Second}, nil)
	client.Connect(context.Background())
	defer client.Close()

	_, err := client.ReadHoldingRegisters(context.Background(), 9999, 1)
	if err == nil {
		t.Fatal("expected error for exception response")
	}
	me, ok := err.(*ModbusError)
	if !ok {
		t.Fatalf("error type = %T, want *ModbusError", err)
	}
	if me.ExceptionCode != ExcIllegalDataAddress {
		t.Errorf("exception = %v, want ExcIllegalDataAddress", me.ExceptionCode)
	}
}

func TestWriteMultipleRegisters(t *testing.T) {
	srv := newMockServer(t, func(fc byte, data []byte) ([]byte, error) {
		// Echo addr + quantity
		return data[:4], nil
	})
	defer srv.close()

	client := NewClient(Config{Address: srv.addr(), Timeout: 2 * time.Second}, nil)
	client.Connect(context.Background())
	defer client.Close()

	err := client.WriteMultipleRegisters(context.Background(), 0, []uint16{100, 200, 300})
	if err != nil {
		t.Fatalf("WriteMultipleRegisters: %v", err)
	}
}

func TestNotConnected(t *testing.T) {
	client := NewClient(Config{Address: "127.0.0.1:0"}, nil)
	_, err := client.ReadHoldingRegisters(context.Background(), 0, 1)
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestProtocolEncodeDecode(t *testing.T) {
	// Test bool encoding/decoding
	values := []bool{true, false, true, true, false, false, true, false, true}
	encoded := encodeBools(values)
	decoded := decodeBools(encoded, len(values))
	for i, v := range values {
		if decoded[i] != v {
			t.Errorf("bool[%d] = %v, want %v", i, decoded[i], v)
		}
	}

	// Test register encoding/decoding
	regs := []uint16{0, 1, 255, 1024, 65535}
	regBytes := encodeRegisters(regs)
	decodedRegs := decodeRegisters(regBytes)
	for i, v := range regs {
		if decodedRegs[i] != v {
			t.Errorf("reg[%d] = %d, want %d", i, decodedRegs[i], v)
		}
	}
}

func TestExceptionCodeString(t *testing.T) {
	tests := []struct {
		code ExceptionCode
		want string
	}{
		{ExcIllegalFunction, "Illegal Function"},
		{ExcIllegalDataAddress, "Illegal Data Address"},
		{ExcSlaveDeviceFailure, "Slave Device Failure"},
		{ExceptionCode(0xFF), "Unknown Exception (0xFF)"},
	}
	for _, tt := range tests {
		if got := tt.code.String(); got != tt.want {
			t.Errorf("ExceptionCode(%d).String() = %s, want %s", tt.code, got, tt.want)
		}
	}
}
