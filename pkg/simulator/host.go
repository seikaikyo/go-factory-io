// Package simulator provides a host-side (MES) simulator for testing SECS/GEM
// equipment, fault injection for resilience testing, and YAML-based scenario scripts.
package simulator

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
	"github.com/dashfactory/go-factory-io/pkg/validator"
)

// Direction indicates message flow direction.
type Direction string

const (
	DirTX Direction = "tx" // Sent by this side
	DirRX Direction = "rx" // Received from remote
)

// MessageInterceptor is called for every sent/received message with validation results.
type MessageInterceptor func(dir Direction, stream, function byte, body *secs2.Item, results []validator.ValidationResult)

// Host simulates a MES host connecting to equipment in active mode.
type Host struct {
	session   *hsms.Session
	sessionID uint16
	logger    *slog.Logger
	schemas   *validator.SchemaRegistry
	intercept MessageInterceptor
}

// NewHost creates a host simulator that connects to the given equipment address.
func NewHost(addr string, sessionID uint16, logger *slog.Logger) *Host {
	if logger == nil {
		logger = slog.Default()
	}
	cfg := hsms.DefaultConfig(addr, hsms.RoleActive, sessionID)
	cfg.LinktestInterval = 0 // Studio controls linktest
	return &Host{
		session:   hsms.NewSession(cfg, logger),
		sessionID: sessionID,
		logger:    logger,
		schemas:   validator.DefaultRegistry(),
	}
}

// Session returns the underlying HSMS session.
func (h *Host) Session() *hsms.Session { return h.session }

// SetInterceptor sets the message interceptor callback.
func (h *Host) SetInterceptor(fn MessageInterceptor) { h.intercept = fn }

// Connect establishes the HSMS connection and performs Select.
func (h *Host) Connect(ctx context.Context) error {
	if err := h.session.Connect(ctx); err != nil {
		return fmt.Errorf("host: connect: %w", err)
	}
	if err := h.session.Select(ctx); err != nil {
		return fmt.Errorf("host: select: %w", err)
	}
	return nil
}

// Close disconnects the host session.
func (h *Host) Close() error {
	return h.session.Close()
}

// sendAndValidate sends a message and validates the reply.
func (h *Host) sendAndValidate(ctx context.Context, stream, function byte, wbit bool, body *secs2.Item) (*secs2.Item, error) {
	var data []byte
	if body != nil {
		var err error
		data, err = secs2.Encode(body)
		if err != nil {
			return nil, fmt.Errorf("host: encode: %w", err)
		}
	}

	msg := hsms.NewDataMessage(h.sessionID, stream, function, wbit, 0, data)

	// Intercept TX
	if h.intercept != nil {
		results := h.schemas.ValidateMessage(stream, function, body)
		h.intercept(DirTX, stream, function, body, results)
	}

	reply, err := h.session.SendMessage(ctx, msg)
	if err != nil {
		return nil, err
	}

	if reply == nil || len(reply.Data) == 0 {
		// Intercept RX (empty)
		if h.intercept != nil {
			h.intercept(DirRX, stream, function+1, nil, nil)
		}
		return nil, nil
	}

	item, err := secs2.Decode(reply.Data)
	if err != nil {
		return nil, fmt.Errorf("host: decode reply: %w", err)
	}

	// Intercept RX with validation
	if h.intercept != nil {
		results := h.schemas.ValidateMessage(stream, function+1, item)
		h.intercept(DirRX, stream, function+1, item, results)
	}

	return item, nil
}

// EstablishComm sends S1F13 and returns the S1F14 reply body.
func (h *Host) EstablishComm(ctx context.Context) (*secs2.Item, error) {
	body := secs2.NewList(
		secs2.NewASCII("HOST"),
		secs2.NewASCII("1.0.0"),
	)
	return h.sendAndValidate(ctx, 1, 13, true, body)
}

// AreYouThere sends S1F1 and returns the S1F2 reply body.
func (h *Host) AreYouThere(ctx context.Context) (*secs2.Item, error) {
	return h.sendAndValidate(ctx, 1, 1, true, nil)
}

// RequestOnline sends S1F17 and returns the S1F18 reply body.
func (h *Host) RequestOnline(ctx context.Context) (*secs2.Item, error) {
	return h.sendAndValidate(ctx, 1, 17, true, nil)
}

// RequestOffline sends S1F15 and returns the S1F16 reply body.
func (h *Host) RequestOffline(ctx context.Context) (*secs2.Item, error) {
	return h.sendAndValidate(ctx, 1, 15, true, nil)
}

// GetSVs sends S1F3 for the specified SVIDs and returns the S1F4 reply.
func (h *Host) GetSVs(ctx context.Context, svids []uint32) (*secs2.Item, error) {
	items := make([]*secs2.Item, len(svids))
	for i, id := range svids {
		items[i] = secs2.NewU4(id)
	}
	body := secs2.NewList(items...)
	return h.sendAndValidate(ctx, 1, 3, true, body)
}

// SendRCMD sends S2F41 with the given command and parameters.
func (h *Host) SendRCMD(ctx context.Context, cmd string, params map[string]string) (*secs2.Item, error) {
	var paramItems []*secs2.Item
	for name, val := range params {
		paramItems = append(paramItems, secs2.NewList(
			secs2.NewASCII(name),
			secs2.NewASCII(val),
		))
	}
	body := secs2.NewList(
		secs2.NewASCII(cmd),
		secs2.NewList(paramItems...),
	)
	return h.sendAndValidate(ctx, 2, 41, true, body)
}

// SendRaw sends an arbitrary SECS-II message and returns the decoded reply.
func (h *Host) SendRaw(ctx context.Context, stream, function byte, wbit bool, body *secs2.Item) (*secs2.Item, error) {
	return h.sendAndValidate(ctx, stream, function, wbit, body)
}

// ReceiveMessage returns the next incoming unsolicited message.
func (h *Host) ReceiveMessage(ctx context.Context) (*hsms.Message, error) {
	return h.session.ReceiveMessage(ctx)
}
