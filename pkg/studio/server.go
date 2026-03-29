// Package studio provides the SECSGEM Studio web UI server, combining an
// integrated simulator, validator, and message tracer in a single-binary
// web interface served via go:embed.
package studio

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"

	existingsim "github.com/dashfactory/go-factory-io/examples/simulator"
	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"github.com/dashfactory/go-factory-io/pkg/simulator"
	"github.com/dashfactory/go-factory-io/pkg/validator"
)

//go:embed web/*
var webFS embed.FS

// Config holds studio server configuration.
type Config struct {
	EquipmentAddr string // External equipment address (empty = embedded simulator)
	SessionID     uint16
}

// Server serves the SECSGEM Studio web UI.
type Server struct {
	logger  *slog.Logger
	config  Config
	mux     *http.ServeMux

	// Core components
	schemas  *validator.SchemaRegistry
	stateVal *validator.StateValidator
	timing   *validator.TimingTracker
	host     *simulator.Host
	equip    *existingsim.Equipment

	// WebSocket clients
	clientsMu sync.RWMutex
	clients   map[*websocket.Conn]struct{}

	// Trace log
	traceMu sync.Mutex
	trace   []TraceEntry
	traceID uint64
}

// TraceEntry records one message for the trace view.
type TraceEntry struct {
	ID         uint64                      `json:"id"`
	Timestamp  time.Time                   `json:"timestamp"`
	Direction  string                      `json:"direction"` // "tx" or "rx"
	Stream     byte                        `json:"stream"`
	Function   byte                        `json:"function"`
	WBit       bool                        `json:"wbit"`
	BodySML    string                      `json:"bodySml"`
	Validation []validator.ValidationResult `json:"validation"`
}

// WSMessage is the JSON envelope for WebSocket communication.
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// NewServer creates a studio server.
func NewServer(cfg Config, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		logger:   logger,
		config:   cfg,
		mux:      http.NewServeMux(),
		schemas:  validator.DefaultRegistry(),
		stateVal: validator.NewStateValidator(),
		timing:   validator.NewTimingTracker(validator.DefaultTimingConfig()),
		clients:  make(map[*websocket.Conn]struct{}),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Static files from embedded FS
	webSub, _ := fs.Sub(webFS, "web")
	s.mux.Handle("/", http.FileServer(http.FS(webSub)))

	// Health endpoint (UptimeRobot / Render health check)
	s.mux.HandleFunc("/health", s.handleHealth)

	// WebSocket endpoint
	s.mux.HandleFunc("/ws", s.handleWS)

	// REST API endpoints (with CORS)
	s.mux.HandleFunc("/api/status", s.cors(s.handleStatus))
	s.mux.HandleFunc("/api/report", s.cors(s.handleReport))
	s.mux.HandleFunc("/api/trace", s.cors(s.handleTrace))
}

func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next(w, r)
	}
}

// Handler returns the http.Handler.
func (s *Server) Handler() http.Handler { return s.mux }

// StartEquipment starts the embedded equipment simulator.
func (s *Server) StartEquipment(ctx context.Context) (string, error) {
	cfg := existingsim.EquipmentConfig{
		ListenAddress:    ":0",
		SessionID:        s.config.SessionID,
		ModelName:        "STUDIO-EQUIP",
		SoftwareRevision: "1.0.0",
		EventInterval:    5 * time.Second,
	}
	s.equip = existingsim.NewEquipment(cfg, s.logger)
	if err := s.equip.Start(ctx); err != nil {
		return "", fmt.Errorf("studio: start equipment: %w", err)
	}
	addr := s.equip.Addr()
	s.logger.Info("Studio equipment simulator started", "addr", addr)
	return addr, nil
}

// ConnectHost connects the host simulator to the equipment.
func (s *Server) ConnectHost(ctx context.Context, addr string) error {
	s.host = simulator.NewHost(addr, s.config.SessionID, s.logger)
	s.host.SetInterceptor(s.onMessage)
	if err := s.host.Connect(ctx); err != nil {
		return fmt.Errorf("studio: connect host: %w", err)
	}
	s.logger.Info("Studio host connected", "addr", addr)
	return nil
}

// StopAll stops both host and equipment.
func (s *Server) StopAll() {
	if s.host != nil {
		s.host.Close()
	}
	if s.equip != nil {
		s.equip.Stop()
	}
}

// onMessage is the message interceptor called for every TX/RX.
func (s *Server) onMessage(dir simulator.Direction, stream, function byte, body *secs2.Item, results []validator.ValidationResult) {
	bodySML := ""
	if body != nil {
		bodySML = body.String()
	}

	s.traceMu.Lock()
	s.traceID++
	entry := TraceEntry{
		ID:         s.traceID,
		Timestamp:  time.Now(),
		Direction:  string(dir),
		Stream:     stream,
		Function:   function,
		BodySML:    bodySML,
		Validation: results,
	}
	s.trace = append(s.trace, entry)
	s.traceMu.Unlock()

	// Broadcast to WebSocket clients
	s.broadcast("trace", entry)
}

func (s *Server) broadcast(msgType string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	msg := WSMessage{Type: msgType, Data: payload}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	for c := range s.clients {
		go func(conn *websocket.Conn) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn.Write(ctx, websocket.MessageText, msgBytes)
		}(c)
	}
}

// --- WebSocket Handler ---

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		s.logger.Error("WebSocket accept failed", "error", err)
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	s.clientsMu.Lock()
	s.clients[c] = struct{}{}
	s.clientsMu.Unlock()
	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, c)
		s.clientsMu.Unlock()
	}()

	// Send current state
	s.sendStatus(c)

	for {
		_, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		var msg WSMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		s.handleWSCommand(r.Context(), c, msg)
	}
}

func (s *Server) handleWSCommand(ctx context.Context, c *websocket.Conn, msg WSMessage) {
	switch msg.Type {
	case "send":
		s.handleWSSend(ctx, msg.Data)
	case "quick_send":
		s.handleWSQuickSend(ctx, msg.Data)
	case "fault":
		s.handleWSFault(ctx, msg.Data)
	case "run_script":
		s.handleWSRunScript(ctx, c, msg.Data)
	case "get_report":
		s.handleWSReport(c)
	case "get_state":
		s.sendStatus(c)
	case "clear_trace":
		s.traceMu.Lock()
		s.trace = nil
		s.traceMu.Unlock()
	}
}

func (s *Server) handleWSSend(ctx context.Context, data json.RawMessage) {
	var cmd struct {
		Stream   byte   `json:"stream"`
		Function byte   `json:"function"`
		WBit     bool   `json:"wbit"`
		Body     string `json:"body"`
	}
	if err := json.Unmarshal(data, &cmd); err != nil {
		return
	}
	if s.host == nil {
		s.broadcast("error", map[string]string{"message": "host not connected"})
		return
	}

	var body *secs2.Item
	if cmd.Body != "" {
		var err error
		body, err = simulator.ParseSML(cmd.Body)
		if err != nil {
			s.broadcast("error", map[string]string{"message": "SML parse error: " + err.Error()})
			return
		}
	}

	s.host.SendRaw(ctx, cmd.Stream, cmd.Function, cmd.WBit, body)
}

func (s *Server) handleWSQuickSend(ctx context.Context, data json.RawMessage) {
	var cmd struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &cmd); err != nil {
		return
	}
	if s.host == nil {
		s.broadcast("error", map[string]string{"message": "host not connected"})
		return
	}

	switch cmd.Name {
	case "establish_comm":
		s.host.EstablishComm(ctx)
	case "are_you_there":
		s.host.AreYouThere(ctx)
	case "request_online":
		s.host.RequestOnline(ctx)
	case "request_offline":
		s.host.RequestOffline(ctx)
	case "rcmd_start":
		s.host.SendRCMD(ctx, "START", nil)
	}
}

func (s *Server) handleWSFault(ctx context.Context, data json.RawMessage) {
	var cmd struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &cmd); err != nil {
		return
	}
	if s.host == nil {
		return
	}
	fi := simulator.NewFaultInjector(s.host.Session(), s.logger)
	fi.Inject(simulator.FaultType(cmd.Type), nil)
}

func (s *Server) handleWSRunScript(ctx context.Context, c *websocket.Conn, data json.RawMessage) {
	var cmd struct {
		Index int `json:"index"` // index into BuiltinScenarios
	}
	if err := json.Unmarshal(data, &cmd); err != nil {
		return
	}
	if s.host == nil {
		return
	}
	scenarios := simulator.BuiltinScenarios()
	if cmd.Index < 0 || cmd.Index >= len(scenarios) {
		return
	}
	runner := simulator.NewScriptRunner(s.host, s.logger)
	result := runner.Run(ctx, scenarios[cmd.Index])
	s.broadcast("script_result", result)
}

func (s *Server) handleWSReport(c *websocket.Conn) {
	report := validator.GenerateReport(s.schemas)
	payload, _ := json.Marshal(report)
	msg := WSMessage{Type: "report", Data: payload}
	msgBytes, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Write(ctx, websocket.MessageText, msgBytes)
}

func (s *Server) sendStatus(c *websocket.Conn) {
	status := map[string]interface{}{
		"connected":  s.host != nil,
		"embedded":   s.equip != nil,
		"traceCount": len(s.trace),
	}
	payload, _ := json.Marshal(status)
	msg := WSMessage{Type: "status", Data: payload}
	msgBytes, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Write(ctx, websocket.MessageText, msgBytes)
}

// --- REST Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"service": "secsgem-studio",
		"version": "0.1.0",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"connected":  s.host != nil,
			"embedded":   s.equip != nil,
			"traceCount": len(s.trace),
		},
	})
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	report := validator.GenerateReport(s.schemas)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    report,
	})
}

func (s *Server) handleTrace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.traceMu.Lock()
	entries := make([]TraceEntry, len(s.trace))
	copy(entries, s.trace)
	s.traceMu.Unlock()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    entries,
	})
}
