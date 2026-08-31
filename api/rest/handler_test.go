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

// TestCORSAllowlist covers the origin allowlist that replaced the previous
// Access-Control-Allow-Origin: * wildcard.
func TestCORSAllowlist(t *testing.T) {
	logger := slog.Default()
	cfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	session := hsms.NewSession(cfg, logger)
	handler := gem.NewHandler(session, 1, "TEST-EQ", "1.0.0", logger)

	tests := []struct {
		name          string
		allowed       []string
		origin        string
		method        string
		wantStatus    int
		wantACAOrigin string
	}{
		{"preflight allowlisted origin", []string{"https://ui.example.com"}, "https://ui.example.com", "OPTIONS", 204, "https://ui.example.com"},
		{"preflight foreign origin", []string{"https://ui.example.com"}, "https://evil.example.com", "OPTIONS", 403, ""},
		{"preflight no allowlist", nil, "https://ui.example.com", "OPTIONS", 403, ""},
		{"preflight no origin header", []string{"https://ui.example.com"}, "", "OPTIONS", 403, ""},
		{"GET allowlisted origin echoes it", []string{"https://ui.example.com"}, "https://ui.example.com", "GET", 200, "https://ui.example.com"},
		{"GET foreign origin gets no header", []string{"https://ui.example.com"}, "https://evil.example.com", "GET", 200, ""},
		{"GET no allowlist gets no header", nil, "https://ui.example.com", "GET", 200, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := NewServerWithAuth(session, handler, logger, Auth{}, CORS{AllowedOrigins: tc.allowed})
			req := httptest.NewRequest(tc.method, "/api/status", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tc.wantStatus)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != tc.wantACAOrigin {
				t.Errorf("Access-Control-Allow-Origin: got %q, want %q", got, tc.wantACAOrigin)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got == "*" {
				t.Error("wildcard CORS origin must never be emitted")
			}
		})
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

// --- Bearer Token Auth Tests ---

func setupAuthServer(t *testing.T) *Server {
	t.Helper()
	logger := slog.Default()
	cfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	session := hsms.NewSession(cfg, logger)
	handler := gem.NewHandler(session, 1, "TEST-EQ", "1.0.0", logger)
	handler.Variables().DefineSV(&gem.StatusVariable{
		SVID: 1001, Name: "WaferCount", Value: uint32(42), Units: "pcs",
	})
	return NewServer(session, handler, logger, "test-secret-token")
}

func doAuthRequest(t *testing.T, srv *Server, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	return w
}

func TestAuthHealthPublic(t *testing.T) {
	srv := setupAuthServer(t)
	// Health should be accessible without token
	w := doAuthRequest(t, srv, "GET", "/health", "")
	if w.Code != 200 {
		t.Errorf("health without token: status %d, want 200", w.Code)
	}
}

func TestAuthRequiredForAPI(t *testing.T) {
	srv := setupAuthServer(t)
	// API without token should return 401
	w := doAuthRequest(t, srv, "GET", "/api/sv", "")
	if w.Code != 401 {
		t.Errorf("API without token: status %d, want 401", w.Code)
	}
}

func TestAuthWrongToken(t *testing.T) {
	srv := setupAuthServer(t)
	w := doAuthRequest(t, srv, "GET", "/api/sv", "wrong-token")
	if w.Code != 401 {
		t.Errorf("API with wrong token: status %d, want 401", w.Code)
	}
}

func TestAuthCorrectToken(t *testing.T) {
	srv := setupAuthServer(t)
	w := doAuthRequest(t, srv, "GET", "/api/sv", "test-secret-token")
	if w.Code != 200 {
		t.Errorf("API with correct token: status %d, want 200", w.Code)
	}
}

func TestAuthStatusEndpoint(t *testing.T) {
	srv := setupAuthServer(t)
	w := doAuthRequest(t, srv, "GET", "/api/status", "test-secret-token")
	if w.Code != 200 {
		t.Errorf("status with token: status %d, want 200", w.Code)
	}
}

// --- Read/write scope separation ---

func setupScopedServer(t *testing.T, auth Auth) *Server {
	t.Helper()
	logger := slog.Default()
	cfg := hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1)
	session := hsms.NewSession(cfg, logger)
	handler := gem.NewHandler(session, 1, "TEST-EQ", "1.0.0", logger)
	handler.Variables().DefineEC(&gem.EquipmentConstant{
		ECID: 1, Name: "Temperature", Value: float64(350.0), Units: "C",
	})
	handler.Variables().DefineSV(&gem.StatusVariable{
		SVID: 1001, Name: "WaferCount", Value: uint32(42), Units: "pcs",
	})
	handler.Commands().Register("START", func(ctx context.Context, params []gem.CommandParam) gem.CommandStatus {
		return gem.CommandOK
	})
	return NewServerWithAuth(session, handler, logger, auth, CORS{})
}

func doScopedRequest(t *testing.T, srv *Server, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	return w
}

// TestAuthScopeSeparation checks that a read token cannot reach the endpoints
// that change equipment state, and that a read-only deployment closes them
// entirely.
func TestAuthScopeSeparation(t *testing.T) {
	const (
		readTok  = "read-token-aaaa"
		writeTok = "write-token-bbbb"
	)

	both := Auth{ReadToken: readTok, WriteToken: writeTok}
	readOnly := Auth{ReadToken: readTok}

	tests := []struct {
		name   string
		auth   Auth
		method string
		path   string
		body   string
		token  string
		want   int
	}{
		// Health stays public whatever is configured.
		{"health needs no token", both, "GET", "/health", "", "", 200},

		// Read endpoints.
		{"read endpoint with read token", both, "GET", "/api/sv", "", readTok, 200},
		{"read endpoint with write token", both, "GET", "/api/sv", "", writeTok, 200},
		{"read endpoint without token", both, "GET", "/api/sv", "", "", 401},
		{"read endpoint with wrong token", both, "GET", "/api/sv", "", "nope", 401},
		{"alarms with read token", both, "GET", "/api/alarms", "", readTok, 200},
		{"security status with read token", both, "GET", "/api/security/status", "", readTok, 200},

		// PUT /api/ec is a write.
		{"set EC with write token", both, "PUT", "/api/ec/1", `{"value":400}`, writeTok, 200},
		{"set EC with read token is forbidden", both, "PUT", "/api/ec/1", `{"value":400}`, readTok, 403},
		{"set EC without token", both, "PUT", "/api/ec/1", `{"value":400}`, "", 401},
		{"set EC with wrong token", both, "PUT", "/api/ec/1", `{"value":400}`, "nope", 403},

		// POST /api/command is a write.
		{"command with write token", both, "POST", "/api/command", `{"command":"START"}`, writeTok, 200},
		{"command with read token is forbidden", both, "POST", "/api/command", `{"command":"START"}`, readTok, 403},
		{"command without token", both, "POST", "/api/command", `{"command":"START"}`, "", 401},

		// A deployment with no write token has no credential that opens the
		// write endpoints.
		{"read-only deployment still serves reads", readOnly, "GET", "/api/sv", "", readTok, 200},
		{"read-only deployment closes set EC", readOnly, "PUT", "/api/ec/1", `{"value":400}`, readTok, 403},
		{"read-only deployment closes command", readOnly, "POST", "/api/command", `{"command":"START"}`, readTok, 403},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := setupScopedServer(t, tc.auth)
			w := doScopedRequest(t, srv, tc.method, tc.path, tc.token, tc.body)
			if w.Code != tc.want {
				t.Errorf("%s %s: status %d, want %d (body: %s)",
					tc.method, tc.path, w.Code, tc.want, w.Body.String())
			}
		})
	}
}

// TestSingleTokenGrantsBothScopes pins the documented behaviour of the
// backwards-compatible NewServer signature.
func TestSingleTokenGrantsBothScopes(t *testing.T) {
	srv := setupScopedServer(t, Auth{ReadToken: "one-token", WriteToken: "one-token"})

	if w := doScopedRequest(t, srv, "GET", "/api/sv", "one-token", ""); w.Code != 200 {
		t.Errorf("read with single token: %d, want 200", w.Code)
	}
	if w := doScopedRequest(t, srv, "PUT", "/api/ec/1", "one-token", `{"value":400}`); w.Code != 200 {
		t.Errorf("write with single token: %d, want 200", w.Code)
	}
}

// TestAuthDisabledWhenNoTokens documents that a server with no tokens serves
// unauthenticated; the binary refuses that combination on a non-loopback bind
// (see security.RequireTokenForListen).
func TestAuthDisabledWhenNoTokens(t *testing.T) {
	srv := setupScopedServer(t, Auth{})
	if w := doScopedRequest(t, srv, "GET", "/api/sv", "", ""); w.Code != 200 {
		t.Errorf("unauthenticated read: %d, want 200", w.Code)
	}
}
