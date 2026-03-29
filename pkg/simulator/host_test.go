package simulator

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	existingsim "github.com/dashfactory/go-factory-io/examples/simulator"
	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"github.com/dashfactory/go-factory-io/pkg/validator"
)

// startEquipment creates and starts an equipment simulator on a random port.
func startEquipment(t *testing.T) (*existingsim.Equipment, string) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := existingsim.EquipmentConfig{
		ListenAddress:    ":0",
		SessionID:        1,
		ModelName:        "TEST-EQUIP",
		SoftwareRevision: "1.0.0",
		EventInterval:    0,
	}
	eq := existingsim.NewEquipment(cfg, logger)
	ctx := context.Background()
	if err := eq.Start(ctx); err != nil {
		t.Fatal("start equipment:", err)
	}
	addr := eq.Addr()
	return eq, addr
}

func TestHost_EstablishComm(t *testing.T) {
	eq, addr := startEquipment(t)
	defer eq.Stop()

	// Wait for equipment to be ready
	time.Sleep(50 * time.Millisecond)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	host := NewHost(addr, 1, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := host.Connect(ctx); err != nil {
		t.Fatal("connect:", err)
	}
	defer host.Close()

	reply, err := host.EstablishComm(ctx)
	if err != nil {
		t.Fatal("establish comm:", err)
	}
	if reply == nil {
		t.Fatal("expected reply body")
	}
	// S1F14 should be L:2 { B:1 COMMACK, L:2 { A, A } }
	if reply.Len() < 2 {
		t.Errorf("S1F14 reply too short: %d items", reply.Len())
	}
}

func TestHost_AreYouThere(t *testing.T) {
	eq, addr := startEquipment(t)
	defer eq.Stop()
	time.Sleep(50 * time.Millisecond)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	host := NewHost(addr, 1, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := host.Connect(ctx); err != nil {
		t.Fatal("connect:", err)
	}
	defer host.Close()

	reply, err := host.AreYouThere(ctx)
	if err != nil {
		t.Fatal("are you there:", err)
	}
	if reply == nil {
		t.Fatal("expected reply body")
	}
	// S1F2 should be L:2 { A MDLN, A SOFTREV }
	if reply.Len() < 2 {
		t.Errorf("S1F2 reply too short: %d items", reply.Len())
	}
}

func TestHost_InterceptorCalled(t *testing.T) {
	eq, addr := startEquipment(t)
	defer eq.Stop()
	time.Sleep(50 * time.Millisecond)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	host := NewHost(addr, 1, logger)

	var txCount, rxCount int
	host.SetInterceptor(func(dir Direction, stream, function byte, body *secs2.Item, results []validator.ValidationResult) {
		if dir == DirTX {
			txCount++
		} else {
			rxCount++
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := host.Connect(ctx); err != nil {
		t.Fatal("connect:", err)
	}
	defer host.Close()

	host.EstablishComm(ctx)

	if txCount != 1 {
		t.Errorf("expected 1 TX intercept, got %d", txCount)
	}
	if rxCount != 1 {
		t.Errorf("expected 1 RX intercept, got %d", rxCount)
	}
}

func TestHost_SendRCMD(t *testing.T) {
	eq, addr := startEquipment(t)
	defer eq.Stop()
	time.Sleep(50 * time.Millisecond)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	host := NewHost(addr, 1, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := host.Connect(ctx); err != nil {
		t.Fatal("connect:", err)
	}
	defer host.Close()

	// Establish comm first so handler is in correct state
	host.EstablishComm(ctx)

	reply, err := host.SendRCMD(ctx, "START", nil)
	if err != nil {
		t.Fatal("send RCMD:", err)
	}
	if reply == nil {
		t.Fatal("expected reply body")
	}
}
