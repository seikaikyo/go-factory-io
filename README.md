# go-factory-io

Open-source SECS/GEM equipment driver in Go. Covers 12 SEMI standards, 5 communication protocols, and IEC 62443 SL4 security -- in a single static binary that runs on a Raspberry Pi.

**[Live Demo](https://factory.dashai.dev/tv/equipment)** | [API Docs](#rest-api) | [Go Library](#go-library-usage)

## Why this project exists

二十幾年前我從自由軟體社群開始寫程式，經營 [phpBB 繁體中文](https://phpbb-tw.net/phpbb/) 論壇，翻譯文件、改程式碼、幫人解問題。那時候覺得「把東西做出來放到網路上讓大家用」是一件很自然的事，到現在還是這樣。

SEMI SECS/GEM 的規格書攤開來超過兩千頁。讀這些文件本身就是一件有趣的事 -- 狀態機怎麼轉、訊息怎麼編、設備跟 host 之間那套嚴格的握手流程，每一層都有設計的道理在。我喜歡研究這些東西，也擅長把不同層的東西整合在一起，就想說來寫一個盡量完整的 Go 實作，看能不能對社群有點貢獻。

開源社群已經有很好的基礎。[secs4net](https://github.com/mkjeff/secs4net)（C#, 590+ stars）在 .NET 生態經過大量生產驗證，[secsgem](https://github.com/bparzella/secsgem)（Python）把 GEM 狀態機做得很完整，[secs4java8](https://github.com/kenta-shimizu/secs4java8) 和 [secs4go](https://github.com/younglifestyle/secs4go) 分別在 Java 和 Go 提供了穩定的通訊層。這些專案讓開發者能跟設備講上話，打下了整個生態的地基。

go-factory-io 想做的，是在這個地基上往上蓋。把 300mm 晶圓廠會用到的標準（E87 Carrier、E40 Process Job、E90 Substrate Tracking、E94 Control Job、E116 OEE）、工廠裡常見的其他協定（OPC-UA、MQTT、Modbus TCP）、還有近年越來越被重視的資安要求（IEC 62443），整合進同一個 binary。讀 secs4net 和 secsgem 的原始碼學到很多，HSMS 連線管理的設計模式有不少是從那邊吸收來的。

### 涵蓋範圍

```
  go-factory-io 整合的 SEMI 標準與協定
  ────────────────────────────────────────
  通訊層          E5 SECS-II 編解碼
                  E30 GEM 狀態機
                  E37 HSMS 傳輸 (TLS/mTLS)

  300mm 擴充      E87  Carrier Management
                  E40  Process Jobs
                  E90  Substrate Tracking
                  E94  Control Jobs
                  E116 EPT / OEE

  安全與合規      E187/E191 Cybersecurity
                  IEC 62443 SL4

  多協定橋接      OPC-UA, MQTT, Modbus TCP
                  REST, gRPC, SSE

  可觀測性        Prometheus metrics
```

### 致謝與相關資源

這個專案受益於開源社群的長期積累。以下專案在 SECS/GEM 領域各有深耕，閱讀它們的原始碼讓我學到很多：

| 專案 | 語言 | 學到什麼 |
|------|------|---------|
| [secs4net](https://github.com/mkjeff/secs4net) | C# | HSMS 連線管理的設計模式、生產環境的邊界處理 |
| [secsgem](https://github.com/bparzella/secsgem) | Python | GEM 狀態機的完整實作結構 |
| [secs4java8](https://github.com/kenta-shimizu/secs4java8) | Java | SECS-I + HSMS-GS 雙模支援的架構思路 |
| [secs4go](https://github.com/younglifestyle/secs4go) | Go | Go 語言處理 SECS-II binary 編碼的慣用寫法 |

go-factory-io 的方向是把通訊層往上延伸 -- 整合 300mm 晶圓廠需要的載體管理、製程追蹤、設備效率分析，再加上工業資安的需求。算是在不同的層次做嘗試。

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

- **[Showcase](https://factory.dashai.dev/showcase)** -- 互動式展示頁，7 個區塊呈現架構、即時數據、安全層
- **[Equipment Monitor](https://factory.dashai.dev/tv/equipment)** -- 即時設備監控儀表板，OEE gauge、FOUP 載體、Process Job 追蹤

## License

MIT
