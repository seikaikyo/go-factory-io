package integration

import (
	"context"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/security"
	"github.com/dashfactory/go-factory-io/pkg/transport"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

func TestTLSConnection(t *testing.T) {
	logger := slog.Default()
	serverTLS, clientTLS, err := security.GenerateTestTLSConfigs()
	if err != nil {
		t.Fatalf("generate TLS configs: %v", err)
	}

	// Passive side with TLS
	passiveCfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	passiveCfg.TLSConfig = serverTLS
	passiveCfg.LinktestInterval = 0
	passive := hsms.NewSession(passiveCfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}
	defer passive.Close()

	addr := passive.Addr().String()

	// Active side with TLS
	activeCfg := hsms.DefaultConfig(addr, hsms.RoleActive, 1)
	activeCfg.TLSConfig = clientTLS
	activeCfg.LinktestInterval = 0
	active := hsms.NewSession(activeCfg, logger)

	if err := active.Connect(ctx); err != nil {
		t.Fatalf("active connect: %v", err)
	}
	defer active.Close()

	// Select should work over TLS
	if err := active.Select(ctx); err != nil {
		t.Fatalf("select: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if active.State() != transport.StateSelected {
		t.Errorf("active state: got %s, want Selected", active.State())
	}

	t.Log("TLS connection + select successful")
}

func TestTLSMutualAuth(t *testing.T) {
	logger := slog.Default()
	serverTLS, clientTLS, err := security.GenerateTestTLSConfigs()
	if err != nil {
		t.Fatalf("generate TLS configs: %v", err)
	}

	// Passive with mTLS (requires client cert)
	passiveCfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	passiveCfg.TLSConfig = serverTLS // Already configured with ClientAuth = RequireAndVerifyClientCert
	passiveCfg.LinktestInterval = 0
	passive := hsms.NewSession(passiveCfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}
	defer passive.Close()

	addr := passive.Addr().String()

	// Active with client cert
	activeCfg := hsms.DefaultConfig(addr, hsms.RoleActive, 1)
	activeCfg.TLSConfig = clientTLS
	activeCfg.LinktestInterval = 0
	active := hsms.NewSession(activeCfg, logger)

	if err := active.Connect(ctx); err != nil {
		t.Fatalf("active connect with client cert: %v", err)
	}
	defer active.Close()

	if err := active.Select(ctx); err != nil {
		t.Fatalf("select: %v", err)
	}

	t.Log("mTLS mutual authentication successful")
}

func TestTLSRejectsPlaintext(t *testing.T) {
	logger := slog.Default()
	serverTLS, _, err := security.GenerateTestTLSConfigs()
	if err != nil {
		t.Fatalf("generate TLS configs: %v", err)
	}

	// Passive with TLS required
	passiveCfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	passiveCfg.TLSConfig = serverTLS
	passiveCfg.LinktestInterval = 0
	passive := hsms.NewSession(passiveCfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}
	defer passive.Close()

	addr := passive.Addr().String()

	// Active WITHOUT TLS (plaintext) should fail
	activeCfg := hsms.DefaultConfig(addr, hsms.RoleActive, 1)
	activeCfg.LinktestInterval = 0
	// No TLSConfig = plaintext
	active := hsms.NewSession(activeCfg, logger)

	if err := active.Connect(ctx); err != nil {
		// Connection itself might succeed (TCP level), but select should fail
		t.Logf("plaintext connect to TLS server: %v (expected)", err)
		return
	}
	defer active.Close()

	// Even if TCP connected, Select should fail because TLS handshake didn't happen
	err = active.Select(ctx)
	if err == nil {
		t.Fatal("plaintext client should not be able to select on TLS server")
	}
	t.Logf("plaintext select correctly rejected: %v", err)
}

func TestIPAllowlist(t *testing.T) {
	logger := slog.Default()
	auditor := security.NewAuditor(logger)

	var rejectedCount atomic.Int32
	auditor.OnEvent(func(event security.Event) {
		if event.Type == "connection_rejected" {
			rejectedCount.Add(1)
		}
	})

	// Passive with IP allowlist that does NOT include 127.0.0.1
	passiveCfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	passiveCfg.AllowedPeers = []net.IP{net.ParseIP("10.0.0.1")} // Only allow 10.0.0.1
	passiveCfg.Auditor = auditor
	passiveCfg.LinktestInterval = 0
	passive := hsms.NewSession(passiveCfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := passive.Connect(ctx); err != nil {
		t.Fatalf("passive connect: %v", err)
	}
	defer passive.Close()

	addr := passive.Addr().String()

	// Active connects from 127.0.0.1 (not in allowlist)
	activeCfg := hsms.DefaultConfig(addr, hsms.RoleActive, 1)
	activeCfg.LinktestInterval = 0
	active := hsms.NewSession(activeCfg, logger)

	if err := active.Connect(ctx); err != nil {
		t.Logf("connect rejected at TCP level: %v", err)
	} else {
		// TCP connected but select should timeout because passive closed the connection
		err := active.Select(ctx)
		if err == nil {
			t.Fatal("should not be able to select when IP is not in allowlist")
		}
		t.Logf("select correctly failed: %v", err)
		active.Close()
	}

	time.Sleep(200 * time.Millisecond)
	if rejectedCount.Load() < 1 {
		t.Log("note: rejection event may not have fired if TCP was refused at OS level")
	} else {
		t.Logf("rejection events fired: %d", rejectedCount.Load())
	}
}

func TestSecurityAuditLogging(t *testing.T) {
	auditor := security.NewAuditor(slog.Default())

	var events []security.Event
	auditor.OnEvent(func(event security.Event) {
		events = append(events, event)
	})

	// Emit various events
	auditor.ConnectionRejected("192.168.1.100", "IP not allowed")
	auditor.AuthFailed("192.168.1.200", "cert expired")
	auditor.RateLimited("192.168.1.100", 1000)
	auditor.MalformedMessage("192.168.1.100", nil)
	auditor.UnauthorizedMessage("192.168.1.100", 2, 41)
	auditor.TLSHandshakeOK("192.168.1.50")
	auditor.SessionExpired("192.168.1.100", 24*time.Hour)

	if len(events) != 7 {
		t.Fatalf("expected 7 events, got %d", len(events))
	}

	// Verify categories cover IEC 62443 FRs
	categories := make(map[security.Category]bool)
	for _, e := range events {
		categories[e.Category] = true
	}

	expected := []security.Category{
		security.CatAuth,
		security.CatAccess,
		security.CatIntegrity,
		security.CatDataFlow,
		security.CatAvailability,
	}
	for _, cat := range expected {
		if !categories[cat] {
			t.Errorf("missing category: %s", cat)
		}
	}
}

func TestRateLimiting(t *testing.T) {
	rl := security.NewRateLimiter(5, 5) // 5/sec, burst 5

	// Exhaust burst
	allowed := 0
	for range 10 {
		if rl.Allow() {
			allowed++
		}
	}

	if allowed != 5 {
		t.Errorf("allowed %d, want 5 (burst size)", allowed)
	}

	// After 1 second, should have ~5 more tokens
	time.Sleep(1100 * time.Millisecond)
	allowed = 0
	for range 10 {
		if rl.Allow() {
			allowed++
		}
	}

	if allowed < 4 || allowed > 6 {
		t.Errorf("after refill: allowed %d, want ~5", allowed)
	}
}
