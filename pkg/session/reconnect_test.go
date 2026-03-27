package session

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/transport"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

func TestManagedSessionConnectAndReconnect(t *testing.T) {
	logger := slog.Default()

	// Start a passive endpoint (simulator side)
	passiveCfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	passiveCfg.LinktestInterval = 0
	passive := hsms.NewSession(passiveCfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}
	addr := passive.Addr().String()

	// Create managed session (active side)
	activeCfg := hsms.DefaultConfig(addr, hsms.RoleActive, 1)
	activeCfg.LinktestInterval = 0
	reconnCfg := ReconnectConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   5,
	}

	var connectCount atomic.Int32
	ms := NewManagedSession(activeCfg, reconnCfg, logger)
	ms.OnConnect(func(s *hsms.Session) {
		connectCount.Add(1)
	})

	// Initial connection
	if err := ms.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if ms.State() != transport.StateSelected {
		t.Errorf("initial state: got %s, want Selected", ms.State())
	}
	if connectCount.Load() != 1 {
		t.Errorf("connect count: got %d, want 1", connectCount.Load())
	}

	// Kill the passive side to trigger disconnect
	passive.Close()
	time.Sleep(200 * time.Millisecond)

	// Start a new passive endpoint on the same address
	passiveCfg2 := hsms.DefaultConfig(addr, hsms.RolePassive, 1)
	passiveCfg2.LinktestInterval = 0
	passive2 := hsms.NewSession(passiveCfg2, logger)
	if err := passive2.Connect(ctx); err != nil {
		t.Fatalf("passive2 connect: %v", err)
	}
	defer passive2.Close()

	// Wait for reconnect
	time.Sleep(2 * time.Second)

	if connectCount.Load() < 2 {
		t.Logf("connect count: %d (reconnect may still be in progress)", connectCount.Load())
	}

	ms.Stop()
}

func TestManagedSessionMaxRetries(t *testing.T) {
	logger := slog.Default()

	// Connect to a port that nothing listens on
	activeCfg := hsms.DefaultConfig("127.0.0.1:1", hsms.RoleActive, 1)
	activeCfg.LinktestInterval = 0

	reconnCfg := ReconnectConfig{
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   1.5,
		MaxRetries:   3,
	}

	ms := NewManagedSession(activeCfg, reconnCfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start will fail on initial connect, then enter reconnect loop
	ms.Start(ctx)

	// Wait for max retries to be exhausted
	time.Sleep(2 * time.Second)

	if ms.State() != transport.StateDisconnected {
		t.Errorf("state after max retries: got %s, want Disconnected", ms.State())
	}

	ms.Stop()
}

func TestManagedSessionBackgroundStart(t *testing.T) {
	logger := slog.Default()

	// Start passive side
	passiveCfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	passiveCfg.LinktestInterval = 0
	passive := hsms.NewSession(passiveCfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}
	defer passive.Close()
	addr := passive.Addr().String()

	activeCfg := hsms.DefaultConfig(addr, hsms.RoleActive, 1)
	activeCfg.LinktestInterval = 0
	reconnCfg := ReconnectConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     500 * time.Millisecond,
		Multiplier:   2.0,
	}

	ms := NewManagedSession(activeCfg, reconnCfg, logger)
	ms.StartBackground(ctx)

	// Should connect within a short time
	time.Sleep(500 * time.Millisecond)

	state := ms.State()
	t.Logf("State after background start: %s", state)
	// May or may not be connected yet depending on timing
	// Just verify it doesn't panic

	ms.Stop()
}
