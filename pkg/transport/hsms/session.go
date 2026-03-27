package hsms

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/transport"
)

// Session manages a single HSMS TCP connection, handling the HSMS state machine,
// control messages, and data message routing.
type Session struct {
	config   Config
	logger   *slog.Logger
	state    atomic.Int32 // transport.State
	conn     net.Conn
	listener net.Listener

	// systemID counter for generating unique transaction IDs
	nextSystemID atomic.Uint32

	// pending replies: systemID -> channel
	pending   map[uint32]chan *Message
	pendingMu sync.Mutex

	// incoming data messages
	inbound chan *Message

	// lifecycle
	cancel context.CancelFunc
	done   chan struct{}
	closed atomic.Bool
}

// NewSession creates a new HSMS session with the given configuration.
func NewSession(config Config, logger *slog.Logger) *Session {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Session{
		config:  config,
		logger:  logger,
		pending: make(map[uint32]chan *Message),
		inbound: make(chan *Message, 256),
		done:    make(chan struct{}),
	}
	s.state.Store(int32(transport.StateDisconnected))
	return s
}

// State returns the current connection state.
func (s *Session) State() transport.State {
	return transport.State(s.state.Load())
}

// Connect establishes the TCP connection based on the configured role.
// For Passive mode, this starts listening and returns immediately.
// The actual connection acceptance happens in the background.
// Use WaitConnected() to wait for a peer to connect.
func (s *Session) Connect(ctx context.Context) error {
	if s.State() != transport.StateDisconnected {
		return fmt.Errorf("hsms: already connected (state=%s)", s.State())
	}

	s.state.Store(int32(transport.StateConnecting))

	switch s.config.Role {
	case RoleActive:
		if err := s.connectActive(ctx); err != nil {
			s.state.Store(int32(transport.StateDisconnected))
			return err
		}
		s.state.Store(int32(transport.StateConnected))
		s.startLoops()

	case RolePassive:
		if err := s.listenPassive(ctx); err != nil {
			s.state.Store(int32(transport.StateDisconnected))
			return err
		}
		// Accept runs in background; readLoop starts after accept
		go s.acceptAndRun(ctx)
	}

	return nil
}

func (s *Session) startLoops() {
	runCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.readLoop(runCtx)
	go s.linktestLoop(runCtx)
}

func (s *Session) connectActive(ctx context.Context) error {
	s.logger.Info("HSMS connecting", "address", s.config.Address, "role", "Active")
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("hsms: dial %s: %w", s.config.Address, err)
	}
	s.conn = conn
	s.logger.Info("HSMS TCP connected", "remote", conn.RemoteAddr())
	return nil
}

func (s *Session) listenPassive(ctx context.Context) error {
	s.logger.Info("HSMS listening", "address", s.config.Address, "role", "Passive")
	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("hsms: listen %s: %w", s.config.Address, err)
	}
	s.listener = listener
	s.logger.Info("HSMS listening on", "address", listener.Addr().String())
	return nil
}

func (s *Session) acceptAndRun(ctx context.Context) {
	conn, err := s.listener.Accept()
	if err != nil {
		if !s.closed.Load() {
			s.logger.Error("HSMS accept failed", "error", err)
		}
		return
	}
	s.conn = conn
	s.state.Store(int32(transport.StateConnected))
	s.logger.Info("HSMS TCP accepted", "remote", conn.RemoteAddr())
	s.startLoops()
}

// Addr returns the listener address (useful for Passive mode with port 0).
func (s *Session) Addr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	if s.conn != nil {
		return s.conn.LocalAddr()
	}
	return nil
}

// Select initiates the HSMS Select procedure (Active mode).
// Returns nil on success. Must be called after Connect().
func (s *Session) Select(ctx context.Context) error {
	if s.State() != transport.StateConnected {
		return fmt.Errorf("hsms: cannot select in state %s", s.State())
	}

	systemID := s.nextSystemID.Add(1)
	req := NewSelectReq(s.config.SessionID, systemID)
	rsp, err := s.sendAndWait(ctx, req, s.config.T6)
	if err != nil {
		return fmt.Errorf("hsms: select failed: %w", err)
	}

	if rsp.Header.SType != STypeSelectRsp {
		return fmt.Errorf("hsms: unexpected response type: %s", rsp.Header.SType)
	}

	status := SelectStatus(rsp.Header.Stream)
	if status != SelectStatusSuccess {
		return fmt.Errorf("hsms: select rejected with status %d", status)
	}

	s.state.Store(int32(transport.StateSelected))
	s.logger.Info("HSMS selected", "sessionID", s.config.SessionID)
	return nil
}

// Send transmits a data message and waits for the reply if WBit is set.
func (s *Session) Send(ctx context.Context, data []byte) error {
	if s.State() != transport.StateSelected {
		return fmt.Errorf("hsms: cannot send in state %s", s.State())
	}
	return s.writeMessage(&Message{
		Header: Header{
			SessionID: s.config.SessionID,
			SType:     STypeDataMessage,
		},
		Data: data,
	})
}

// SendMessage sends a complete HSMS message and optionally waits for a reply.
// If the message WBit is set and it's a primary message, it waits for the reply
// with the T3 timeout.
func (s *Session) SendMessage(ctx context.Context, msg *Message) (*Message, error) {
	if s.State() != transport.StateSelected && msg.Header.SType == STypeDataMessage {
		return nil, fmt.Errorf("hsms: cannot send data in state %s", s.State())
	}

	if msg.Header.SystemID == 0 {
		msg.Header.SystemID = s.nextSystemID.Add(1)
	}

	if msg.Header.WBit && msg.Header.Function%2 == 1 {
		return s.sendAndWait(ctx, msg, s.config.T3)
	}

	err := s.writeMessage(msg)
	return nil, err
}

// Receive returns the next incoming data message.
func (s *Session) Receive(ctx context.Context) ([]byte, error) {
	select {
	case msg, ok := <-s.inbound:
		if !ok {
			return nil, errors.New("hsms: session closed")
		}
		return msg.Data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ReceiveMessage returns the next incoming HSMS data message with full headers.
func (s *Session) ReceiveMessage(ctx context.Context) (*Message, error) {
	select {
	case msg, ok := <-s.inbound:
		if !ok {
			return nil, errors.New("hsms: session closed")
		}
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close gracefully shuts down the session.
func (s *Session) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	s.logger.Info("HSMS closing session")

	if s.cancel != nil {
		s.cancel()
	}

	var errs []error
	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	s.state.Store(int32(transport.StateDisconnected))
	close(s.done)
	return errors.Join(errs...)
}

// Done returns a channel that's closed when the session terminates.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// --- Internal ---

func (s *Session) writeMessage(msg *Message) error {
	data, err := msg.MarshalBinary()
	if err != nil {
		return fmt.Errorf("hsms: marshal: %w", err)
	}
	if s.conn == nil {
		return errors.New("hsms: no connection")
	}
	_, err = s.conn.Write(data)
	return err
}

func (s *Session) sendAndWait(ctx context.Context, msg *Message, timeout time.Duration) (*Message, error) {
	ch := make(chan *Message, 1)
	s.pendingMu.Lock()
	s.pending[msg.Header.SystemID] = ch
	s.pendingMu.Unlock()

	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, msg.Header.SystemID)
		s.pendingMu.Unlock()
	}()

	if err := s.writeMessage(msg); err != nil {
		return nil, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case rsp := <-ch:
		return rsp, nil
	case <-timer.C:
		return nil, fmt.Errorf("hsms: timeout waiting for reply (systemID=%d, timeout=%s)", msg.Header.SystemID, timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Session) readLoop(ctx context.Context) {
	defer func() {
		s.logger.Info("HSMS read loop exiting")
		if !s.closed.Load() {
			s.Close()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, err := s.readMessage()
		if err != nil {
			if s.closed.Load() {
				return
			}
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				s.logger.Warn("HSMS connection closed by peer")
				return
			}
			s.logger.Error("HSMS read error", "error", err)
			return
		}

		s.handleMessage(ctx, msg)
	}
}

func (s *Session) readMessage() (*Message, error) {
	// Read 4-byte length header
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(s.conn, lenBuf); err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint32(lenBuf)
	if msgLen < headerLen {
		return nil, fmt.Errorf("hsms: message length %d too small", msgLen)
	}

	// Read the rest of the message
	msgBuf := make([]byte, 4+msgLen)
	copy(msgBuf[0:4], lenBuf)
	if _, err := io.ReadFull(s.conn, msgBuf[4:]); err != nil {
		return nil, err
	}

	msg := &Message{}
	if err := msg.UnmarshalBinary(msgBuf); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *Session) handleMessage(ctx context.Context, msg *Message) {
	// Check if it's a reply to a pending request
	s.pendingMu.Lock()
	ch, isPending := s.pending[msg.Header.SystemID]
	s.pendingMu.Unlock()

	if isPending {
		select {
		case ch <- msg:
		default:
		}
		return
	}

	switch msg.Header.SType {
	case STypeDataMessage:
		select {
		case s.inbound <- msg:
		default:
			s.logger.Warn("HSMS inbound buffer full, dropping message",
				"stream", msg.Header.Stream, "function", msg.Header.Function)
		}

	case STypeSelectReq:
		s.handleSelectReq(msg)

	case STypeDeselectReq:
		s.handleDeselectReq(msg)

	case STypeLinktestReq:
		rsp := NewLinktestRsp(msg.Header.SystemID)
		if err := s.writeMessage(rsp); err != nil {
			s.logger.Error("HSMS linktest response failed", "error", err)
		}

	case STypeSeparateReq:
		s.logger.Info("HSMS received Separate.req, closing")
		s.Close()

	default:
		s.logger.Warn("HSMS unhandled message type", "stype", msg.Header.SType)
	}
}

func (s *Session) handleSelectReq(msg *Message) {
	status := SelectStatusSuccess
	currentState := s.State()

	if currentState == transport.StateSelected {
		status = SelectStatusAlreadyActive
	}

	rsp := NewSelectRsp(msg.Header.SessionID, msg.Header.SystemID, status)
	if err := s.writeMessage(rsp); err != nil {
		s.logger.Error("HSMS select response failed", "error", err)
		return
	}

	if status == SelectStatusSuccess {
		s.state.Store(int32(transport.StateSelected))
		s.logger.Info("HSMS selected by peer", "sessionID", msg.Header.SessionID)
	}
}

func (s *Session) handleDeselectReq(msg *Message) {
	rsp := NewDeselectRsp(msg.Header.SessionID, msg.Header.SystemID, 0)
	if err := s.writeMessage(rsp); err != nil {
		s.logger.Error("HSMS deselect response failed", "error", err)
		return
	}
	s.state.Store(int32(transport.StateConnected))
	s.logger.Info("HSMS deselected by peer")
}

func (s *Session) linktestLoop(ctx context.Context) {
	if s.config.LinktestInterval <= 0 {
		return
	}

	ticker := time.NewTicker(s.config.LinktestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.State() < transport.StateConnected {
				continue
			}
			systemID := s.nextSystemID.Add(1)
			req := NewLinktestReq(systemID)
			_, err := s.sendAndWait(ctx, req, s.config.T6)
			if err != nil {
				s.logger.Error("HSMS linktest failed", "error", err)
				s.Close()
				return
			}
			s.logger.Debug("HSMS linktest ok")
		}
	}
}
