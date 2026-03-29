package simulator

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func setupHost(t *testing.T, addr string) *Host {
	t.Helper()
	time.Sleep(50 * time.Millisecond)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	host := NewHost(addr, 1, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := host.Connect(ctx); err != nil {
		t.Fatal("connect:", err)
	}
	return host
}

func TestAllFaultTypes(t *testing.T) {
	types := AllFaultTypes()
	if len(types) != 5 {
		t.Errorf("expected 5 fault types, got %d", len(types))
	}
}

func TestFaultInjector_DisconnectEquipment(t *testing.T) {
	eq, addr := startEquipment(t)
	defer eq.Stop()

	host := setupHost(t, addr)
	defer host.Close()

	fi := NewFaultInjector(host.Session(), host.logger)
	if err := fi.Inject(FaultDisconnect, nil); err != nil {
		t.Fatal("disconnect fault:", err)
	}
	// After disconnect, session should be closed
	// Attempting to send should fail (but we don't need to test that here)
}

func TestFaultInjector_UnknownFault(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fi := NewFaultInjector(nil, logger)
	err := fi.Inject("unknown_fault", nil)
	if err == nil {
		t.Error("expected error for unknown fault type")
	}
}

func TestFaultInjector_CorruptNoConn(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fi := NewFaultInjector(nil, logger)
	err := fi.Inject(FaultCorruptData, nil)
	if err == nil {
		t.Error("expected error when no raw connection")
	}
}
