package security

import (
	"testing"
	"time"
)

func TestSecurityStatusCounters(t *testing.T) {
	ss := NewSecurityStatus()

	ss.RecordAuthFailure()
	ss.RecordAuthFailure()
	ss.RecordConnection()
	ss.RecordRejectedConnection()
	ss.RecordRateLimit()
	ss.RecordMalformed()
	ss.RecordUnauthorized()

	if ss.AuthFailures.Load() != 2 {
		t.Errorf("AuthFailures = %d, want 2", ss.AuthFailures.Load())
	}
	if ss.ConnectionsTotal.Load() != 1 {
		t.Errorf("ConnectionsTotal = %d, want 1", ss.ConnectionsTotal.Load())
	}
	if ss.RejectedConns.Load() != 1 {
		t.Errorf("RejectedConns = %d, want 1", ss.RejectedConns.Load())
	}
	if ss.LastAuthFailure.IsZero() {
		t.Error("LastAuthFailure should be set")
	}
	if ss.LastRateLimit.IsZero() {
		t.Error("LastRateLimit should be set")
	}
}

func TestSecurityStatusReport(t *testing.T) {
	ss := NewSecurityStatus()
	ss.SetTLS(true, "TLS 1.3", "TLS_AES_256_GCM_SHA384", true)
	ss.RecordConnection()

	report := ss.Report()

	if report["tls_enabled"] != true {
		t.Error("tls_enabled should be true")
	}
	if report["tls_version"] != "TLS 1.3" {
		t.Errorf("tls_version = %v, want TLS 1.3", report["tls_version"])
	}
	if report["mutual_tls"] != true {
		t.Error("mutual_tls should be true")
	}

	counters := report["counters"].(map[string]interface{})
	if counters["connections_total"] != int64(1) {
		t.Errorf("connections_total = %v, want 1", counters["connections_total"])
	}

	uptime := report["uptime_seconds"].(int)
	if uptime < 0 {
		t.Errorf("uptime = %d, should be >= 0", uptime)
	}
}

func TestSecurityStatusEventHandler(t *testing.T) {
	ss := NewSecurityStatus()
	handler := ss.AsEventHandler()

	handler(Event{Time: time.Now(), Type: "auth_failed"})
	handler(Event{Time: time.Now(), Type: "auth_failed"})
	handler(Event{Time: time.Now(), Type: "rate_limited"})
	handler(Event{Time: time.Now(), Type: "connection_rejected"})
	handler(Event{Time: time.Now(), Type: "malformed_message"})
	handler(Event{Time: time.Now(), Type: "unauthorized_message"})
	handler(Event{Time: time.Now(), Type: "tls_handshake_ok"}) // Should not increment anything

	if ss.AuthFailures.Load() != 2 {
		t.Errorf("AuthFailures = %d, want 2", ss.AuthFailures.Load())
	}
	if ss.RateLimitHits.Load() != 1 {
		t.Errorf("RateLimitHits = %d, want 1", ss.RateLimitHits.Load())
	}
	if ss.RejectedConns.Load() != 1 {
		t.Errorf("RejectedConns = %d, want 1", ss.RejectedConns.Load())
	}
	if ss.MalformedMsgs.Load() != 1 {
		t.Errorf("MalformedMsgs = %d, want 1", ss.MalformedMsgs.Load())
	}
	if ss.UnauthorizedMsgs.Load() != 1 {
		t.Errorf("UnauthorizedMsgs = %d, want 1", ss.UnauthorizedMsgs.Load())
	}
}

func TestFormatTimeOrEmpty(t *testing.T) {
	if got := formatTimeOrEmpty(time.Time{}); got != "" {
		t.Errorf("zero time = %q, want empty", got)
	}
	now := time.Now()
	if got := formatTimeOrEmpty(now); got == "" {
		t.Error("non-zero time should not be empty")
	}
}
