---
title: Phase 3 - Prometheus Metrics, OPC-UA, Python Client
type: feature
status: completed
created: 2026-03-27
---

# Phase 3 - Observability, Multi-Protocol, Integration

## 變更內容

### 1. Prometheus Metrics
- `/metrics` endpoint, Prometheus text format
- Counters: connections, messages sent/received/dropped, errors, alarms
- Gauges: active connections, active alarms, uptime
- GEM state info label

### 2. OPC-UA Transport Driver
- gopcua wrapper: Read/Write/Browse/Subscribe
- 支援 SecurityPolicy + SecurityMode + Certificate auth
- Data change subscription with handler callback
- ReadMultiple batch 讀取

### 3. Python Client (smart-factory-demo integration)
- Async (httpx.AsyncClient) + Sync wrapper
- Typed dataclasses: EquipmentStatus, StatusVariable, EquipmentConstant, AlarmInfo
- SSE event stream consumer
- FastAPI integration example code

## 影響範圍

### 新增
- `pkg/metrics/` — Prometheus collector + /metrics handler
- `pkg/transport/opcua/` — OPC-UA client (gopcua wrapper)
- `clients/python/factory_io.py` — Python client library
- `examples/smart-factory/integration.py` — FastAPI 整合範例

### 修改
- `cmd/secsgem/main.go` — /metrics endpoint 整合
- `go.mod` — 新增 gopcua dependency

## Checklist
- [x] Prometheus metrics collector (14 metrics)
- [x] /metrics endpoint (text/plain Prometheus format)
- [x] OPC-UA client: Connect, Read, Write, Browse
- [x] OPC-UA subscription (data change monitoring)
- [x] Python async client (httpx)
- [x] Python sync client wrapper
- [x] SSE event stream consumer
- [x] FastAPI integration example
- [x] Metrics unit tests
- [x] OPC-UA unit tests (offline)
