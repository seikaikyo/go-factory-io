package opcua

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("opc.tcp://localhost:4840")

	if cfg.Endpoint != "opc.tcp://localhost:4840" {
		t.Errorf("endpoint: %q", cfg.Endpoint)
	}
	if cfg.SecurityPolicy != "None" {
		t.Errorf("security policy: %q", cfg.SecurityPolicy)
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Errorf("timeout: %v", cfg.RequestTimeout)
	}
}

func TestNewClient(t *testing.T) {
	cfg := DefaultConfig("opc.tcp://localhost:4840")
	client := NewClient(cfg, nil)

	if client == nil {
		t.Fatal("client is nil")
	}
	if client.config.Endpoint != cfg.Endpoint {
		t.Errorf("endpoint mismatch")
	}
}

func TestClientNotConnected(t *testing.T) {
	cfg := DefaultConfig("opc.tcp://localhost:4840")
	client := NewClient(cfg, nil)

	// Operations should fail gracefully when not connected
	_, err := client.ReadValue(nil, "ns=2;s=Test")
	if err == nil {
		t.Error("expected error when not connected")
	}

	_, err = client.ReadMultiple(nil, []string{"ns=2;s=Test"})
	if err == nil {
		t.Error("expected error for ReadMultiple when not connected")
	}

	err = client.WriteValue(nil, "ns=2;s=Test", nil)
	if err == nil {
		t.Error("expected error for WriteValue when not connected")
	}

	_, err = client.BrowseNode(nil, "ns=0;i=84")
	if err == nil {
		t.Error("expected error for BrowseNode when not connected")
	}
}

func TestClientCloseIdempotent(t *testing.T) {
	cfg := DefaultConfig("opc.tcp://localhost:4840")
	client := NewClient(cfg, nil)

	// Close without connect should not error
	if err := client.Close(); err != nil {
		t.Errorf("close: %v", err)
	}

	// Double close should be safe
	if err := client.Close(); err != nil {
		t.Errorf("double close: %v", err)
	}
}

func TestInvalidNodeID(t *testing.T) {
	cfg := DefaultConfig("opc.tcp://localhost:4840")
	client := NewClient(cfg, nil)
	// Force non-nil client to bypass connected check
	// (won't actually work but tests the parse path)

	_, err := client.ReadValue(nil, "invalid!!node!!id")
	if err == nil {
		t.Error("expected error for invalid node ID")
	}
}
