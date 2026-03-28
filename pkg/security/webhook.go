package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// WebhookConfig configures an HTTP webhook for security event forwarding.
type WebhookConfig struct {
	URL         string            // HTTP POST endpoint
	Headers     map[string]string // Extra headers (e.g., Authorization)
	Timeout     time.Duration     // Per-request timeout (default 10s)
	RetryCount  int               // Max retries on failure (default 3)
	RetryDelay  time.Duration     // Initial retry delay, doubles each retry (default 1s)
	BatchSize   int               // Batch N events per request; 0 = send immediately
	BatchWindow time.Duration     // Max time to wait before flushing batch (default 5s)
}

// webhookHandler sends security events as JSON to an HTTP endpoint.
type webhookHandler struct {
	config WebhookConfig
	logger *slog.Logger
	client *http.Client

	// Batching
	mu    sync.Mutex
	batch []Event
	timer *time.Timer
}

// NewWebhookHandler creates an EventHandler that POSTs security events to a webhook URL.
func NewWebhookHandler(cfg WebhookConfig, logger *slog.Logger) EventHandler {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.RetryCount == 0 {
		cfg.RetryCount = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 1 * time.Second
	}
	if cfg.BatchWindow == 0 {
		cfg.BatchWindow = 5 * time.Second
	}

	wh := &webhookHandler{
		config: cfg,
		logger: logger,
		client: &http.Client{Timeout: cfg.Timeout},
	}

	return wh.handle
}

func (wh *webhookHandler) handle(event Event) {
	if wh.config.BatchSize <= 0 {
		go wh.send([]Event{event})
		return
	}

	wh.mu.Lock()
	wh.batch = append(wh.batch, event)

	if len(wh.batch) >= wh.config.BatchSize {
		events := wh.batch
		wh.batch = nil
		if wh.timer != nil {
			wh.timer.Stop()
			wh.timer = nil
		}
		wh.mu.Unlock()
		go wh.send(events)
		return
	}

	if wh.timer == nil {
		wh.timer = time.AfterFunc(wh.config.BatchWindow, wh.flush)
	}
	wh.mu.Unlock()
}

func (wh *webhookHandler) flush() {
	wh.mu.Lock()
	if len(wh.batch) == 0 {
		wh.timer = nil
		wh.mu.Unlock()
		return
	}
	events := wh.batch
	wh.batch = nil
	wh.timer = nil
	wh.mu.Unlock()
	wh.send(events)
}

func (wh *webhookHandler) send(events []Event) {
	body, err := json.Marshal(events)
	if err != nil {
		wh.logger.Error("webhook: marshal events", "error", err)
		return
	}

	delay := wh.config.RetryDelay
	for attempt := 0; attempt <= wh.config.RetryCount; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
			delay *= 2
		}

		req, err := http.NewRequest(http.MethodPost, wh.config.URL, bytes.NewReader(body))
		if err != nil {
			wh.logger.Error("webhook: create request", "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range wh.config.Headers {
			req.Header.Set(k, v)
		}

		resp, err := wh.client.Do(req)
		if err != nil {
			wh.logger.Warn("webhook: request failed", "attempt", attempt+1, "error", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}

		if resp.StatusCode >= 500 {
			wh.logger.Warn("webhook: server error", "attempt", attempt+1, "status", resp.StatusCode)
			continue
		}

		// 4xx = client error, don't retry
		wh.logger.Error("webhook: client error", "status", resp.StatusCode, "url", wh.config.URL)
		return
	}

	wh.logger.Error("webhook: all retries exhausted", "url", wh.config.URL, "events", len(events))
}

// SyslogConfig configures a syslog sink for security events.
type SyslogConfig struct {
	Network string // "tcp" or "udp"
	Address string // "syslog.example.com:514"
	Tag     string // Syslog tag (default "secsgem")
}

// syslogHandler sends security events to a remote syslog server via RFC 5424.
type syslogHandler struct {
	config SyslogConfig
	logger *slog.Logger
	mu     sync.Mutex
	conn   net.Conn
}

// NewSyslogHandler creates an EventHandler that forwards security events to syslog.
func NewSyslogHandler(cfg SyslogConfig, logger *slog.Logger) (EventHandler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Tag == "" {
		cfg.Tag = "secsgem"
	}
	if cfg.Network == "" {
		cfg.Network = "udp"
	}

	sh := &syslogHandler{
		config: cfg,
		logger: logger,
	}

	return sh.handle, nil
}

func (sh *syslogHandler) handle(event Event) {
	severity := syslogSeverity(event.Level)
	facility := 1 // user-level
	priority := facility*8 + severity

	msg := fmt.Sprintf("<%d>1 %s %s %s - - - [category=%s type=%s] %s",
		priority,
		event.Time.UTC().Format(time.RFC3339),
		"-",
		sh.config.Tag,
		event.Category,
		event.Type,
		event.Message,
	)

	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.conn == nil {
		conn, err := net.DialTimeout(sh.config.Network, sh.config.Address, 5*time.Second)
		if err != nil {
			sh.logger.Error("syslog: connect failed", "error", err)
			return
		}
		sh.conn = conn
	}

	if _, err := sh.conn.Write([]byte(msg + "\n")); err != nil {
		sh.logger.Error("syslog: write failed", "error", err)
		sh.conn.Close()
		sh.conn = nil
	}
}

func syslogSeverity(level Level) int {
	switch level {
	case LevelCritical:
		return 2 // critical
	case LevelWarning:
		return 4 // warning
	default:
		return 6 // informational
	}
}
