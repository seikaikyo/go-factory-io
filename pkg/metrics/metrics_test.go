package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCollectorMetrics(t *testing.T) {
	c := NewCollector("SIM-EQUIP-01")

	// Increment some counters
	c.ConnectionsTotal.Add(5)
	c.ConnectionsActive.Store(2)
	c.MessagesReceived.Add(1000)
	c.MessagesSent.Add(500)
	c.DecodeErrors.Add(3)
	c.AlarmsActive.Store(1)
	c.AlarmsTotal.Add(10)
	c.SetGEMState("COMMUNICATING", "ONLINE/REMOTE")

	// Serve metrics
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, req)

	body := w.Body.String()

	checks := []string{
		`secsgem_connections_total{equipment="SIM-EQUIP-01"} 5`,
		`secsgem_connections_active{equipment="SIM-EQUIP-01"} 2`,
		`secsgem_messages_received_total{equipment="SIM-EQUIP-01"} 1000`,
		`secsgem_messages_sent_total{equipment="SIM-EQUIP-01"} 500`,
		`secsgem_decode_errors_total{equipment="SIM-EQUIP-01"} 3`,
		`secsgem_alarms_active{equipment="SIM-EQUIP-01"} 1`,
		`secsgem_alarms_total{equipment="SIM-EQUIP-01"} 10`,
		`secsgem_gem_info{equipment="SIM-EQUIP-01",comm_state="COMMUNICATING",control_state="ONLINE/REMOTE"} 1`,
		`secsgem_uptime_seconds`,
	}

	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("missing metric: %q", check)
		}
	}

	// Content type
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type: %q", ct)
	}
}

func TestCollectorPrometheusFormat(t *testing.T) {
	c := NewCollector("EQ-1")

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, req)

	body := w.Body.String()

	// Verify Prometheus exposition format
	if !strings.Contains(body, "# HELP") {
		t.Error("missing HELP comments")
	}
	if !strings.Contains(body, "# TYPE") {
		t.Error("missing TYPE comments")
	}
	if !strings.Contains(body, "counter") {
		t.Error("missing counter type")
	}
	if !strings.Contains(body, "gauge") {
		t.Error("missing gauge type")
	}
}

func TestCollectorZeroValues(t *testing.T) {
	c := NewCollector("test")

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, req)

	body := w.Body.String()
	// All counters should be 0
	if !strings.Contains(body, `secsgem_connections_total{equipment="test"} 0`) {
		t.Error("expected 0 connections")
	}
}
