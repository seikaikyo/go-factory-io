package modbus

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MBAP Header: TransactionID(2) + ProtocolID(2) + Length(2) + UnitID(1) = 7 bytes
const mbapHeaderLen = 7

// Function codes
const (
	fcReadCoils            byte = 0x01
	fcReadDiscreteInputs   byte = 0x02
	fcReadHoldingRegisters byte = 0x03
	fcReadInputRegisters   byte = 0x04
	fcWriteSingleCoil      byte = 0x05
	fcWriteSingleRegister  byte = 0x06
	fcWriteMultipleCoils   byte = 0x0F
	fcWriteMultipleRegs    byte = 0x10
)

// encodeMBAP creates an MBAP header.
func encodeMBAP(txnID uint16, pduLen int, unitID byte) []byte {
	buf := make([]byte, mbapHeaderLen)
	binary.BigEndian.PutUint16(buf[0:2], txnID)
	binary.BigEndian.PutUint16(buf[2:4], 0) // Protocol ID = 0 (Modbus)
	binary.BigEndian.PutUint16(buf[4:6], uint16(pduLen+1)) // Length = PDU + UnitID
	buf[6] = unitID
	return buf
}

// readResponse reads a full MBAP + PDU response from a reader.
func readResponse(r io.Reader) (txnID uint16, unitID byte, pdu []byte, err error) {
	header := make([]byte, mbapHeaderLen)
	if _, err = io.ReadFull(r, header); err != nil {
		return 0, 0, nil, fmt.Errorf("modbus: read MBAP header: %w", err)
	}

	txnID = binary.BigEndian.Uint16(header[0:2])
	protocolID := binary.BigEndian.Uint16(header[2:4])
	length := binary.BigEndian.Uint16(header[4:6])
	unitID = header[6]

	if protocolID != 0 {
		return 0, 0, nil, fmt.Errorf("modbus: invalid protocol ID: %d", protocolID)
	}
	if length < 1 || length > 260 {
		return 0, 0, nil, fmt.Errorf("modbus: invalid length: %d", length)
	}

	// Length includes UnitID (1 byte), remaining PDU = length - 1
	pduLen := int(length) - 1
	if pduLen <= 0 {
		return txnID, unitID, nil, nil
	}

	pdu = make([]byte, pduLen)
	if _, err = io.ReadFull(r, pdu); err != nil {
		return 0, 0, nil, fmt.Errorf("modbus: read PDU: %w", err)
	}

	return txnID, unitID, pdu, nil
}

// checkException checks if the PDU is an exception response.
func checkException(fc byte, pdu []byte) error {
	if len(pdu) < 1 {
		return fmt.Errorf("modbus: empty PDU")
	}
	if pdu[0]&0x80 != 0 {
		excCode := ExceptionCode(0)
		if len(pdu) > 1 {
			excCode = ExceptionCode(pdu[1])
		}
		return &ModbusError{FunctionCode: fc, ExceptionCode: excCode}
	}
	if pdu[0] != fc {
		return fmt.Errorf("modbus: unexpected function code 0x%02X, want 0x%02X", pdu[0], fc)
	}
	return nil
}

// decodeBools decodes a bit-packed byte slice into booleans.
func decodeBools(data []byte, count int) []bool {
	result := make([]bool, count)
	for i := 0; i < count; i++ {
		byteIdx := i / 8
		bitIdx := uint(i % 8)
		if byteIdx < len(data) {
			result[i] = data[byteIdx]&(1<<bitIdx) != 0
		}
	}
	return result
}

// decodeRegisters decodes big-endian uint16 register values.
func decodeRegisters(data []byte) []uint16 {
	count := len(data) / 2
	result := make([]uint16, count)
	for i := 0; i < count; i++ {
		result[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
	}
	return result
}

// encodeBools encodes booleans as bit-packed bytes.
func encodeBools(values []bool) []byte {
	byteCount := (len(values) + 7) / 8
	data := make([]byte, byteCount)
	for i, v := range values {
		if v {
			data[i/8] |= 1 << uint(i%8)
		}
	}
	return data
}

// encodeRegisters encodes uint16 values as big-endian bytes.
func encodeRegisters(values []uint16) []byte {
	data := make([]byte, len(values)*2)
	for i, v := range values {
		binary.BigEndian.PutUint16(data[i*2:], v)
	}
	return data
}
