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
	"net/url"
	"sync"
	"time"

	"nhooyr.io/websocket"

	existingsim "github.com/dashfactory/go-factory-io/examples/simulator"
	"github.com/dashfactory/go-factory-io/pkg/message/secs2"
	"github.com/dashfactory/go-factory-io/pkg/security"
	"github.com/dashfactory/go-factory-io/pkg/simulator"
	"github.com/dashfactory/go-factory-io/pkg/validator"
)

//go:embed web/*
var webFS embed.FS

// Config holds studio server configuration.
type Config struct {
	EquipmentAddr string // External equipment address (empty = embedded simulator)
	SessionID     uint16

	// AllowedOrigins lists extra browser origins permitted to open the
	// WebSocket and to make cross-origin REST calls. Empty means same-origin
	// only, which is what the embedded UI needs.
	AllowedOrigins []string

	// Token gates /ws and /api/. When empty the WebSocket still connects and
	// serves the read-only commands, but every command that drives the
	// equipment (send, quick_send, fault, run_script) is refused unless
	// LoopbackOnly is set.
	Token string

	// SimulatorDemo relaxes the token requirement for equipment-driving
	// commands when the server is backed by the embedded simulator and no
	// real equipment is reachable. Intended for public demos where the only
	// thing a visitor can drive is a simulated device inside this process.
	// Never set it when EquipmentAddr points at a real tool.
	SimulatorDemo bool

	// LoopbackOnly records that the HTTP listener is bound to loopback and is
	// therefore unreachable from the network. It relaxes the token
	// requirement for the equipment-driving commands so `secsgem studio`
	// stays usable on a developer machine. Callers must derive it from the
	// actual listen address (security.IsLoopbackListen), never from the
	// client's remote address, which a proxy can forge.
	LoopbackOnly bool
}

// Server serves the SECSGEM Studio web UI.
type Server struct {
	logger *slog.Logger
	config Config
	mux    *http.ServeMux

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
	ID         uint64                       `json:"id"`
	Timestamp  time.Time                    `json:"timestamp"`
	Direction  string                       `json:"direction"` // "tx" or "rx"
	Stream     byte                         `json:"stream"`
	Function   byte                         `json:"function"`
	WBit       bool                         `json:"wbit"`
	BodySML    string                       `json:"bodySml"`
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

	// REST API endpoints (CORS allowlist + token)
	s.mux.HandleFunc("/api/status", s.cors(s.auth(s.handleStatus)))
	s.mux.HandleFunc("/api/report", s.cors(s.auth(s.handleReport)))
	s.mux.HandleFunc("/api/trace", s.cors(s.auth(s.handleTrace)))
}

// cors echoes CORS headers only for an allowlisted origin. With no allowlist
// configured the studio API is same-origin only.
func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := security.OriginAllowed(s.config.AllowedOrigins, origin)

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Add("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			if !allowed {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// auth requires the configured token on /api/ routes. No token configured
// means the studio is expected to be loopback-bound (enforced by the caller
// via security.RequireTokenForListen), so the check is skipped.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config.Token == "" {
			next(w, r)
			return
		}
		if !s.tokenOK(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   map[string]interface{}{"code": 401, "message": "invalid or missing token"},
			})
			return
		}
		next(w, r)
	}
}

// tokenOK validates the request credential against the configured token.
//
// The Authorization header is the preferred carrier. The `token` query
// parameter exists because the browser WebSocket API cannot set request
// headers; it is accepted on every route for consistency. Query credentials
// can end up in access logs, so operators fronting the studio with a proxy
// should prefer the header.
func (s *Server) tokenOK(r *http.Request) bool {
	if presented, ok := security.BearerToken(r.Header.Get("Authorization")); ok {
		return security.CompareToken(s.config.Token, presented)
	}
	return security.CompareToken(s.config.Token, r.URL.Query().Get("token"))
}

// Handler returns the http.Handler.
func (s *Server) Handler() http.Handler { return s.mux }

// StartEquipment starts the embedded equipment simulator.
//
// It binds to loopback: the embedded simulator exists only for the studio's
// own host to talk to, and a wildcard bind would publish an unauthenticated
// HSMS endpoint on every interface alongside the web UI.
func (s *Server) StartEquipment(ctx context.Context) (string, error) {
	cfg := existingsim.EquipmentConfig{
		ListenAddress:    "127.0.0.1:0",
		SessionID:        s.config.SessionID,
		ModelName:        "STUDIO-EQUIP",
		SoftwareRevision: "1.0.0",
		EventInterval:    5 * time.Second,
		// The sandbox simulator has to accept the messages the studio
		// demonstrates. Reaching it means driving the studio host, which the
		// WebSocket command gate already restricts.
		AllowWrites: true,
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
	// Reject before the upgrade so an unauthorized peer never gets a socket.
	if s.config.Token != "" && !s.tokenOK(r) {
		s.logger.Warn("WebSocket rejected: invalid or missing token", "remote", r.RemoteAddr)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// No InsecureSkipVerify: the library's default is same-origin, and
	// OriginPatterns widens that only to explicitly configured hosts.
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: originPatterns(s.config.AllowedOrigins),
	})
	if err != nil {
		s.logger.Error("WebSocket accept failed", "error", err, "remote", r.RemoteAddr,
			"origin", r.Header.Get("Origin"))
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	// A socket authenticated with the token may drive the equipment, and so
	// may a loopback-bound listener that nothing off-box can reach. A public
	// demo backed only by the embedded simulator may too, because the worst a
	// visitor can reach is a simulated device inside this process. Any other
	// case is read-only.
	privileged := s.config.Token != "" || s.config.LoopbackOnly || s.simulatorDemo()

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
		s.handleWSCommand(r.Context(), c, msg, privileged)
	}
}

// mutatingWSCommands drive the equipment: they compose and send SECS-II
// traffic, inject faults, or run scripted sequences. They require a socket
// authenticated with the studio token.
var mutatingWSCommands = map[string]struct{}{
	"send":       {},
	"quick_send": {},
	"fault":      {},
	"run_script": {},
}

// IsMutatingWSCommand reports whether a WebSocket command drives the
// equipment and therefore needs the studio token.
func IsMutatingWSCommand(cmd string) bool {
	_, ok := mutatingWSCommands[cmd]
	return ok
}

func (s *Server) handleWSCommand(ctx context.Context, c *websocket.Conn, msg WSMessage, privileged bool) {
	if IsMutatingWSCommand(msg.Type) && !privileged {
		s.logger.Warn("WebSocket command refused: studio token not configured", "command", msg.Type)
		s.sendTo(c, "error", map[string]string{
			"message": "command '" + msg.Type + "' requires the studio token; start with --studio-token or " +
				security.EnvStudioToken,
		})
		return
	}

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

// sendTo writes a single envelope to one client.
func (s *Server) sendTo(c *websocket.Conn, msgType string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	msgBytes, err := json.Marshal(WSMessage{Type: msgType, Data: payload})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.Write(ctx, websocket.MessageText, msgBytes)
}

// originPatterns converts the configured origin allowlist into the host
// patterns nhooyr.io/websocket matches against the Origin header. A nil
// result keeps the library's same-origin default.
func originPatterns(origins []string) []string {
	var out []string
	for _, o := range origins {
		host := o
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			host = u.Host
		}
		if host != "" {
			out = append(out, host)
		}
	}
	return out
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

// simulatorDemo reports whether the equipment-driving relaxation applies: the
// caller asked for demo mode and no external equipment address is configured,
// so the only reachable device is the embedded simulator.
func (s *Server) simulatorDemo() bool {
	return s.config.SimulatorDemo && s.config.EquipmentAddr == ""
}
