package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dashfactory/go-factory-io/api/rest"
	"github.com/dashfactory/go-factory-io/pkg/driver/gem"
	"github.com/dashfactory/go-factory-io/pkg/metrics"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

// newTestAPIHandler builds the simulate listener exactly as runSimulator
// does, around an unconnected session.
func newTestAPIHandler(t *testing.T, readToken, writeToken string) http.Handler {
	t.Helper()
	logger := slog.Default()
	session := hsms.NewSession(hsms.DefaultConfig("127.0.0.1:0", hsms.RolePassive, 1), logger)
	handler := gem.NewHandler(session, 1, "TEST-EQ", "1.0.0", logger)
	handler.Variables().DefineEC(&gem.EquipmentConstant{ECID: 1, Name: "Temperature", Value: float64(350), Units: "C"})
	handler.Commands().Register("START", func(ctx context.Context, params []gem.CommandParam) gem.CommandStatus {
		return gem.CommandOK
	})
	api := rest.NewServerWithAuth(session, handler, logger,
		rest.Auth{ReadToken: readToken, WriteToken: writeToken}, rest.CORS{})
	return newAPIHandler(api.Handler(), metrics.NewCollector("TEST-EQ").Handler(), readToken, writeToken)
}

type apiCase struct {
	name        string
	method      string
	path        string
	host        string
	contentType string
	token       string
	body        string
	want        int
}

func runAPICases(t *testing.T, h http.Handler, cases []apiCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body *strings.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			req.Host = tc.host
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Errorf("%s %s (Host %s): status %d, want %d; body %s",
					tc.method, tc.path, tc.host, w.Code, tc.want, strings.TrimSpace(w.Body.String()))
			}
		})
	}
}

// TestSimulateAPIWithoutTokens covers a loopback simulate listener with no
// tokens: reads stay open to loopback clients, the write endpoints are
// closed as the startup message says, and a non-loopback Host (a page that
// re-pointed its own name at 127.0.0.1) is refused.
func TestSimulateAPIWithoutTokens(t *testing.T) {
	h := newTestAPIHandler(t, "", "")
	const local = "127.0.0.1:8080"
	runAPICases(t, h, []apiCase{
		{"read on loopback host", "GET", "/api/ec/1", local, "", "", "", 200},
		{"read on localhost name", "GET", "/api/status", "localhost:8080", "", "", "", 200},
		{"read on ipv6 loopback", "GET", "/api/status", "[::1]:8080", "", "", "", 200},
		{"metrics on loopback host", "GET", "/metrics", local, "", "", "", 200},
		{"read on foreign host", "GET", "/api/ec/1", "rebind.example:8080", "", "", "", 403},
		{"metrics on foreign host", "GET", "/metrics", "rebind.example:8080", "", "", "", 403},
		{"set EC closed", "PUT", "/api/ec/1", local, "application/json", "", `{"value":999}`, 403},
		{"command closed", "POST", "/api/command", local, "application/json", "", `{"command":"START"}`, 403},
		{"text/plain command closed", "POST", "/api/command", local, "text/plain", "", `{"command":"START"}`, 403},
	})
}

// TestSimulateAPIWriteContentType checks that the write endpoints only take
// JSON, so a form or text/plain request a browser sends without a preflight
// is refused even when it carries no credentials at all.
func TestSimulateAPIWriteContentType(t *testing.T) {
	h := newTestAPIHandler(t, "read-tok", "write-tok")
	const host = "10.0.0.5:8080"
	runAPICases(t, h, []apiCase{
		{"json command", "POST", "/api/command", host, "application/json", "write-tok", `{"command":"START"}`, 200},
		{"json with charset", "PUT", "/api/ec/1", host, "application/json; charset=utf-8", "write-tok", `{"value":360}`, 200},
		{"text/plain command", "POST", "/api/command", host, "text/plain", "write-tok", `{"command":"START"}`, 415},
		{"form set EC", "PUT", "/api/ec/1", host, "application/x-www-form-urlencoded", "write-tok", `value=1`, 415},
		{"read with token on any host", "GET", "/api/ec/1", host, "", "read-tok", "", 200},
	})
}

// TestSimulateMetricsRequiresToken checks that /metrics sits behind the same
// token as the API once a token is configured.
func TestSimulateMetricsRequiresToken(t *testing.T) {
	h := newTestAPIHandler(t, "read-tok", "write-tok")
	const host = "10.0.0.5:8080"
	runAPICases(t, h, []apiCase{
		{"metrics without token", "GET", "/metrics", host, "", "", "", 401},
		{"metrics with wrong token", "GET", "/metrics", host, "", "guess", "", 401},
		{"metrics with read token", "GET", "/metrics", host, "", "read-tok", "", 200},
		{"metrics with write token", "GET", "/metrics", host, "", "write-tok", "", 200},
		{"health stays public", "GET", "/health", host, "", "", "", 200},
	})
}
