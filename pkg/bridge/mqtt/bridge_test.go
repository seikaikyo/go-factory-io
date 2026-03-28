package mqtt

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("tcp://localhost:1883", "factory/eq01")
	if cfg.BrokerURL != "tcp://localhost:1883" {
		t.Errorf("BrokerURL = %s, want tcp://localhost:1883", cfg.BrokerURL)
	}
	if cfg.TopicPrefix != "factory/eq01" {
		t.Errorf("TopicPrefix = %s, want factory/eq01", cfg.TopicPrefix)
	}
	if cfg.QoS != 1 {
		t.Errorf("QoS = %d, want 1", cfg.QoS)
	}
	if !cfg.AutoReconnect {
		t.Error("AutoReconnect should be true by default")
	}
	if cfg.KeepAlive != 30*time.Second {
		t.Errorf("KeepAlive = %v, want 30s", cfg.KeepAlive)
	}
}

func TestEventPayloadJSON(t *testing.T) {
	p := EventPayload{
		Type:      "alarm",
		Timestamp: "2026-03-28T12:00:00Z",
		Data:      map[string]interface{}{"alid": float64(1), "state": "set"},
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded EventPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "alarm" {
		t.Errorf("Type = %s, want alarm", decoded.Type)
	}
	if decoded.Timestamp != "2026-03-28T12:00:00Z" {
		t.Errorf("Timestamp = %s, want 2026-03-28T12:00:00Z", decoded.Timestamp)
	}
}

func TestNewBridgeDefaults(t *testing.T) {
	cfg := Config{BrokerURL: "tcp://localhost:1883", TopicPrefix: "test"}
	b := NewBridge(cfg, nil)
	if b.config.KeepAlive != 30*time.Second {
		t.Errorf("KeepAlive = %v, want 30s", b.config.KeepAlive)
	}
}

func TestPublishNotConnected(t *testing.T) {
	b := NewBridge(Config{TopicPrefix: "test"}, nil)
	err := b.PublishStatus(map[string]string{"state": "ok"})
	if err == nil {
		t.Error("expected error when not connected")
	}
}
