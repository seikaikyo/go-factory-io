// Package security provides OT security primitives for industrial communication.
// Implements controls per IEC 62443, SEMI E187, and NIST SP 800-82.
package security

import (
	"log/slog"
	"time"
)

// Level represents the severity of a security event.
type Level int

const (
	LevelInfo     Level = iota // Informational (successful operations)
	LevelWarning               // Warning (potential concern, no action needed)
	LevelCritical              // Critical (requires attention, possible attack)
)

func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelWarning:
		return "WARNING"
	case LevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// Category classifies security events per IEC 62443 Foundational Requirements.
type Category string

const (
	CatAuth         Category = "auth"         // FR1: Identification & Authentication
	CatAccess       Category = "access"       // FR2: Use Control
	CatIntegrity    Category = "integrity"    // FR3: System Integrity
	CatConfidential Category = "confidential" // FR4: Data Confidentiality
	CatDataFlow     Category = "dataflow"     // FR5: Restricted Data Flow
	CatAudit        Category = "audit"        // FR6: Timely Response to Events
	CatAvailability Category = "availability" // FR7: Resource Availability
)

// Event represents a security-relevant event for audit logging.
// Covers IEC 62443 FR6, NIST AU, SEMI E187 Security Monitoring.
type Event struct {
	Time     time.Time
	Level    Level
	Category Category
	Type     string                 // e.g., "connection_rejected", "auth_failed"
	Source   string                 // Remote IP / session ID
	Message  string                 // Human-readable description
	Details  map[string]interface{} // Additional context
}

// EventHandler processes security events.
type EventHandler func(event Event)

// Auditor collects and dispatches security events.
type Auditor struct {
	logger   *slog.Logger
	handlers []EventHandler
}

// NewAuditor creates a security event auditor.
func NewAuditor(logger *slog.Logger) *Auditor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Auditor{logger: logger}
}

// OnEvent registers a security event handler.
func (a *Auditor) OnEvent(handler EventHandler) {
	a.handlers = append(a.handlers, handler)
}

// Emit logs and dispatches a security event.
func (a *Auditor) Emit(event Event) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}

	// Log via slog
	level := slog.LevelInfo
	switch event.Level {
	case LevelWarning:
		level = slog.LevelWarn
	case LevelCritical:
		level = slog.LevelError
	}

	a.logger.Log(nil, level, "SECURITY",
		"category", string(event.Category),
		"type", event.Type,
		"source", event.Source,
		"message", event.Message,
	)

	// Dispatch to handlers
	for _, h := range a.handlers {
		h(event)
	}
}

// --- Convenience emitters ---

func (a *Auditor) ConnectionRejected(source, reason string) {
	a.Emit(Event{
		Level:    LevelWarning,
		Category: CatDataFlow,
		Type:     "connection_rejected",
		Source:   source,
		Message:  reason,
	})
}

func (a *Auditor) AuthFailed(source, reason string) {
	a.Emit(Event{
		Level:    LevelCritical,
		Category: CatAuth,
		Type:     "auth_failed",
		Source:   source,
		Message:  reason,
	})
}

func (a *Auditor) RateLimited(source string, rate int) {
	a.Emit(Event{
		Level:    LevelWarning,
		Category: CatAvailability,
		Type:     "rate_limited",
		Source:   source,
		Message:  "message rate exceeded",
		Details:  map[string]interface{}{"rate": rate},
	})
}

func (a *Auditor) MalformedMessage(source string, err error) {
	msg := "malformed message"
	if err != nil {
		msg = err.Error()
	}
	a.Emit(Event{
		Level:    LevelWarning,
		Category: CatIntegrity,
		Type:     "malformed_message",
		Source:   source,
		Message:  msg,
	})
}

func (a *Auditor) UnauthorizedMessage(source string, stream, function byte) {
	a.Emit(Event{
		Level:    LevelCritical,
		Category: CatAccess,
		Type:     "unauthorized_message",
		Source:   source,
		Message:  "message not allowed by session policy",
		Details:  map[string]interface{}{"stream": stream, "function": function},
	})
}

func (a *Auditor) TLSHandshakeOK(source string) {
	a.Emit(Event{
		Level:    LevelInfo,
		Category: CatAuth,
		Type:     "tls_handshake_ok",
		Source:   source,
		Message:  "TLS handshake completed",
	})
}

func (a *Auditor) SessionExpired(source string, duration time.Duration) {
	a.Emit(Event{
		Level:    LevelInfo,
		Category: CatAccess,
		Type:     "session_expired",
		Source:   source,
		Message:  "session TTL exceeded",
		Details:  map[string]interface{}{"duration": duration.String()},
	})
}
