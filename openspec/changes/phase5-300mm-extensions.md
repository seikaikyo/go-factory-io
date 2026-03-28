---
title: Phase 5 - 300mm GEM Extensions (E87/E40/E90/E94/E116)
type: feature
status: completed
created: 2026-03-28
---

# Phase 5 - 300mm GEM Extensions

## 目標

實作 SEMI 300mm 晶圓廠設備通訊標準，讓 go-factory-io 從 demo 等級升級為可部署的 300mm 設備 driver。
沒有 E87/E40 的 SECS/GEM driver 在 300mm 廠無法使用。

## 規範依據

| 規範 | 用途 |
|------|------|
| SEMI E87 | Carrier Management Service (CMS) - FOUP/載體管理 |
| SEMI E40 | Process Job Management - 製程作業管理 |
| SEMI E90 | Substrate Tracking - 晶圓片追蹤 |
| SEMI E94 | Control Job Management - 控制作業排程 |
| SEMI E116 | Equipment Performance Tracking (EPT) - OEE 指標 |

## 變更內容

### 1. E87 - Carrier Management Service

載體（FOUP）在設備上的完整生命週期管理。

**Carrier State Model:**
```
           ┌──────────────────────────────────────┐
           │                                      │
     ──► NotAccessed ──► WaitingForHost ──► NotRead
           │                                  │
           │              ┌───────────────────┘
           │              ▼
           │          InAccess ──► CarrierComplete
           │              │              │
           │              ▼              ▼
           └──────── Stopped ◄──── ReadyToUnload
```

States: NotAccessed, WaitingForHost, InAccess, CarrierComplete, Stopped, ReadyToUnload

**Load Port State Model:**
```
OutOfService ──► InService
    ▲                │
    └────────────────┘

InService substates:
  TransferBlocked ──► ReadyToLoad ──► ReadyToUnload
                          │
                     TransferReady
```

### 2. E40 - Process Job Management

製程作業的建立、執行、完成流程。

**Process Job State Model:**
```
Queued ──► SettingUp ──► WaitingForStart ──► Processing
  │            │                                 │
  │            ▼                                 ▼
  └──► Aborting ◄────────────────────── ProcessComplete
         │                                       │
         ▼                                       ▼
      Aborted                                 Stopping
                                                 │
                                                 ▼
                                              Stopped
```

### 3. E90 - Substrate Tracking

晶圓片位置追蹤和物料移動事件。

- Substrate ID 管理 (lot ID + slot)
- 位置追蹤: source port → process chamber → destination port
- Material movement events (CEID)
- Slot map (25 slots per FOUP)

### 4. E94 - Control Job Management

控制作業排程，將多個 Process Job 組織為一個 Control Job。

**Control Job State Model:**
```
Queued ──► Selected ──► WaitingForStart ──► Executing
  │           │                                │
  │           ▼                                ▼
  └──► Pausing ◄───────────────────────── Completed
         │                                     │
         ▼                                     ▼
       Paused                              Stopping
                                               │
                                               ▼
                                            Stopped
```

### 5. E116 - Equipment Performance Tracking

設備效能追蹤，用於計算 OEE (Overall Equipment Effectiveness)。

**Equipment States:**
- Idle, Busy, Blocked, BusyAndBlocked
- StandbyScheduled, StandbyUnscheduled
- EngineeringScheduled, EngineeringUnscheduled
- Down (Scheduled/Unscheduled)
- NonScheduled

## 影響範圍

### 新增
- `pkg/driver/gem/carrier.go` — E87 Carrier state model + Load Port
- `pkg/driver/gem/carrier_test.go`
- `pkg/driver/gem/processjob.go` — E40 Process Job state model + manager
- `pkg/driver/gem/processjob_test.go`
- `pkg/driver/gem/substrate.go` — E90 Substrate tracking
- `pkg/driver/gem/substrate_test.go`
- `pkg/driver/gem/controljob.go` — E94 Control Job state model + manager
- `pkg/driver/gem/controljob_test.go`
- `pkg/driver/gem/ept.go` — E116 Equipment Performance Tracking
- `pkg/driver/gem/ept_test.go`

### 修改
- `pkg/driver/gem/handler.go` — 新增 S3/S16 message handlers, 整合新 managers
- `README.md` — 300mm 文件

## 測試計畫

| 項目 | 方式 |
|------|------|
| Carrier state transitions | 驗證所有合法/非法狀態轉換 |
| Load port state model | Port InService/OutOfService 切換 |
| Process job lifecycle | Create → Setup → Process → Complete |
| Process job abort/stop | 各狀態中斷驗證 |
| Substrate slot map | 25 slots 追蹤正確性 |
| Material movement | Source → Chamber → Destination |
| Control job scheduling | 多 PJ 組合為 CJ |
| EPT state tracking | 狀態時間累計 + OEE 計算 |

## Checklist

### E87 - Carrier Management
- [x] CarrierState enum + state machine (6 states)
- [x] LoadPortState enum + state machine (5 states)
- [x] Carrier struct (ID, 25-slot map, state, timestamps)
- [x] LoadPort struct (port ID, state, carrier binding)
- [x] CarrierManager (bind, unbind, proceed, access, stop, resume, unload)
- [x] Handler 整合 (Carriers() accessor)
- [x] Unit tests (7 tests: lifecycle, stop/resume, invalid transition, port states, slot map, callback, list)

### E40 - Process Job
- [x] ProcessJobState enum + state machine (9 states)
- [x] ProcessJob struct (ID, recipe, carrier, slots, params, timestamps)
- [x] ProcessJobManager (create, setup, start, complete, abort, stop, remove)
- [x] Handler 整合 (ProcessJobs() accessor)
- [x] Unit tests (8 tests: lifecycle, abort, stop, invalid, duplicate, remove, callback, list)

### E90 - Substrate Tracking
- [x] SubstrateLocation struct (Type + ID + Slot)
- [x] 25-slot FOUP mapping via Carrier.SlotMap
- [x] Material movement tracking (History log, auto state update)
- [x] Handler 整合 (Substrates() accessor)
- [x] Unit tests (7 tests: movement, getAt, listByCarrier, reject, callback, remove, unknown)

### E94 - Control Job
- [x] ControlJobState enum + state machine (9 states)
- [x] ControlJob struct (ID, process jobs list, priority, timestamps)
- [x] ControlJobManager (create, select, execute, pause, complete, stop, remove)
- [x] Handler 整合 (ControlJobs() accessor)
- [x] Unit tests (7 tests: lifecycle, pause, stop, invalid, remove, callback, list)

### E116 - EPT
- [x] EPTState enum (11 states)
- [x] EPTTracker (state transitions, time accounting, transition log)
- [x] OEE calculation (availability, performance, quality)
- [x] Unit tests (9 tests: transitions, no-op, counting, OEE, no units, no schedule, durations, reset, callback)
