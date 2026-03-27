package rest

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/driver/gem"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

func setupTestServer(t *testing.T) (*Server, *gem.Handler) {
	t.Helper()
	logger := slog.Default()

	// Create a minimal session (not connected — API tests don't need real TCP)
	cfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	session := hsms.NewSession(cfg, logger)
	handler := gem.NewHandler(session, 1, "TEST-EQ", "1.0.0", logger)

	// Register test data
	handler.Variables().DefineEC(&gem.EquipmentConstant{
		ECID: 1, Name: "Temperature", Value: float64(350.0), Units: "C",
	})
	handler.Variables().DefineSV(&gem.StatusVariable{
		SVID: 1001, Name: "WaferCount", Value: uint32(42), Units: "pcs",
	})
	handler.Alarms().DefineAlarm(&gem.Alarm{
		ALID: 1, Name: "OverTemp", Text: "Temperature exceeded limit", Enabled: true,
	})
	handler.Commands().Register("START", func(ctx context.Context, params []gem.CommandParam) gem.CommandStatus {
		return gem.CommandOK
	})

	srv := NewServer(session, handler, logger)
	return srv, handler
}

func doRequest(t *testing.T, srv *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	return w
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("parse JSON: %v\nbody: %s", err, w.Body.String())
	}
	return result
}

func TestHealthEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "GET", "/health", "")

	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	result := parseResponse(t, w)
	if result["success"] != true {
		t.Errorf("success: %v", result["success"])
	}
}

func TestStatusEndpoint(t *testing.T) {
	srv, handler := setupTestServer(t)

	// Set up communicating state
	handler.State().EnableComm()
	handler.State().CommEstablished()
	handler.State().GoOnlineRemote()

	w := doRequest(t, srv, "GET", "/api/status", "")
	result := parseResponse(t, w)
	data := result["data"].(map[string]interface{})

	if data["communicating"] != true {
		t.Errorf("communicating: %v", data["communicating"])
	}
	if data["online"] != true {
		t.Errorf("online: %v", data["online"])
	}
	if data["controlState"] != "ONLINE/REMOTE" {
		t.Errorf("controlState: %v", data["controlState"])
	}
}

func TestListSV(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "GET", "/api/sv", "")

	result := parseResponse(t, w)
	data := result["data"].([]interface{})

	if len(data) != 1 {
		t.Fatalf("expected 1 SV, got %d", len(data))
	}

	sv := data[0].(map[string]interface{})
	if sv["name"] != "WaferCount" {
		t.Errorf("name: %v", sv["name"])
	}
}

func TestGetSV(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "GET", "/api/sv/1001", "")

	result := parseResponse(t, w)
	data := result["data"].(map[string]interface{})

	if data["name"] != "WaferCount" {
		t.Errorf("name: %v", data["name"])
	}
}

func TestGetSVNotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "GET", "/api/sv/9999", "")

	if w.Code != 404 {
		t.Errorf("status: %d, want 404", w.Code)
	}
}

func TestListEC(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "GET", "/api/ec", "")

	result := parseResponse(t, w)
	data := result["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("expected 1 EC, got %d", len(data))
	}

	ec := data[0].(map[string]interface{})
	if ec["name"] != "Temperature" {
		t.Errorf("name: %v", ec["name"])
	}
}

func TestGetEC(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "GET", "/api/ec/1", "")

	result := parseResponse(t, w)
	data := result["data"].(map[string]interface{})
	if data["name"] != "Temperature" {
		t.Errorf("name: %v", data["name"])
	}
}

func TestSetEC(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "PUT", "/api/ec/1", `{"value": 400.0}`)

	if w.Code != 200 {
		t.Fatalf("status: %d, body: %s", w.Code, w.Body.String())
	}
	result := parseResponse(t, w)
	if result["success"] != true {
		t.Errorf("success: %v", result["success"])
	}

	// Verify it was set
	w2 := doRequest(t, srv, "GET", "/api/ec/1", "")
	result2 := parseResponse(t, w2)
	data := result2["data"].(map[string]interface{})
	if data["value"] != 400.0 {
		t.Errorf("value: %v, want 400.0", data["value"])
	}
}

func TestListAlarms(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "GET", "/api/alarms", "")

	result := parseResponse(t, w)
	data := result["data"].([]interface{})
	if len(data) != 1 {
		t.Fatalf("expected 1 alarm, got %d", len(data))
	}
}

func TestActiveAlarms(t *testing.T) {
	srv, handler := setupTestServer(t)

	// No active alarms initially
	w := doRequest(t, srv, "GET", "/api/alarms/active", "")
	result := parseResponse(t, w)
	data := result["data"].([]interface{})
	if len(data) != 0 {
		t.Fatalf("expected 0 active alarms, got %d", len(data))
	}

	// Set alarm
	handler.Alarms().SetAlarm(1)
	w2 := doRequest(t, srv, "GET", "/api/alarms/active", "")
	result2 := parseResponse(t, w2)
	data2 := result2["data"].([]interface{})
	if len(data2) != 1 {
		t.Fatalf("expected 1 active alarm, got %d", len(data2))
	}
}

func TestCommand(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "POST", "/api/command", `{"command": "START", "params": {}}`)

	result := parseResponse(t, w)
	data := result["data"].(map[string]interface{})
	if data["status"] != "OK" {
		t.Errorf("status: %v", data["status"])
	}
}

func TestCommandNotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "POST", "/api/command", `{"command": "NONEXISTENT"}`)

	result := parseResponse(t, w)
	data := result["data"].(map[string]interface{})
	if data["status"] != "INVALID_COMMAND" {
		t.Errorf("status: %v", data["status"])
	}
}

func TestSSEBroadcast(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Subscribe
	ch := make(chan EventPayload, 8)
	srv.sseClientsMu.Lock()
	srv.sseClients[ch] = struct{}{}
	srv.sseClientsMu.Unlock()

	// Broadcast
	srv.BroadcastEvent("alarm", map[string]interface{}{"alid": 1, "state": "SET"})

	select {
	case event := <-ch:
		if event.Type != "alarm" {
			t.Errorf("event type: %v", event.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}

	srv.sseClientsMu.Lock()
	delete(srv.sseClients, ch)
	srv.sseClientsMu.Unlock()
}

func TestCORSHeaders(t *testing.T) {
	srv, _ := setupTestServer(t)
	w := doRequest(t, srv, "OPTIONS", "/api/status", "")

	if w.Code != 204 {
		t.Errorf("OPTIONS status: %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

func TestResponseFormat(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Success response
	w := doRequest(t, srv, "GET", "/health", "")
	result := parseResponse(t, w)
	if _, ok := result["success"]; !ok {
		t.Error("missing 'success' field")
	}
	if _, ok := result["data"]; !ok {
		t.Error("missing 'data' field")
	}

	// Error response
	w2 := doRequest(t, srv, "GET", "/api/sv/abc", "")
	result2 := parseResponse(t, w2)
	if result2["success"] != false {
		t.Error("expected success=false for error")
	}
	if _, ok := result2["error"]; !ok {
		t.Error("missing 'error' field")
	}
}
