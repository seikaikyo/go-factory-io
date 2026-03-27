// Package session provides connection management with automatic reconnection.
package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/transport"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

// ReconnectConfig controls the reconnection behavior.
type ReconnectConfig struct {
	// InitialDelay is the first reconnect delay (typically T5).
	InitialDelay time.Duration

	// MaxDelay caps the exponential backoff.
	MaxDelay time.Duration

	// Multiplier for each successive retry (typically 2.0).
	Multiplier float64

	// MaxRetries is the maximum number of reconnect attempts. 0 = unlimited.
	MaxRetries int
}

// DefaultReconnectConfig returns a sensible default.
func DefaultReconnectConfig() ReconnectConfig {
	return ReconnectConfig{
		InitialDelay: 10 * time.Second, // T5
		MaxDelay:     5 * time.Minute,
		Multiplier:   2.0,
		MaxRetries:   0, // unlimited
	}
}

// ManagedSession wraps an HSMS Session with automatic reconnection.
type ManagedSession struct {
	mu          sync.RWMutex
	config      hsms.Config
	reconnCfg   ReconnectConfig
	logger      *slog.Logger
	session     *hsms.Session
	onConnect   func(*hsms.Session) // Callback after successful connection+select
	onDisconnect func()             // Callback when disconnected

	cancel context.CancelFunc
	done   chan struct{}
}

// NewManagedSession creates a managed session with auto-reconnect.
func NewManagedSession(config hsms.Config, reconnCfg ReconnectConfig, logger *slog.Logger) *ManagedSession {
	if logger == nil {
		logger = slog.Default()
	}
	return &ManagedSession{
		config:    config,
		reconnCfg: reconnCfg,
		logger:    logger,
		done:      make(chan struct{}),
	}
}

// OnConnect sets a callback invoked after each successful connection.
func (ms *ManagedSession) OnConnect(fn func(*hsms.Session)) {
	ms.mu.Lock()
	ms.onConnect = fn
	ms.mu.Unlock()
}

// OnDisconnect sets a callback invoked when the connection is lost.
func (ms *ManagedSession) OnDisconnect(fn func()) {
	ms.mu.Lock()
	ms.onDisconnect = fn
	ms.mu.Unlock()
}

// Session returns the current active session (may be nil if disconnected).
func (ms *ManagedSession) Session() *hsms.Session {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.session
}

// Start begins the connection loop. It connects, selects, and monitors the
// connection, automatically reconnecting on failure.
func (ms *ManagedSession) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	ms.cancel = cancel

	// Try initial connection
	err := ms.connect(runCtx)
	if err != nil {
		// Start reconnect loop in background
		go ms.reconnectLoop(runCtx)
		return err
	}

	// Monitor connection health
	go ms.monitorLoop(runCtx)
	return nil
}

// StartBackground begins the connection loop without blocking.
// It returns immediately and connects in the background.
func (ms *ManagedSession) StartBackground(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	ms.cancel = cancel
	go ms.reconnectLoop(runCtx)
}

// Stop shuts down the managed session and its reconnection loop.
func (ms *ManagedSession) Stop() error {
	if ms.cancel != nil {
		ms.cancel()
	}

	ms.mu.Lock()
	s := ms.session
	ms.session = nil
	ms.mu.Unlock()

	if s != nil {
		return s.Close()
	}
	close(ms.done)
	return nil
}

// Done returns a channel that's closed when the managed session stops.
func (ms *ManagedSession) Done() <-chan struct{} {
	return ms.done
}

// State returns the current transport state.
func (ms *ManagedSession) State() transport.State {
	ms.mu.RLock()
	s := ms.session
	ms.mu.RUnlock()
	if s == nil {
		return transport.StateDisconnected
	}
	return s.State()
}

func (ms *ManagedSession) connect(ctx context.Context) error {
	session := hsms.NewSession(ms.config, ms.logger)

	if err := session.Connect(ctx); err != nil {
		return err
	}

	// For Active mode, perform Select
	if ms.config.Role == hsms.RoleActive {
		if err := session.Select(ctx); err != nil {
			session.Close()
			return err
		}
	}

	ms.mu.Lock()
	ms.session = session
	cb := ms.onConnect
	ms.mu.Unlock()

	if cb != nil {
		cb(session)
	}

	ms.logger.Info("Managed session connected", "address", ms.config.Address)
	return nil
}

func (ms *ManagedSession) monitorLoop(ctx context.Context) {
	for {
		ms.mu.RLock()
		s := ms.session
		ms.mu.RUnlock()

		if s == nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-s.Done():
			ms.logger.Warn("Managed session disconnected, starting reconnect")

			ms.mu.Lock()
			ms.session = nil
			cb := ms.onDisconnect
			ms.mu.Unlock()

			if cb != nil {
				cb()
			}

			ms.reconnectLoop(ctx)
			return
		}
	}
}

func (ms *ManagedSession) reconnectLoop(ctx context.Context) {
	delay := ms.reconnCfg.InitialDelay
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		attempt++
		if ms.reconnCfg.MaxRetries > 0 && attempt > ms.reconnCfg.MaxRetries {
			ms.logger.Error("Max reconnect retries exceeded",
				"attempts", attempt-1,
				"maxRetries", ms.reconnCfg.MaxRetries,
			)
			return
		}

		ms.logger.Info("Reconnecting",
			"attempt", attempt,
			"delay", delay,
			"address", ms.config.Address,
		)

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		connectCtx, connectCancel := context.WithTimeout(ctx, 30*time.Second)
		err := ms.connect(connectCtx)
		connectCancel()

		if err == nil {
			ms.logger.Info("Reconnected successfully", "attempt", attempt)
			// Start monitoring again
			go ms.monitorLoop(ctx)
			return
		}

		ms.logger.Warn("Reconnect failed", "attempt", attempt, "error", err)

		// Exponential backoff
		delay = time.Duration(float64(delay) * ms.reconnCfg.Multiplier)
		if delay > ms.reconnCfg.MaxDelay {
			delay = ms.reconnCfg.MaxDelay
		}
	}
}
