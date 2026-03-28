package security

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestWebhookImmediate(t *testing.T) {
	var received []Event
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %s, want application/json", ct)
		}

		var events []Event
		json.NewDecoder(r.Body).Decode(&events)
		mu.Lock()
		received = append(received, events...)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	handler := NewWebhookHandler(WebhookConfig{
		URL:        srv.URL,
		Timeout:    2 * time.Second,
		RetryCount: 1,
		RetryDelay: 10 * time.Millisecond,
	}, nil)

	handler(Event{
		Time:     time.Now(),
		Level:    LevelWarning,
		Category: CatAuth,
		Type:     "auth_failed",
		Source:   "192.168.1.1",
		Message:  "test event",
	})

	// Wait for async send
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("received %d events, want 1", len(received))
	}
	if received[0].Type != "auth_failed" {
		t.Errorf("event type = %s, want auth_failed", received[0].Type)
	}
}

func TestWebhookCustomHeaders(t *testing.T) {
	var gotAuth string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	handler := NewWebhookHandler(WebhookConfig{
		URL:     srv.URL,
		Headers: map[string]string{"Authorization": "Bearer test-token"},
	}, nil)

	handler(Event{Time: time.Now(), Level: LevelInfo, Type: "test"})
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %s, want Bearer test-token", gotAuth)
	}
}

func TestWebhookRetryOn5xx(t *testing.T) {
	var attempts int
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	handler := NewWebhookHandler(WebhookConfig{
		URL:        srv.URL,
		RetryCount: 3,
		RetryDelay: 10 * time.Millisecond,
	}, nil)

	handler(Event{Time: time.Now(), Level: LevelInfo, Type: "test"})
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if attempts < 3 {
		t.Errorf("attempts = %d, want >= 3", attempts)
	}
}

func TestWebhookNoRetryOn4xx(t *testing.T) {
	var attempts int
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()
		w.WriteHeader(400)
	}))
	defer srv.Close()

	handler := NewWebhookHandler(WebhookConfig{
		URL:        srv.URL,
		RetryCount: 3,
		RetryDelay: 10 * time.Millisecond,
	}, nil)

	handler(Event{Time: time.Now(), Level: LevelInfo, Type: "test"})
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 4xx)", attempts)
	}
}

func TestSyslogSeverityMapping(t *testing.T) {
	tests := []struct {
		level Level
		want  int
	}{
		{LevelInfo, 6},
		{LevelWarning, 4},
		{LevelCritical, 2},
	}
	for _, tt := range tests {
		got := syslogSeverity(tt.level)
		if got != tt.want {
			t.Errorf("syslogSeverity(%v) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestNewSyslogHandlerDefaults(t *testing.T) {
	handler, err := NewSyslogHandler(SyslogConfig{Address: "localhost:514"}, nil)
	if err != nil {
		t.Fatalf("NewSyslogHandler: %v", err)
	}
	if handler == nil {
		t.Fatal("handler is nil")
	}
}
