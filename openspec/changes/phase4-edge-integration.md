---
title: Phase 4 - Edge Integration (MQTT, Webhook, gRPC, Modbus)
type: feature
status: completed
created: 2026-03-28
---

# Phase 4 - Edge Integration

## 目標

補齊 go-factory-io 在邊緣部署場景的整合能力。目前只有 REST API（pull），缺少主動推送（push）機制。
工廠實際架構：設備 → Edge Gateway (go-factory-io) → MQTT Broker → MES/SCADA/Historian。

## 變更內容

### 1. MQTT Bridge — 設備事件推送到 MQTT Broker

將 GEM 事件（Collection Event、Alarm、SV 變化）自動 publish 到 MQTT topic。

```go
type MQTTBridgeConfig struct {
    BrokerURL    string // tcp://broker:1883 or ssl://broker:8883
    ClientID     string
    Username     string
    Password     string
    TopicPrefix  string // e.g. "factory/eq01"
    QoS          byte   // 0, 1, 2
    TLSConfig    *tls.Config
    RetainEvents bool   // retain last event per topic
}
```

Topic 結構：
- `{prefix}/status` — 設備狀態變更
- `{prefix}/event/{ceid}` — Collection Event
- `{prefix}/alarm/{alid}` — Alarm set/clear
- `{prefix}/sv/{svid}` — Status Variable (on-change or periodic)

payload 用 JSON，跟 REST API 回應格式一致。

### 2. Security Event Webhook — 安全事件外推

Phase 2 P1 遺留。Auditor 已有 `EventHandler` 介面，加 HTTP POST 和 Syslog 兩種 sink。

```go
type WebhookConfig struct {
    URL         string        // HTTP POST endpoint
    Headers     map[string]string
    Timeout     time.Duration
    RetryCount  int
    BatchSize   int           // batch N events per request, 0 = immediate
}

type SyslogConfig struct {
    Network  string // "tcp" or "udp"
    Address  string // "siem.factory.local:514"
    Facility syslog.Priority
    Tag      string
}
```

### 3. gRPC API — 高頻 M2M 通訊

REST API 的 gRPC 對等實作，適合 MES/SCADA 高頻輪詢。

```protobuf
service FactoryIO {
    rpc GetStatus(Empty) returns (EquipmentStatus);
    rpc GetStatusVariables(SVRequest) returns (SVResponse);
    rpc GetEquipmentConstants(ECRequest) returns (ECResponse);
    rpc SetEquipmentConstant(SetECRequest) returns (SetECResponse);
    rpc GetAlarms(AlarmsRequest) returns (AlarmsResponse);
    rpc ExecuteCommand(CommandRequest) returns (CommandResponse);
    rpc StreamEvents(StreamRequest) returns (stream EventMessage);
}
```

### 4. Modbus TCP Driver — PLC/感測器通訊

實作 Transport interface，讓 go-factory-io 也能讀寫 Modbus 裝置（PLC、溫控器、電表）。

```go
type ModbusConfig struct {
    Address    string        // "192.168.1.10:502"
    SlaveID    byte
    Timeout    time.Duration
    MaxRetries int
}

type ModbusClient interface {
    ReadCoils(addr, quantity uint16) ([]bool, error)
    ReadDiscreteInputs(addr, quantity uint16) ([]bool, error)
    ReadHoldingRegisters(addr, quantity uint16) ([]uint16, error)
    ReadInputRegisters(addr, quantity uint16) ([]uint16, error)
    WriteSingleCoil(addr uint16, value bool) error
    WriteSingleRegister(addr, value uint16) error
    WriteMultipleCoils(addr uint16, values []bool) error
    WriteMultipleRegisters(addr uint16, values []uint16) error
}
```

## 影響範圍

### 新增
- `pkg/bridge/mqtt/` — MQTT bridge (publish GEM events)
- `pkg/security/webhook.go` — HTTP webhook + Syslog event sink
- `pkg/security/webhook_test.go` — Webhook 測試
- `pkg/transport/modbus/` — Modbus TCP client
- `pkg/transport/modbus/modbus_test.go` — Modbus 測試
- `api/grpc/proto/factoryio.proto` — gRPC service 定義
- `api/grpc/server.go` — gRPC server 實作
- `api/grpc/server_test.go` — gRPC 測試

### 修改
- `go.mod` — 新增 paho.mqtt, google.golang.org/grpc
- `cmd/secsgem/main.go` — MQTT bridge + gRPC server 啟動
- `README.md` — 新增 MQTT/gRPC/Modbus 文件

## 測試計畫

| 項目 | 方式 |
|------|------|
| MQTT publish | 本機 mosquitto broker, 驗證 topic + payload |
| MQTT TLS | TLS broker 連線測試 |
| MQTT reconnect | broker 斷線後自動重連 |
| Webhook | httptest server 驗證 POST body |
| Webhook retry | 模擬 5xx 回應，驗證重試邏輯 |
| Syslog | UDP syslog receiver 驗證格式 |
| gRPC unary | 各 RPC method 回應正確 |
| gRPC stream | StreamEvents 推送驗證 |
| Modbus read | 模擬 Modbus server, 驗證 register 讀取 |
| Modbus write | 寫入 coil/register 驗證 |
| Modbus error | 錯誤碼處理 (illegal address, slave failure) |

## Checklist

### MQTT Bridge
- [x] MQTTBridgeConfig 結構
- [x] Connect / Reconnect / Disconnect
- [x] Publish GEM events (Collection Event, Alarm, SV change)
- [x] Topic 結構 ({prefix}/status, /event/{ceid}, /alarm/{alid}, /sv/{svid})
- [x] JSON payload (與 REST API 格式一致)
- [x] TLS 支援
- [x] Unit tests

### Security Event Webhook
- [x] HTTP POST webhook sink (EventHandler)
- [x] Configurable headers, timeout, retry
- [x] Batch mode (optional)
- [x] Syslog sink (TCP/UDP)
- [x] Unit tests

### gRPC API
- [x] Proto 定義 (service + messages)
- [x] Server 實作 (7 unary + 1 stream)
- [x] Bearer token interceptor
- [x] Hand-written types (no protoc dependency)

### Modbus TCP Driver
- [x] Modbus TCP client (connect, read, write)
- [x] Function codes: FC01/02/03/04/05/06/15/16
- [x] Error handling (exception responses)
- [x] Pure Go (no external dependency)
- [x] Unit tests (mock TCP server)

### Integration
- [x] GEM Handler event hooks (OnEventSent, OnAlarmSent)
- [x] main.go CLI flags (--mqtt-broker, --grpc-addr, --webhook-url, --syslog-addr)
- [x] README 更新
- [x] Cross-compile ARM64 (12MB with gRPC deps)
