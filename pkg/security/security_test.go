package security

import (
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

// --- Rate Limiter Tests ---

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(10, 10) // 10/sec, burst 10

	// Should allow burst
	for i := range 10 {
		if !rl.Allow() {
			t.Fatalf("denied at burst %d", i)
		}
	}

	// Should deny after burst exhausted
	if rl.Allow() {
		t.Fatal("should deny after burst exhausted")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(100, 5) // 100/sec, burst 5

	// Exhaust burst
	for range 5 {
		rl.Allow()
	}

	// Wait for refill
	time.Sleep(60 * time.Millisecond)

	// Should have ~6 tokens refilled
	if !rl.Allow() {
		t.Fatal("should allow after refill")
	}
}

func TestRateLimiterZeroRate(t *testing.T) {
	rl := NewRateLimiter(0, 0) // 0 rate, 0 burst
	if rl.Allow() {
		t.Fatal("should deny with zero rate")
	}
}

// --- Auditor Tests ---

func TestAuditorEmit(t *testing.T) {
	logger := slog.Default()
	auditor := NewAuditor(logger)

	var count atomic.Int32
	auditor.OnEvent(func(event Event) {
		count.Add(1)
	})

	auditor.ConnectionRejected("192.168.1.100", "IP not in allowlist")
	auditor.AuthFailed("192.168.1.200", "TLS handshake failed")
	auditor.RateLimited("192.168.1.100", 1000)
	auditor.MalformedMessage("192.168.1.100", nil)

	// nil error test — should not panic
	auditor.MalformedMessage("192.168.1.100", nil)

	if count.Load() != 5 {
		t.Errorf("event count: got %d, want 5", count.Load())
	}
}

func TestAuditorEventFields(t *testing.T) {
	logger := slog.Default()
	auditor := NewAuditor(logger)

	var captured Event
	auditor.OnEvent(func(event Event) {
		captured = event
	})

	auditor.AuthFailed("10.0.0.1:5000", "certificate expired")

	if captured.Level != LevelCritical {
		t.Errorf("level: got %s, want CRITICAL", captured.Level)
	}
	if captured.Category != CatAuth {
		t.Errorf("category: got %s, want auth", captured.Category)
	}
	if captured.Type != "auth_failed" {
		t.Errorf("type: got %s, want auth_failed", captured.Type)
	}
	if captured.Source != "10.0.0.1:5000" {
		t.Errorf("source: got %s", captured.Source)
	}
	if captured.Time.IsZero() {
		t.Error("time should be auto-set")
	}
}

// --- TLS Config Tests ---

func TestDefaultTLSConfig(t *testing.T) {
	cfg := DefaultTLSConfig()
	if cfg.MinVersion != 0x0303 { // TLS 1.2
		t.Errorf("MinVersion: got 0x%04x, want TLS 1.2 (0x0303)", cfg.MinVersion)
	}
	if len(cfg.CipherSuites) == 0 {
		t.Error("no cipher suites configured")
	}
}

func TestGenerateTestTLSConfigs(t *testing.T) {
	server, client, err := GenerateTestTLSConfigs()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if len(server.Certificates) == 0 {
		t.Error("server has no certificates")
	}
	if server.ClientCAs == nil {
		t.Error("server has no client CA pool (mTLS not configured)")
	}
	if len(client.Certificates) == 0 {
		t.Error("client has no certificates")
	}
	if client.RootCAs == nil {
		t.Error("client has no root CA pool")
	}
}

func TestLoadServerTLSMissingFile(t *testing.T) {
	_, err := LoadServerTLS("/nonexistent/cert.pem", "/nonexistent/key.pem", "")
	if err == nil {
		t.Fatal("expected error for missing cert file")
	}
}

func TestLoadClientTLSMissingFile(t *testing.T) {
	_, err := LoadClientTLS("/nonexistent/cert.pem", "/nonexistent/key.pem", "")
	if err == nil {
		t.Fatal("expected error for missing cert file")
	}
}
