---
title: SECSGEM Studio - Integrated Simulator & Validator with Web UI
type: feature
status: completed
created: 2026-03-29
---

# SECSGEM Studio

Integrated simulator + validator + message tracer, served as a single-binary web UI via `go:embed`.

## Summary

go-factory-io currently has a basic equipment simulator (`examples/simulator/`) and CLI tools (`secsgem simulate`, `secsgem connect`), but lacks:

1. **Host (MES) simulator** — can only simulate equipment side, not host side
2. **Message validator** — no schema validation against SEMI S/F structures
3. **State transition validator** — existing state checks are internal, not exposed as a tool
4. **Fault injection** — cannot simulate disconnects, timeouts, malformed messages
5. **Visual trace/debug** — no message sequence diagram or real-time monitoring UI
6. **Compliance report** — no coverage report of implemented S/F messages

SECSGEM Studio combines all of these into one `secsgem studio` subcommand with an embedded web UI.

## Architecture

```
secsgem studio --port 8080
│
├── Web UI (go:embed, vanilla HTML/CSS/JS)
│   ├── Dashboard   — live stats, E30 state machine diagram, message feed
│   ├── Simulator   — role selection (Host/Equipment), SML editor, send messages
│   ├── Validator   — message schema check, state transition check, timing check
│   ├── Trace       — sequence diagram (HOST <-> EQUIP), per-message validation badge
│   └── Report      — standard coverage bars, S/F implementation table
│
├── Backend (Go, reuses existing pkg/*)
│   ├── pkg/studio/server.go       — HTTP server, WebSocket hub, go:embed assets
│   ├── pkg/studio/session.go      — manages simulated Host/Equipment sessions
│   ├── pkg/studio/scenario.go     — scripted interaction scenarios (YAML)
│   │
│   ├── pkg/validator/schema.go    — S/F message structure validation
│   ├── pkg/validator/state.go     — state transition compliance checking
│   ├── pkg/validator/timing.go    — T3-T8 timeout compliance
│   ├── pkg/validator/report.go    — coverage report generator
│   │
│   ├── pkg/simulator/host.go      — Host (MES) simulator
│   ├── pkg/simulator/fault.go     — fault injection (disconnect, timeout, corrupt)
│   └── pkg/simulator/script.go    — scenario script runner
│
└── WebSocket (/ws)
    ├── message stream    — real-time TX/RX with validation results
    ├── state updates     — state machine transitions
    └── commands          — send message, inject fault, run scenario
```

## Change Scope

### New Files

| Path | Purpose |
|------|---------|
| `pkg/studio/server.go` | HTTP server + WebSocket hub + static file serving |
| `pkg/studio/session.go` | Studio session management (host/equip/both) |
| `pkg/studio/scenario.go` | YAML scenario definitions and runner |
| `pkg/studio/embed.go` | `go:embed` directive for web assets |
| `pkg/studio/web/` | Static HTML/CSS/JS assets (embedded) |
| `pkg/validator/schema.go` | S/F message structure validator |
| `pkg/validator/state.go` | State transition validator (wraps existing state machines) |
| `pkg/validator/timing.go` | Protocol timing validator (T3-T8) |
| `pkg/validator/report.go` | Implementation coverage report |
| `pkg/simulator/host.go` | Host-side (MES) simulator |
| `pkg/simulator/fault.go` | Fault injection engine |
| `pkg/simulator/script.go` | Scenario script executor |

### Modified Files

| Path | Change |
|------|--------|
| `cmd/secsgem/main.go` | Add `studio` subcommand |
| `go.mod` | Add `nhooyr.io/websocket` (BSD, WebSocket) |
| `README.md` | Add Studio section |

### No Changes

- `pkg/driver/gem/*` — reuse as-is
- `pkg/transport/hsms/*` — reuse as-is
- `pkg/message/secs2/*` — reuse as-is
- `pkg/security/*` — reuse as-is
- `api/rest/*`, `api/grpc/*` — independent, no changes

## Implementation Phases

### Phase 1: Validator Engine (`pkg/validator/`)

Core validation logic, no UI dependency.

- **schema.go**: Define S/F message schemas (expected item structure per E30/E87/E40)
  - `MessageSchema` struct: Stream, Function, Direction, Items (nested tree)
  - `Validate(stream, function, body) -> []ValidationResult`
  - Cover S1, S2, S3, S5, S6, S7, S16 families
- **state.go**: Wrap existing state machines, expose validation API
  - `ValidateTransition(fromState, toState, trigger) -> ValidationResult`
  - Support E30 comm/control, E87 carrier, E40 process job, E94 control job
- **timing.go**: Track T3-T8 compliance
  - `RecordRequest(systemByte, timestamp)`
  - `RecordReply(systemByte, timestamp) -> TimingResult`
- **report.go**: Scan handler for registered S/F handlers
  - `GenerateCoverage(handler) -> CoverageReport`
  - Output: per-standard coverage percentage, per-S/F status (full/partial/none)

### Phase 2: Enhanced Simulator (`pkg/simulator/`)

- **host.go**: MES-side simulator
  - Reuse `hsms.Session` in Active mode
  - Pre-built message sequences: establish comm, go online, send RCMD, create PJ
  - `SendMessage(stream, function, body)` — arbitrary message sending
- **fault.go**: Fault injection
  - `InjectDisconnect()` — drop TCP connection
  - `InjectTimeout(duration)` — suppress reply for N seconds
  - `InjectCorrupt()` — send garbled bytes
  - `InjectBadSType()` — send invalid HSMS header
- **script.go**: Scenario runner
  - YAML format: sequence of send/expect/delay/assert steps
  - Built-in scenarios: E30 comm setup, E87 carrier flow, E40 process job lifecycle

### Phase 3: Web UI (`pkg/studio/`)

Single-binary embedded web UI.

- **server.go**: HTTP + WebSocket server
  - `go:embed web/*` for static assets
  - WebSocket hub: broadcast messages, accept commands
  - REST endpoints: `/api/studio/send`, `/api/studio/fault`, `/api/studio/scenario`
- **session.go**: Studio session lifecycle
  - Create host/equipment/both sessions
  - Message interception for real-time validation
  - State tracking across all active sessions
- **web/**: Static assets
  - `index.html` — single page with tab navigation (based on mockup)
  - `studio.js` — WebSocket client, tab logic, SML editor
  - `studio.css` — dark theme (based on mockup)
  - No build step, no Node.js dependency

### Phase 4: CLI Integration

- Add `studio` subcommand to `cmd/secsgem/main.go`
- Flags: `--port 8080`, `--equipment :5000`, `--host :5001`
- Auto-start embedded simulator if no external target specified

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| `go:embed` for UI | Single binary, zero runtime deps, matches project philosophy |
| Vanilla HTML/JS | No build step, no Node.js, `go build` produces everything |
| `nhooyr.io/websocket` | Pure Go, BSD license, stdlib-compatible API |
| Validator as separate pkg | Can be used programmatically without UI (testing, CI) |
| Simulator as separate pkg | Host simulator useful independently of Studio |
| YAML scenarios | Human-readable, version-controllable test scripts |

## Legal / Compliance (per CLAUDE.md 2.5)

| Item | Status |
|------|--------|
| SEMI trademark | Add disclaimer: "SEMI, SECS, GEM are trademarks of SEMI. Not affiliated with or endorsed by SEMI." |
| Standard references | Use "implements E30" / "follows E87 spec", never "compliant" or "certified" |
| Report page wording | "Implementation Coverage" not "Compliance Report" |
| State diagrams | Self-drawn SVG, not copied from SEMI documents |
| License | nhooyr.io/websocket is BSD — OK |
| AI disclosure | Co-Authored-By in all commits |

## Test Plan

1. **Validator unit tests**: Each S/F schema, valid + invalid cases
2. **State validator tests**: All state machine transitions (valid + invalid)
3. **Timing tests**: T3 timeout detection, T6/T7/T8 boundary cases
4. **Host simulator tests**: E2E with equipment simulator (both directions)
5. **Fault injection tests**: Verify each fault type triggers expected behavior
6. **Scenario runner tests**: Run built-in scenarios, verify pass/fail
7. **WebSocket tests**: Connect, send command, receive response
8. **Coverage report tests**: Compare report output against known handler registration
9. **Integration test**: Full stack — start studio, open browser, run scenario, check results

## Checklist

- [ ] Phase 1: pkg/validator/schema.go + tests
- [ ] Phase 1: pkg/validator/state.go + tests
- [ ] Phase 1: pkg/validator/timing.go + tests
- [ ] Phase 1: pkg/validator/report.go + tests
- [ ] Phase 2: pkg/simulator/host.go + tests
- [ ] Phase 2: pkg/simulator/fault.go + tests
- [ ] Phase 2: pkg/simulator/script.go + tests
- [ ] Phase 3: pkg/studio/server.go (HTTP + WebSocket)
- [ ] Phase 3: pkg/studio/session.go
- [ ] Phase 3: pkg/studio/web/ (HTML/CSS/JS from mockup)
- [ ] Phase 4: cmd/secsgem/main.go add studio subcommand
- [ ] Phase 4: go.mod add websocket dependency
- [ ] README.md update
- [ ] Legal disclaimer in README
- [ ] All tests pass
- [ ] `go build` produces single binary with embedded UI
