// Package rest provides an HTTP REST API for the SECS/GEM driver.
// This allows external services (e.g., smart-factory-demo's FastAPI backend)
// to interact with equipment through simple HTTP calls.
package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/dashfactory/go-factory-io/pkg/driver/gem"
	"github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

// Server is the REST API server for equipment communication.
type Server struct {
	logger      *slog.Logger
	handler     *gem.Handler
	session     *hsms.Session
	mux         *http.ServeMux
	bearerToken string // Empty = no auth required

	// SSE event subscribers
	sseClients   map[chan EventPayload]struct{}
	sseClientsMu sync.Mutex
}

// EventPayload is the JSON structure for SSE events.
type EventPayload struct {
	Type      string      `json:"type"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// NewServer creates a REST API server.
// bearerToken: if non-empty, requires Authorization: Bearer <token> on all /api/* endpoints.
func NewServer(session *hsms.Session, handler *gem.Handler, logger *slog.Logger, bearerToken ...string) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	token := ""
	if len(bearerToken) > 0 {
		token = bearerToken[0]
	}
	s := &Server{
		logger:      logger,
		handler:     handler,
		session:     session,
		mux:         http.NewServeMux(),
		bearerToken: token,
		sseClients:  make(map[chan EventPayload]struct{}),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/sv", s.handleListSV)
	s.mux.HandleFunc("GET /api/sv/{svid}", s.handleGetSV)
	s.mux.HandleFunc("GET /api/ec", s.handleListEC)
	s.mux.HandleFunc("GET /api/ec/{ecid}", s.handleGetEC)
	s.mux.HandleFunc("PUT /api/ec/{ecid}", s.handleSetEC)
	s.mux.HandleFunc("GET /api/alarms", s.handleListAlarms)
	s.mux.HandleFunc("GET /api/alarms/active", s.handleActiveAlarms)
	s.mux.HandleFunc("POST /api/command", s.handleCommand)
	s.mux.HandleFunc("GET /api/events", s.handleSSE)
}

// Handler returns the http.Handler for this server.
func (s *Server) Handler() http.Handler {
	h := http.Handler(s.mux)
	if s.bearerToken != "" {
		h = s.withAuth(h)
	}
	return withCORS(h)
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health endpoint is always public
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		expected := "Bearer " + s.bearerToken
		if auth != expected {
			writeError(w, http.StatusUnauthorized, "invalid or missing bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// BroadcastEvent sends an event to all SSE subscribers.
func (s *Server) BroadcastEvent(eventType string, data interface{}) {
	payload := EventPayload{
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
	s.sseClientsMu.Lock()
	defer s.sseClientsMu.Unlock()
	for ch := range s.sseClients {
		select {
		case ch <- payload:
		default:
			// Client too slow, skip
		}
	}
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"status":  "ok",
			"version": "0.1.0",
		},
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	state := s.handler.State()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"commState":    state.CommState().String(),
			"controlState": state.ControlState().String(),
			"communicating": state.IsCommunicating(),
			"online":       state.IsOnline(),
			"transport":    s.session.State().String(),
		},
	})
}

func (s *Server) handleListSV(w http.ResponseWriter, r *http.Request) {
	vars := s.handler.Variables()
	svids := vars.ListSVIDs()

	result := make([]map[string]interface{}, 0, len(svids))
	for _, svid := range svids {
		info, _ := vars.GetSVInfo(svid)
		val, _ := vars.GetSV(svid)
		result = append(result, map[string]interface{}{
			"svid":  svid,
			"name":  info.Name,
			"value": val,
			"units": info.Units,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    result,
	})
}

func (s *Server) handleGetSV(w http.ResponseWriter, r *http.Request) {
	svid, err := parseUint32(r.PathValue("svid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid SVID")
		return
	}

	val, ok := s.handler.Variables().GetSV(svid)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("SVID %d not found", svid))
		return
	}

	info, _ := s.handler.Variables().GetSVInfo(svid)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"svid":  svid,
			"name":  info.Name,
			"value": val,
			"units": info.Units,
		},
	})
}

func (s *Server) handleListEC(w http.ResponseWriter, r *http.Request) {
	vars := s.handler.Variables()
	ecids := vars.ListECIDs()

	result := make([]map[string]interface{}, 0, len(ecids))
	for _, ecid := range ecids {
		ec, _ := vars.GetEC(ecid)
		result = append(result, map[string]interface{}{
			"ecid":  ecid,
			"name":  ec.Name,
			"value": ec.Value,
			"units": ec.Units,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    result,
	})
}

func (s *Server) handleGetEC(w http.ResponseWriter, r *http.Request) {
	ecid, err := parseUint32(r.PathValue("ecid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ECID")
		return
	}

	ec, ok := s.handler.Variables().GetEC(ecid)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("ECID %d not found", ecid))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"ecid":  ecid,
			"name":  ec.Name,
			"value": ec.Value,
			"units": ec.Units,
		},
	})
}

func (s *Server) handleSetEC(w http.ResponseWriter, r *http.Request) {
	ecid, err := parseUint32(r.PathValue("ecid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ECID")
		return
	}

	var body struct {
		Value interface{} `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := s.handler.Variables().SetEC(ecid, body.Value); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"ecid": ecid, "value": body.Value},
	})
}

func (s *Server) handleListAlarms(w http.ResponseWriter, r *http.Request) {
	alarms := s.handler.Alarms().ListAlarms()
	result := make([]map[string]interface{}, 0, len(alarms))
	for _, a := range alarms {
		result = append(result, map[string]interface{}{
			"alid":     a.ALID,
			"name":     a.Name,
			"text":     a.Text,
			"state":    a.State.String(),
			"enabled":  a.Enabled,
			"severity": a.Severity,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    result,
	})
}

func (s *Server) handleActiveAlarms(w http.ResponseWriter, r *http.Request) {
	alarms := s.handler.Alarms().ListActiveAlarms()
	result := make([]map[string]interface{}, 0, len(alarms))
	for _, a := range alarms {
		result = append(result, map[string]interface{}{
			"alid": a.ALID,
			"name": a.Name,
			"text": a.Text,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    result,
	})
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string                 `json:"command"`
		Params  map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	var params []gem.CommandParam
	for k, v := range body.Params {
		params = append(params, gem.CommandParam{Name: k, Value: v})
	}

	status := s.handler.Commands().Execute(context.Background(), body.Command, params)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"command": body.Command,
			"status":  status.String(),
			"code":    int(status),
		},
	})
}

// handleSSE serves Server-Sent Events for real-time equipment updates.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan EventPayload, 64)
	s.sseClientsMu.Lock()
	s.sseClients[ch] = struct{}{}
	s.sseClientsMu.Unlock()

	defer func() {
		s.sseClientsMu.Lock()
		delete(s.sseClients, ch)
		s.sseClientsMu.Unlock()
		close(ch)
	}()

	s.logger.Info("SSE client connected", "remote", r.RemoteAddr)

	for {
		select {
		case <-r.Context().Done():
			s.logger.Info("SSE client disconnected", "remote", r.RemoteAddr)
			return
		case event := <-ch:
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{
		"success": false,
		"error":   map[string]interface{}{"code": status, "message": message},
	})
}

func parseUint32(s string) (uint32, error) {
	n, err := strconv.ParseUint(s, 10, 32)
	return uint32(n), err
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
