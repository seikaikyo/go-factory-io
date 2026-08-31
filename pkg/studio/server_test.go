package studio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestServer_StaticFiles(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Test index.html
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET / status: %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Error("missing Content-Type for /")
	}

	// Test CSS
	resp, err = http.Get(ts.URL + "/studio.css")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /studio.css status: %d", resp.StatusCode)
	}

	// Test JS
	resp, err = http.Get(ts.URL + "/studio.js")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /studio.js status: %d", resp.StatusCode)
	}
}

func TestServer_StatusAPI(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] != true {
		t.Error("expected success=true")
	}
}

func TestServer_ReportAPI(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/report")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/report: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] != true {
		t.Error("expected success=true")
	}
	data := result["data"].(map[string]interface{})
	if data["totalExpected"].(float64) == 0 {
		t.Error("expected non-zero totalExpected in report")
	}
}

func TestServer_TraceAPI(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/trace")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/trace: %d", resp.StatusCode)
	}
}

// --- WebSocket origin and command authorization ---

// dialWS opens the studio WebSocket with an optional Origin header and an
// optional ?token= credential.
func dialWS(t *testing.T, tsURL, origin, token string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(tsURL, "http") + "/ws"
	if token != "" {
		wsURL += "?token=" + url.QueryEscape(token)
	}
	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	if origin != "" {
		opts.HTTPHeader.Set("Origin", origin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return websocket.Dial(ctx, wsURL, opts)
}

// TestWSOriginCheck covers the removal of InsecureSkipVerify: a page on
// another origin must not be able to open the control socket.
func TestWSOriginCheck(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	t.Run("foreign origin rejected", func(t *testing.T) {
		c, _, err := dialWS(t, ts.URL, "https://evil.example.com", "")
		if err == nil {
			c.Close(websocket.StatusNormalClosure, "")
			t.Fatal("cross-origin dial succeeded; the origin check is not enforced")
		}
	})

	t.Run("same origin accepted", func(t *testing.T) {
		c, _, err := dialWS(t, ts.URL, ts.URL, "")
		if err != nil {
			t.Fatalf("same-origin dial: %v", err)
		}
		c.Close(websocket.StatusNormalClosure, "")
	})

	t.Run("no origin header accepted", func(t *testing.T) {
		// Non-browser clients send no Origin. They are still gated by the
		// token when one is configured.
		c, _, err := dialWS(t, ts.URL, "", "")
		if err != nil {
			t.Fatalf("dial without Origin: %v", err)
		}
		c.Close(websocket.StatusNormalClosure, "")
	})
}

// TestWSOriginAllowlist checks that a configured origin is admitted and
// others are still refused.
func TestWSOriginAllowlist(t *testing.T) {
	srv := NewServer(Config{SessionID: 1, AllowedOrigins: []string{"https://ui.example.com"}}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	c, _, err := dialWS(t, ts.URL, "https://ui.example.com", "")
	if err != nil {
		t.Fatalf("allowlisted origin should connect: %v", err)
	}
	c.Close(websocket.StatusNormalClosure, "")

	if c2, _, err := dialWS(t, ts.URL, "https://evil.example.com", ""); err == nil {
		c2.Close(websocket.StatusNormalClosure, "")
		t.Fatal("origin outside the allowlist connected")
	}
}

// TestWSTokenRequired checks that a configured token gates the upgrade itself.
func TestWSTokenRequired(t *testing.T) {
	srv := NewServer(Config{SessionID: 1, Token: "studio-secret"}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	tests := []struct {
		name     string
		token    string
		wantDial bool
	}{
		{"no token", "", false},
		{"wrong token", "guess", false},
		{"correct token", "studio-secret", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _, err := dialWS(t, ts.URL, ts.URL, tc.token)
			if tc.wantDial {
				if err != nil {
					t.Fatalf("dial should succeed: %v", err)
				}
				c.Close(websocket.StatusNormalClosure, "")
				return
			}
			if err == nil {
				c.Close(websocket.StatusNormalClosure, "")
				t.Fatal("dial should have been rejected")
			}
		})
	}
}

// TestWSCommandAuthorization covers the fail-closed default: without a token
// and without a loopback-only bind, the commands that drive the equipment are
// refused, while read-only commands still work.
func TestWSCommandAuthorization(t *testing.T) {
	srv := NewServer(Config{SessionID: 1}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	mutating := []string{"send", "quick_send", "fault", "run_script"}
	for _, cmd := range mutating {
		t.Run(cmd+" refused", func(t *testing.T) {
			c, _, err := dialWS(t, ts.URL, ts.URL, "")
			if err != nil {
				t.Fatalf("dial: %v", err)
			}
			defer c.Close(websocket.StatusNormalClosure, "")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Drain the status frame the server sends on connect.
			if _, _, err := c.Read(ctx); err != nil {
				t.Fatalf("read status: %v", err)
			}

			payload, _ := json.Marshal(WSMessage{Type: cmd, Data: json.RawMessage(`{}`)})
			if err := c.Write(ctx, websocket.MessageText, payload); err != nil {
				t.Fatalf("write: %v", err)
			}

			_, data, err := c.Read(ctx)
			if err != nil {
				t.Fatalf("read reply: %v", err)
			}
			var reply WSMessage
			if err := json.Unmarshal(data, &reply); err != nil {
				t.Fatalf("decode reply: %v", err)
			}
			if reply.Type != "error" {
				t.Fatalf("reply type %q, want error", reply.Type)
			}
			if !strings.Contains(string(reply.Data), "requires the studio token") {
				t.Errorf("reply should explain the refusal, got: %s", reply.Data)
			}
		})
	}

	t.Run("read-only command still served", func(t *testing.T) {
		c, _, err := dialWS(t, ts.URL, ts.URL, "")
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer c.Close(websocket.StatusNormalClosure, "")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, _, err := c.Read(ctx); err != nil {
			t.Fatalf("read status: %v", err)
		}

		payload, _ := json.Marshal(WSMessage{Type: "get_state"})
		if err := c.Write(ctx, websocket.MessageText, payload); err != nil {
			t.Fatalf("write: %v", err)
		}

		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("read reply: %v", err)
		}
		var reply WSMessage
		if err := json.Unmarshal(data, &reply); err != nil {
			t.Fatalf("decode reply: %v", err)
		}
		if reply.Type != "status" {
			t.Errorf("reply type %q, want status", reply.Type)
		}
	})
}

// TestLoopbackOnlyAllowsControl documents the local-development exemption: a
// loopback-bound studio is unreachable from the network, so the commands stay
// available without a token.
func TestLoopbackOnlyAllowsControl(t *testing.T) {
	srv := NewServer(Config{SessionID: 1, LoopbackOnly: true}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	c, _, err := dialWS(t, ts.URL, ts.URL, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, _, err := c.Read(ctx); err != nil {
		t.Fatalf("read status: %v", err)
	}

	payload, _ := json.Marshal(WSMessage{Type: "quick_send", Data: json.RawMessage(`{"name":"are_you_there"}`)})
	if err := c.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	var reply WSMessage
	if err := json.Unmarshal(data, &reply); err != nil {
		t.Fatalf("decode reply: %v", err)
	}
	// No host is connected in this fixture, so the command reports that
	// rather than a token refusal. The point is that it was not gated.
	if strings.Contains(string(reply.Data), "requires the studio token") {
		t.Error("loopback-only studio should not gate control commands")
	}
}

// TestAPITokenRequired covers the token gate on the studio REST routes.
func TestAPITokenRequired(t *testing.T) {
	srv := NewServer(Config{SessionID: 1, Token: "studio-secret"}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	tests := []struct {
		name   string
		header string
		query  string
		want   int
	}{
		{"no credential", "", "", 401},
		{"wrong bearer", "Bearer nope", "", 401},
		{"correct bearer", "Bearer studio-secret", "", 200},
		{"correct query token", "", "?token=studio-secret", 200},
		{"wrong query token", "", "?token=nope", 401},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", ts.URL+"/api/status"+tc.query, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Errorf("status %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}

	// Health stays public for the container probe.
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("health status %d, want 200", resp.StatusCode)
	}
}

// TestStudioCORSNoWildcard pins that the studio never emits a wildcard origin.
func TestStudioCORSNoWildcard(t *testing.T) {
	srv := NewServer(Config{SessionID: 1, AllowedOrigins: []string{"https://ui.example.com"}}, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	tests := []struct {
		origin string
		want   string
	}{
		{"https://ui.example.com", "https://ui.example.com"},
		{"https://evil.example.com", ""},
		{"", ""},
	}

	for _, tc := range tests {
		req, _ := http.NewRequest("GET", ts.URL+"/api/status", nil)
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != tc.want {
			t.Errorf("origin %q: Access-Control-Allow-Origin = %q, want %q", tc.origin, got, tc.want)
		}
	}
}
