# go-factory-io

SECS/GEM driver for semiconductor equipment communication, written in Go.

Implements SEMI E5 (SECS-II), E30 (GEM), and E37 (HSMS) standards. Designed for deployment on IoT gateways, Raspberry Pi, or industrial PCs.

## Features

- **SECS-II (E5)**: Full 14-type encode/decode, nested list support, 7M+ ops/sec
- **HSMS (E37)**: Active/Passive TCP, Select/Deselect/Linktest, T3-T8 timeouts
- **GEM (E30)**: Communication + Control state machines, EC/SV, Collection Events, Reports, Alarms, Remote Commands
- **Auto-reconnect**: Exponential backoff with configurable max retries
- **REST API**: HTTP endpoints for integration with external services (FastAPI, etc.)
- **SSE**: Real-time event streaming via Server-Sent Events
- **Equipment simulator**: Built-in simulator for development and testing
- **Cross-platform**: Single static binary, cross-compiles to Linux ARM64/AMD64

## Quick Start

```bash
# Build
go build -o bin/secsgem ./cmd/secsgem/

# Run simulator (HSMS on :5000, REST API on :8080)
./bin/secsgem simulate

# Connect as host from another terminal
./bin/secsgem connect localhost:5000
```

## Cross-Compile for Raspberry Pi

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o secsgem ./cmd/secsgem/
# Result: ~2.7MB static binary, copy to RPi and run directly
```

## Docker

```bash
docker build -t secsgem .
docker run -p 5000:5000 -p 8080:8080 secsgem
```

## REST API

The simulator exposes an HTTP API for integration with other services.

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

Response format follows `{ success: boolean, data?: T, error?: { code, message } }`.

### Examples

```bash
# Equipment status
curl http://localhost:8080/api/status

# Read all status variables
curl http://localhost:8080/api/sv

# Read temperature
curl http://localhost:8080/api/sv/1002

# Set equipment constant
curl -X PUT http://localhost:8080/api/ec/1 -d '{"value": 400.0}'

# Execute remote command
curl -X POST http://localhost:8080/api/command -d '{"command":"START","params":{}}'

# Subscribe to real-time events (SSE)
curl -N http://localhost:8080/api/events
```

## Go Library Usage

```go
package main

import (
    "context"
    "log/slog"

    "github.com/dashfactory/go-factory-io/pkg/driver/gem"
    "github.com/dashfactory/go-factory-io/pkg/message/secs2"
    "github.com/dashfactory/go-factory-io/pkg/transport/hsms"
)

func main() {
    logger := slog.Default()
    cfg := hsms.DefaultConfig("192.168.1.100:5000", hsms.RoleActive, 1)
    session := hsms.NewSession(cfg, logger)

    ctx := context.Background()
    session.Connect(ctx)
    session.Select(ctx)

    // Send S1F13 Establish Communication
    body := secs2.NewList(
        secs2.NewASCII("HOST"),
        secs2.NewASCII("1.0.0"),
    )
    data, _ := secs2.Encode(body)
    msg := hsms.NewDataMessage(1, 1, 13, true, 0, data)
    reply, _ := session.SendMessage(ctx, msg)

    // Decode reply
    item, _ := secs2.Decode(reply.Data)
    logger.Info("Equipment response", "body", item.String())

    session.Close()
}
```

### Auto-Reconnect

```go
import "github.com/dashfactory/go-factory-io/pkg/session"

cfg := hsms.DefaultConfig("192.168.1.100:5000", hsms.RoleActive, 1)
reconnCfg := session.DefaultReconnectConfig()

ms := session.NewManagedSession(cfg, reconnCfg, logger)
ms.OnConnect(func(s *hsms.Session) {
    logger.Info("Connected to equipment")
})
ms.Start(ctx)  // Auto-reconnects on disconnect
```

### Equipment Simulator

```go
import "github.com/dashfactory/go-factory-io/examples/simulator"

cfg := simulator.DefaultEquipmentConfig()
cfg.ListenAddress = ":5000"
eq := simulator.NewEquipment(cfg, logger)
eq.Start(ctx)

// Register custom SV
eq.Handler().Variables().DefineSVDynamic(2001, "SensorA", "mV", func() interface{} {
    return readSensor()
})

// Register custom RCMD
eq.Handler().Commands().Register("PP_SELECT", func(ctx context.Context, params []gem.CommandParam) gem.CommandStatus {
    // Process program selection logic
    return gem.CommandOK
})
```

## Architecture

```
                                  go-factory-io
                    ┌──────────────────────────────────────┐
                    │                                      │
 External           │  ┌──────────┐    ┌────────────────┐  │
 Services  ◄──REST──┤  │ REST API │◄──►│  GEM Handler   │  │
 (FastAPI)          │  └──────────┘    │  - State       │  │
                    │                  │  - Variables   │  │
                    │  ┌──────────┐    │  - Events      │  │
 Browser   ◄──SSE───┤  │ SSE      │◄──►│  - Alarms     │  │
                    │  └──────────┘    │  - Commands    │  │
                    │                  └───────┬────────┘  │
                    │                          │           │
                    │                  ┌───────▼────────┐  │
                    │                  │  SECS-II Codec │  │
                    │                  └───────┬────────┘  │
                    │                          │           │
                    │                  ┌───────▼────────┐  │
                    │                  │  HSMS Session  │  │
 Equipment ◄──TCP───┤                  │  - Select      │  │
                    │                  │  - Linktest    │  │
                    │                  │  - Reconnect   │  │
                    │                  └────────────────┘  │
                    └──────────────────────────────────────┘
```

## Supported SECS Messages

| Stream/Function | Name | Direction |
|----------------|------|-----------|
| S1F1/F2 | Are You There | H→E / E→H |
| S1F3/F4 | Selected Equipment Status | H→E / E→H |
| S1F11/F12 | SV Namelist | H→E / E→H |
| S1F13/F14 | Establish Communication | H→E / E→H |
| S1F15/F16 | Request OFF-LINE | H→E / E→H |
| S1F17/F18 | Request ON-LINE | H→E / E→H |
| S2F13/F14 | Equipment Constant Request | H→E / E→H |
| S2F15/F16 | New Equipment Constant | H→E / E→H |
| S2F29/F30 | EC Namelist | H→E / E→H |
| S2F33/F34 | Define Report | H→E / E→H |
| S2F35/F36 | Link Event Report | H→E / E→H |
| S2F37/F38 | Enable/Disable Event | H→E / E→H |
| S2F41/F42 | Host Command Send (RCMD) | H→E / E→H |
| S5F1/F2 | Alarm Report Send | E→H / H→E |
| S5F3/F4 | Enable/Disable Alarm | H→E / E→H |
| S5F5/F6 | List Alarms | H→E / E→H |
| S5F7/F8 | List Enabled Alarms | H→E / E→H |
| S6F11/F12 | Event Report Send | E→H / H→E |

## Project Structure

```
go-factory-io/
├── api/rest/          REST API handlers + tests
├── cmd/secsgem/       CLI daemon entry point
├── examples/simulator Equipment simulator
├── pkg/
│   ├── driver/gem/    GEM state machine, variables, events, alarms, commands
│   ├── message/secs2/ SECS-II encode/decode
│   ├── session/       Auto-reconnect managed session
│   └── transport/hsms HSMS TCP transport
└── test/integration/  End-to-end tests
```

## Testing

```bash
# All tests
go test -race ./...

# Benchmarks
go test -bench=. -benchmem ./pkg/message/secs2/

# Integration tests only
go test -v ./test/integration/
```

## License

MIT
