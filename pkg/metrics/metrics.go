// Package metrics provides Prometheus-compatible metrics for the SECS/GEM driver.
// Exposes connection stats, message rates, error counts, and GEM state info.
package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Collector aggregates runtime metrics for the SECS/GEM driver.
type Collector struct {
	startTime time.Time

	// Connection metrics
	ConnectionsTotal   atomic.Int64 // Total connections (cumulative)
	ConnectionsActive  atomic.Int64 // Currently active connections
	ConnectionsFailed  atomic.Int64 // Failed connection attempts
	ReconnectsTotal    atomic.Int64 // Total reconnection attempts

	// Message metrics
	MessagesReceived atomic.Int64
	MessagesSent     atomic.Int64
	MessagesDropped  atomic.Int64 // Dropped due to rate limiting or buffer full

	// Error metrics
	DecodeErrors     atomic.Int64
	TLSHandshakeFail atomic.Int64
	AuthFailures     atomic.Int64
	RateLimited      atomic.Int64

	// Alarm metrics
	AlarmsActive atomic.Int64
	AlarmsTotal  atomic.Int64

	// GEM state (string, protected by mutex)
	mu           sync.RWMutex
	commState    string
	controlState string
	equipment    string
}

// NewCollector creates a new metrics collector.
func NewCollector(equipment string) *Collector {
	return &Collector{
		startTime:    time.Now(),
		commState:    "DISABLED",
		controlState: "OFFLINE/EQUIPMENT",
		equipment:    equipment,
	}
}

// SetGEMState updates the GEM state labels.
func (c *Collector) SetGEMState(comm, control string) {
	c.mu.Lock()
	c.commState = comm
	c.controlState = control
	c.mu.Unlock()
}

// Handler returns an HTTP handler that serves Prometheus text format metrics.
func (c *Collector) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.mu.RLock()
		comm := c.commState
		control := c.controlState
		equip := c.equipment
		c.mu.RUnlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		uptime := time.Since(c.startTime).Seconds()

		fmt.Fprintf(w, "# HELP secsgem_uptime_seconds Time since daemon start.\n")
		fmt.Fprintf(w, "# TYPE secsgem_uptime_seconds gauge\n")
		fmt.Fprintf(w, "secsgem_uptime_seconds{equipment=%q} %.1f\n", equip, uptime)

		// Connection metrics
		fmt.Fprintf(w, "# HELP secsgem_connections_total Total connections (cumulative).\n")
		fmt.Fprintf(w, "# TYPE secsgem_connections_total counter\n")
		fmt.Fprintf(w, "secsgem_connections_total{equipment=%q} %d\n", equip, c.ConnectionsTotal.Load())

		fmt.Fprintf(w, "# HELP secsgem_connections_active Currently active connections.\n")
		fmt.Fprintf(w, "# TYPE secsgem_connections_active gauge\n")
		fmt.Fprintf(w, "secsgem_connections_active{equipment=%q} %d\n", equip, c.ConnectionsActive.Load())

		fmt.Fprintf(w, "# HELP secsgem_connections_failed Failed connection attempts.\n")
		fmt.Fprintf(w, "# TYPE secsgem_connections_failed counter\n")
		fmt.Fprintf(w, "secsgem_connections_failed{equipment=%q} %d\n", equip, c.ConnectionsFailed.Load())

		fmt.Fprintf(w, "# HELP secsgem_reconnects_total Reconnection attempts.\n")
		fmt.Fprintf(w, "# TYPE secsgem_reconnects_total counter\n")
		fmt.Fprintf(w, "secsgem_reconnects_total{equipment=%q} %d\n", equip, c.ReconnectsTotal.Load())

		// Message metrics
		fmt.Fprintf(w, "# HELP secsgem_messages_received_total Messages received.\n")
		fmt.Fprintf(w, "# TYPE secsgem_messages_received_total counter\n")
		fmt.Fprintf(w, "secsgem_messages_received_total{equipment=%q} %d\n", equip, c.MessagesReceived.Load())

		fmt.Fprintf(w, "# HELP secsgem_messages_sent_total Messages sent.\n")
		fmt.Fprintf(w, "# TYPE secsgem_messages_sent_total counter\n")
		fmt.Fprintf(w, "secsgem_messages_sent_total{equipment=%q} %d\n", equip, c.MessagesSent.Load())

		fmt.Fprintf(w, "# HELP secsgem_messages_dropped_total Messages dropped.\n")
		fmt.Fprintf(w, "# TYPE secsgem_messages_dropped_total counter\n")
		fmt.Fprintf(w, "secsgem_messages_dropped_total{equipment=%q} %d\n", equip, c.MessagesDropped.Load())

		// Error metrics
		fmt.Fprintf(w, "# HELP secsgem_decode_errors_total SECS-II decode errors.\n")
		fmt.Fprintf(w, "# TYPE secsgem_decode_errors_total counter\n")
		fmt.Fprintf(w, "secsgem_decode_errors_total{equipment=%q} %d\n", equip, c.DecodeErrors.Load())

		fmt.Fprintf(w, "# HELP secsgem_tls_handshake_failures_total TLS handshake failures.\n")
		fmt.Fprintf(w, "# TYPE secsgem_tls_handshake_failures_total counter\n")
		fmt.Fprintf(w, "secsgem_tls_handshake_failures_total{equipment=%q} %d\n", equip, c.TLSHandshakeFail.Load())

		fmt.Fprintf(w, "# HELP secsgem_auth_failures_total Authentication failures.\n")
		fmt.Fprintf(w, "# TYPE secsgem_auth_failures_total counter\n")
		fmt.Fprintf(w, "secsgem_auth_failures_total{equipment=%q} %d\n", equip, c.AuthFailures.Load())

		fmt.Fprintf(w, "# HELP secsgem_rate_limited_total Messages rate limited.\n")
		fmt.Fprintf(w, "# TYPE secsgem_rate_limited_total counter\n")
		fmt.Fprintf(w, "secsgem_rate_limited_total{equipment=%q} %d\n", equip, c.RateLimited.Load())

		// Alarm metrics
		fmt.Fprintf(w, "# HELP secsgem_alarms_active Currently active alarms.\n")
		fmt.Fprintf(w, "# TYPE secsgem_alarms_active gauge\n")
		fmt.Fprintf(w, "secsgem_alarms_active{equipment=%q} %d\n", equip, c.AlarmsActive.Load())

		fmt.Fprintf(w, "# HELP secsgem_alarms_total Total alarms triggered.\n")
		fmt.Fprintf(w, "# TYPE secsgem_alarms_total counter\n")
		fmt.Fprintf(w, "secsgem_alarms_total{equipment=%q} %d\n", equip, c.AlarmsTotal.Load())

		// GEM state
		fmt.Fprintf(w, "# HELP secsgem_gem_info GEM state information.\n")
		fmt.Fprintf(w, "# TYPE secsgem_gem_info gauge\n")
		fmt.Fprintf(w, "secsgem_gem_info{equipment=%q,comm_state=%q,control_state=%q} 1\n",
			equip, comm, control)
	})
}
