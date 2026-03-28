# go-factory-io

Open-source SECS/GEM equipment driver in Go. Covers 12 SEMI standards, 5 communication protocols, and IEC 62443 SL4 security -- in a single static binary that runs on a Raspberry Pi.

**[Live Demo](https://factory.dashai.dev/tv/equipment)** | [API Docs](#rest-api) | [Go Library](#go-library-usage)

## Why this project exists

I started programming through the open-source community over twenty years ago, running a [phpBB forum](https://phpbb-tw.net/phpbb/), translating docs, fixing code, helping people. Building things and putting them on the internet for others to use felt natural then. It still does.

The SEMI SECS/GEM specs run over two thousand pages. Reading them is genuinely interesting -- how the state machines transition, how messages encode, the strict handshake protocol between equipment and host. Every layer has a reason behind its design. I enjoy digging into these systems and integrating across layers, so I decided to write a comprehensive Go implementation and see if it could be useful to the community.

The open-source ecosystem already has solid foundations. [secs4net](https://github.com/mkjeff/secs4net) (C#, 590+ stars) is battle-tested in .NET production environments. [secsgem](https://github.com/bparzella/secsgem) (Python) has a thorough GEM state machine. [secs4java8](https://github.com/kenta-shimizu/secs4java8) and [secs4go](https://github.com/younglifestyle/secs4go) provide stable transport layers in Java and Go respectively. These projects laid the groundwork for the entire ecosystem.

go-factory-io builds on top of that groundwork. It integrates the 300mm fab standards (E87 Carrier, E40 Process Job, E90 Substrate Tracking, E94 Control Job, E116 OEE), common factory protocols (OPC-UA, MQTT, Modbus TCP), and the increasingly important cybersecurity requirements (IEC 62443) into a single binary. I learned a lot reading secs4net and secsgem source code -- quite a few HSMS connection management patterns were absorbed from there.

### Coverage

```
  SEMI Standards & Protocols
  ────────────────────────────────────────
  Transport       E5 SECS-II Codec
                  E30 GEM State Machine
                  E37 HSMS (TLS/mTLS)

  300mm           E87  Carrier Management
                  E40  Process Jobs
                  E90  Substrate Tracking
                  E94  Control Jobs
                  E116 EPT / OEE

  Security        E187/E191 Cybersecurity
                  IEC 62443 SL4

  Multi-Protocol  OPC-UA, MQTT, Modbus TCP
                  REST, gRPC, SSE

  Observability   Prometheus metrics
```

### Acknowledgments

This project stands on the shoulders of the SECS/GEM open-source community. Reading their source code shaped many design decisions:

| Project | Language | What I learned |
|---------|----------|---------------|
| [secs4net](https://github.com/mkjeff/secs4net) | C# | HSMS connection management patterns, production edge-case handling |
| [secsgem](https://github.com/bparzella/secsgem) | Python | Thorough GEM state machine implementation structure |
| [secs4java8](https://github.com/kenta-shimizu/secs4java8) | Java | Dual-mode SECS-I + HSMS-GS architecture |
| [secs4go](https://github.com/younglifestyle/secs4go) | Go | Idiomatic Go patterns for SECS-II binary encoding |

go-factory-io extends the transport layer upward -- integrating carrier management, process tracking, equipment performance analytics, and industrial cybersecurity that 300mm fabs need. A different layer of the same problem space.

## SEMI Standards Coverage

| Standard | Description | Status |
|----------|-------------|--------|
| E5 | SECS-II Message Encoding | Full (14 types, 7M+ ops/sec) |
| E30 | GEM Equipment Model | Full (state machines, SV/EC, CE, alarm, RCMD) |
| E37 | HSMS Transport | Full (Active/Passive, T3-T8, TLS/mTLS) |
| E87 | Carrier Management | Full (FOUP lifecycle, 25-slot map, load port) |
| E40 | Process Job Management | Full (9-state lifecycle, recipe, abort/stop) |
| E90 | Substrate Tracking | Full (wafer location, movement history) |
| E94 | Control Job Management | Full (scheduling, pause/resume) |
| E116 | Equipment Performance Tracking | Full (OEE calculation, 11 states) |
| E187 | Fab Equipment Cybersecurity | Implemented (TLS, RBAC, audit) |
| E191 | Cybersecurity Status Reporting | Implemented (/api/security/status) |

## Quick Start

```bash
# Build
go build -o secsgem ./cmd/secsgem/

# Run equipment simulator with REST API
./secsgem simulate

# In another terminal: query equipment
curl http://localhost:8080/api/status
curl http://localhost:8080/api/sv
curl http://localhost:8080/api/alarms

# Real-time event stream
curl -N http://localhost:8080/api/events
```

The simulator starts an HSMS equipment on `:5000` and a REST API on `:8080`. Connect as host:

```bash
./secsgem connect localhost:5000
```

## Architecture

```
                             go-factory-io
  ┌────────────────────────────────────────────────────────┐
  │                                                        │
  │  ┌─────────┐ ┌──────┐ ┌──────┐                        │
  │  │REST API │ │ gRPC │ │ MQTT │  Northbound             │
  │  │  + SSE  │ │      │ │Bridge│  (to MES/SCADA)         │
  │  └────┬────┘ └──┬───┘ └──┬───┘                         │
  │       └─────────┼────────┘                             │
  │            ┌────▼─────────────────────┐                │
  │            │       GEM Handler        │                │
  │            │  State Machine (E30)     │                │
  │            │  Variables (SV/EC)       │                │
  │            │  Events & Reports        │                │
  │            │  Alarms & Safety (S2)    │                │
  │            │  Remote Commands         │                │
  │            ├──────────────────────────┤                │
  │            │    300mm Extensions      │                │
  │            │  Carrier Mgmt (E87)      │                │
  │            │  Process Jobs (E40)      │                │
  │            │  Substrate Track (E90)   │                │
  │            │  Control Jobs (E94)      │                │
  │            │  EPT / OEE (E116)        │                │
  │            └────────┬─────────────────┘                │
  │            ┌────────▼────────┐                         │
  │            │  SECS-II Codec  │  Encode/Decode          │
  │            └────────┬────────┘                         │
  │       ┌─────────────┼──────────────┐                   │
  │  ┌────▼────┐  ┌─────▼─────┐ ┌─────▼─────┐             │
  │  │  HSMS   │  │  OPC-UA   │ │  Modbus   │ Southbound  │
  │  │TCP/TLS  │  │           │ │   TCP     │ (to equip)  │
  │  └────┬────┘  └─────┬─────┘ └─────┬─────┘             │
  └───────┼─────────────┼─────────────┼────────────────────┘
          │             │             │
     Equipment      OPC-UA        PLC/Sensor
     (SECS/GEM)     Server        (Modbus)
```

## Deployment

Single static binary. No runtime dependencies.

```bash
# Linux AMD64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o secsgem ./cmd/secsgem/

# Raspberry Pi (ARM64) -- runs on 512MB RAM, <15MB resident
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o secsgem ./cmd/secsgem/

# Docker
docker build -t secsgem .
docker run -p 5000:5000 -p 8080:8080 secsgem
```

## REST API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/api/status` | Equipment state (comm, control, transport) |
| GET | `/api/sv` | List all Status Variables |
| GET | `/api/sv/{svid}` | Get a specific SV |
| GET | `/api/ec` | List all Equipment Constants |
| GET | `/api/ec/{ecid}` | Get a specific EC |
| PUT | `/api/ec/{ecid}` | Update an EC value |
| GET | `/api/alarms` | List all alarms |
| GET | `/api/alarms/active` | List active alarms |
| POST | `/api/command` | Execute a remote command (RCMD) |
| GET | `/api/events` | SSE stream for real-time events |
| GET | `/api/security/status` | SEMI E191 cybersecurity status |
| GET | `/metrics` | Prometheus metrics |

All responses: `{ "success": true, "data": ... }` or `{ "success": false, "error": { "code": 400, "message": "..." } }`

## Go Library Usage

### Connect to Equipment

```go
cfg := hsms.DefaultConfig("192.168.1.100:5000", hsms.RoleActive, 1)
session := hsms.NewSession(cfg, logger)
session.Connect(ctx)
session.Select(ctx)

// Send S1F13 Establish Communication
body := secs2.NewList(secs2.NewASCII("HOST"), secs2.NewASCII("1.0.0"))
data, _ := secs2.Encode(body)
reply, _ := session.SendMessage(ctx, hsms.NewDataMessage(1, 1, 13, true, 0, data))
```

### Auto-Reconnect

```go
ms := session.NewManagedSession(cfg, session.DefaultReconnectConfig(), logger)
ms.OnConnect(func(s *hsms.Session) { logger.Info("Connected") })
ms.Start(ctx) // Reconnects automatically with exponential backoff
```

### Equipment Simulator

```go
eq := simulator.NewEquipment(simulator.DefaultEquipmentConfig(), logger)
eq.Start(ctx)

// Custom sensor
eq.Handler().Variables().DefineSVDynamic(2001, "SensorA", "mV", func() interface{} {
    return readSensor()
})

// Custom command
eq.Handler().Commands().Register("PP_SELECT", func(ctx context.Context, params []gem.CommandParam) gem.CommandStatus {
    return gem.CommandOK
})
```

### 300mm Carrier Management (E87)

```go
cm := handler.Carriers()
cm.DefinePort(1)
cm.SetPortInService(1)
cm.BindCarrier("FOUP-001", 1, "LOT-A", "PRODUCT")

// Full lifecycle
cm.ProceedWithCarrier("FOUP-001")
cm.StartAccess("FOUP-001")
cm.CompleteAccess("FOUP-001")
cm.ReadyToUnload("FOUP-001")
```

### Process Jobs (E40) & OEE (E116)

```go
// Create and run process job
pm := handler.ProcessJobs()
pm.Create("PJ-001", "RECIPE-A", "FOUP-001", []int{1,2,3}, nil)
pm.Setup("PJ-001")
pm.SetupComplete("PJ-001")
pm.Start("PJ-001")
pm.Complete("PJ-001")

// Track equipment performance
ept := handler.EPT()
ept.SetState(gem.EPTBusy)
ept.RecordUnit(false) // good unit
a, p, q, oee := ept.OEE()
```

## Multi-Protocol Support

### MQTT Bridge

Publishes GEM events to MQTT broker for MES/SCADA integration.

```bash
./secsgem simulate --mqtt-broker tcp://localhost:1883 --mqtt-prefix factory/eq01

# Subscribe from another terminal
mosquitto_sub -t "factory/eq01/#"
# factory/eq01/event/100  {"type":"collection_event",...}
# factory/eq01/alarm/1    {"type":"alarm","data":{"state":"set",...}}
```

Topics: `{prefix}/status`, `{prefix}/event/{ceid}`, `{prefix}/alarm/{alid}`, `{prefix}/sv/{svid}`

### gRPC API

```bash
./secsgem simulate --grpc-addr :50051
```

Proto at `api/grpc/proto/secsgem.proto`. 7 unary RPCs + 1 server-streaming (events).

### Modbus TCP

```go
client := modbus.NewClient(modbus.Config{Address: "192.168.1.100:502", UnitID: 1}, logger)
client.Connect(ctx)
regs, _ := client.ReadHoldingRegisters(ctx, 0, 10)
client.WriteSingleRegister(ctx, 100, 42)
```

FC01-FC06, FC15, FC16. Pure Go, no external dependencies.

### OPC-UA

```go
client := opcua.NewClient(opcua.Config{Endpoint: "opc.tcp://192.168.1.100:4840"}, logger)
client.Connect(ctx)
val, _ := client.Read(ctx, "ns=2;s=Temperature")
```

## Security (IEC 62443 SL4)

| Layer | Feature |
|-------|---------|
| Transport | TLS 1.2+, mTLS, IP allowlist, session TTL |
| Access | Per-session RBAC, read-only mode, S/F allowlist/denylist |
| Application | AES-256-GCM payload encryption, key rotation |
| Monitoring | Security event audit, webhook/syslog forwarding, anomaly detection interface |
| Certificate | CRL cache, OCSP checking |
| Key Storage | HSM/PKCS#11 interface (software fallback for testing) |
| Reporting | SEMI E191 cybersecurity status endpoint |
| Safety | SEMI S2 alarm severity interlock (ForceOffline/ForceIdle) |

```go
// One-line SL2 secure config
cfg := hsms.SecureConfig("equip:5000", hsms.RoleActive, 1)

// Or manual TLS + RBAC
cfg.TLSConfig, _ = security.LoadClientTLS("client.crt", "client.key", "ca.crt")
handler.SetPolicy(security.ReadOnlyPolicy())
handler.SetAuditor(auditor)
```

## Project Structure

```
go-factory-io/
├── api/
│   ├── rest/              REST API + SSE + E191 endpoint
│   └── grpc/              gRPC server + proto
├── clients/python/        Async/sync Python client
├── cmd/secsgem/           CLI daemon
├── examples/simulator/    Equipment simulator
├── pkg/
│   ├── bridge/mqtt/       MQTT event bridge
│   ├── driver/gem/        GEM (E30) + 300mm extensions
│   ├── message/secs2/     SECS-II codec (7M+ ops/sec)
│   ├── metrics/           Prometheus collector
│   ├── security/          TLS, RBAC, AES-GCM, audit, HSM, anomaly
│   ├── session/           Auto-reconnect
│   └── transport/
│       ├── hsms/          HSMS (E37)
│       ├── modbus/        Modbus TCP
│       └── opcua/         OPC-UA
└── test/integration/      E2E tests
```

## Testing

```bash
go test -race ./...          # All tests (70+)
go test -bench=. ./pkg/message/secs2/  # Benchmarks
go test -v ./test/integration/         # E2E with simulator
```

## Live Demo

- **[Showcase](https://factory.dashai.dev/showcase)** -- Interactive exhibit: architecture, live data, security layers
- **[Equipment Monitor](https://factory.dashai.dev/tv/equipment)** -- Real-time dashboard: OEE gauges, FOUP carriers, process job tracking

## License

MIT
