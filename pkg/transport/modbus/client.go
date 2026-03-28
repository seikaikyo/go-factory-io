package modbus

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// Config holds Modbus TCP client configuration.
type Config struct {
	Address   string        // "192.168.1.100:502"
	UnitID    byte          // Modbus slave address (default 1)
	Timeout   time.Duration // Per-request timeout (default 5s)
	TLSConfig *tls.Config   // Optional TLS
}

// Client is a Modbus TCP client.
// All operations are serialized via mutex because most Modbus devices
// cannot handle concurrent requests on a single TCP connection.
type Client struct {
	config Config
	logger *slog.Logger
	mu     sync.Mutex
	conn   net.Conn
	txnID  uint16
}

// NewClient creates a Modbus TCP client.
func NewClient(cfg Config, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.UnitID == 0 {
		cfg.UnitID = 1
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &Client{
		config: cfg,
		logger: logger,
	}
}

// Connect establishes the TCP connection.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var d net.Dialer
	d.Deadline = time.Now().Add(c.config.Timeout)

	var conn net.Conn
	var err error

	if c.config.TLSConfig != nil {
		conn, err = tls.DialWithDialer(&d, "tcp", c.config.Address, c.config.TLSConfig)
	} else {
		conn, err = d.DialContext(ctx, "tcp", c.config.Address)
	}
	if err != nil {
		return fmt.Errorf("modbus: connect %s: %w", c.config.Address, err)
	}

	c.conn = conn
	c.logger.Info("Modbus connected", "address", c.config.Address)
	return nil
}

// Close closes the TCP connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// ReadCoils reads coil status (FC01).
func (c *Client) ReadCoils(ctx context.Context, addr, quantity uint16) ([]bool, error) {
	pdu := make([]byte, 5)
	pdu[0] = fcReadCoils
	binary.BigEndian.PutUint16(pdu[1:3], addr)
	binary.BigEndian.PutUint16(pdu[3:5], quantity)

	resp, err := c.sendRequest(ctx, pdu)
	if err != nil {
		return nil, err
	}
	if err := checkException(fcReadCoils, resp); err != nil {
		return nil, err
	}
	if len(resp) < 2 {
		return nil, fmt.Errorf("modbus: FC01 response too short")
	}
	byteCount := int(resp[1])
	if len(resp) < 2+byteCount {
		return nil, fmt.Errorf("modbus: FC01 response data too short")
	}
	return decodeBools(resp[2:2+byteCount], int(quantity)), nil
}

// ReadDiscreteInputs reads discrete input status (FC02).
func (c *Client) ReadDiscreteInputs(ctx context.Context, addr, quantity uint16) ([]bool, error) {
	pdu := make([]byte, 5)
	pdu[0] = fcReadDiscreteInputs
	binary.BigEndian.PutUint16(pdu[1:3], addr)
	binary.BigEndian.PutUint16(pdu[3:5], quantity)

	resp, err := c.sendRequest(ctx, pdu)
	if err != nil {
		return nil, err
	}
	if err := checkException(fcReadDiscreteInputs, resp); err != nil {
		return nil, err
	}
	if len(resp) < 2 {
		return nil, fmt.Errorf("modbus: FC02 response too short")
	}
	byteCount := int(resp[1])
	if len(resp) < 2+byteCount {
		return nil, fmt.Errorf("modbus: FC02 response data too short")
	}
	return decodeBools(resp[2:2+byteCount], int(quantity)), nil
}

// ReadHoldingRegisters reads holding registers (FC03).
func (c *Client) ReadHoldingRegisters(ctx context.Context, addr, quantity uint16) ([]uint16, error) {
	pdu := make([]byte, 5)
	pdu[0] = fcReadHoldingRegisters
	binary.BigEndian.PutUint16(pdu[1:3], addr)
	binary.BigEndian.PutUint16(pdu[3:5], quantity)

	resp, err := c.sendRequest(ctx, pdu)
	if err != nil {
		return nil, err
	}
	if err := checkException(fcReadHoldingRegisters, resp); err != nil {
		return nil, err
	}
	if len(resp) < 2 {
		return nil, fmt.Errorf("modbus: FC03 response too short")
	}
	byteCount := int(resp[1])
	if len(resp) < 2+byteCount {
		return nil, fmt.Errorf("modbus: FC03 response data too short")
	}
	return decodeRegisters(resp[2 : 2+byteCount]), nil
}

// ReadInputRegisters reads input registers (FC04).
func (c *Client) ReadInputRegisters(ctx context.Context, addr, quantity uint16) ([]uint16, error) {
	pdu := make([]byte, 5)
	pdu[0] = fcReadInputRegisters
	binary.BigEndian.PutUint16(pdu[1:3], addr)
	binary.BigEndian.PutUint16(pdu[3:5], quantity)

	resp, err := c.sendRequest(ctx, pdu)
	if err != nil {
		return nil, err
	}
	if err := checkException(fcReadInputRegisters, resp); err != nil {
		return nil, err
	}
	if len(resp) < 2 {
		return nil, fmt.Errorf("modbus: FC04 response too short")
	}
	byteCount := int(resp[1])
	if len(resp) < 2+byteCount {
		return nil, fmt.Errorf("modbus: FC04 response data too short")
	}
	return decodeRegisters(resp[2 : 2+byteCount]), nil
}

// WriteSingleCoil writes a single coil (FC05).
func (c *Client) WriteSingleCoil(ctx context.Context, addr uint16, value bool) error {
	pdu := make([]byte, 5)
	pdu[0] = fcWriteSingleCoil
	binary.BigEndian.PutUint16(pdu[1:3], addr)
	if value {
		pdu[3] = 0xFF
		pdu[4] = 0x00
	}

	resp, err := c.sendRequest(ctx, pdu)
	if err != nil {
		return err
	}
	return checkException(fcWriteSingleCoil, resp)
}

// WriteSingleRegister writes a single holding register (FC06).
func (c *Client) WriteSingleRegister(ctx context.Context, addr, value uint16) error {
	pdu := make([]byte, 5)
	pdu[0] = fcWriteSingleRegister
	binary.BigEndian.PutUint16(pdu[1:3], addr)
	binary.BigEndian.PutUint16(pdu[3:5], value)

	resp, err := c.sendRequest(ctx, pdu)
	if err != nil {
		return err
	}
	return checkException(fcWriteSingleRegister, resp)
}

// WriteMultipleCoils writes multiple coils (FC15).
func (c *Client) WriteMultipleCoils(ctx context.Context, addr uint16, values []bool) error {
	data := encodeBools(values)
	pdu := make([]byte, 6+len(data))
	pdu[0] = fcWriteMultipleCoils
	binary.BigEndian.PutUint16(pdu[1:3], addr)
	binary.BigEndian.PutUint16(pdu[3:5], uint16(len(values)))
	pdu[5] = byte(len(data))
	copy(pdu[6:], data)

	resp, err := c.sendRequest(ctx, pdu)
	if err != nil {
		return err
	}
	return checkException(fcWriteMultipleCoils, resp)
}

// WriteMultipleRegisters writes multiple holding registers (FC16).
func (c *Client) WriteMultipleRegisters(ctx context.Context, addr uint16, values []uint16) error {
	data := encodeRegisters(values)
	pdu := make([]byte, 6+len(data))
	pdu[0] = fcWriteMultipleRegs
	binary.BigEndian.PutUint16(pdu[1:3], addr)
	binary.BigEndian.PutUint16(pdu[3:5], uint16(len(values)))
	pdu[5] = byte(len(data))
	copy(pdu[6:], data)

	resp, err := c.sendRequest(ctx, pdu)
	if err != nil {
		return err
	}
	return checkException(fcWriteMultipleRegs, resp)
}

// sendRequest sends a Modbus PDU and returns the response PDU.
func (c *Client) sendRequest(ctx context.Context, pdu []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, fmt.Errorf("modbus: not connected")
	}

	c.txnID++
	txnID := c.txnID

	// Set deadline from context or config timeout
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(c.config.Timeout)
	}
	c.conn.SetDeadline(deadline)

	// Write MBAP + PDU
	header := encodeMBAP(txnID, len(pdu), c.config.UnitID)
	frame := append(header, pdu...)
	if _, err := c.conn.Write(frame); err != nil {
		return nil, fmt.Errorf("modbus: write: %w", err)
	}

	// Read response
	respTxnID, _, respPDU, err := readResponse(c.conn)
	if err != nil {
		return nil, err
	}
	if respTxnID != txnID {
		return nil, fmt.Errorf("modbus: transaction ID mismatch: got %d, want %d", respTxnID, txnID)
	}

	return respPDU, nil
}
