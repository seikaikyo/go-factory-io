---
title: Phase 1 - SECS/GEM Go Driver
type: feature
status: in-progress
created: 2026-03-27
---

# Phase 1 - SECS/GEM Go Driver

## 目標

用 Go 實作 SEMI SECS/GEM 通訊協議驅動，部署目標為 IoT gateway（樹莓派、工業 PC），供 smart-factory-demo 及未來工廠專案使用。架構預留擴充空間，日後可加入 OPC-UA、Modbus 等協議。

## 為什麼用 Go

- 單一 binary 部署，適合嵌入式 / IoT gateway（RPi Zero 2W 只有 512MB RAM）
- 交叉編譯一行搞定：`GOOS=linux GOARCH=arm64 go build`
- goroutine 原生並行，適合同時管理多台設備連線
- 冷啟動 < 50ms，記憶體 < 15MB

## Phase 1 範圍

### 1. HSMS 傳輸層（SEMI E37）

| 功能 | 說明 | 優先級 |
|------|------|--------|
| TCP 連線管理 | Active / Passive 兩種模式 | P0 |
| 控制訊息 | Select.req/rsp, Deselect, Linktest, Separate | P0 |
| Timeout 管理 | T3 (Reply), T5 (Connect Sep), T6 (Control), T7 (Not Selected), T8 (Network) | P0 |
| 自動重連 | 斷線後 exponential backoff 重連 | P0 |
| 多設備連線 | Connection pool，支援同時連多台設備 | P1 |
| TLS | 加密傳輸（工廠內網可選） | P2 |

### 2. SECS-II 訊息編解碼（SEMI E5）

| 功能 | 說明 | 優先級 |
|------|------|--------|
| 資料型別 | List, ASCII, Binary, Boolean, I1/I2/I4/I8, U1/U2/U4/U8, F4/F8 | P0 |
| Nested List | 遞迴編碼/解碼，支援任意深度巢狀 | P0 |
| Stream/Function | S{stream}F{function} 訊息路由 | P0 |
| Multi-block | 大型訊息分塊傳輸 | P1 |
| SML 解析 | SECS Message Language 文字格式解析（測試/除錯用） | P2 |

### 3. GEM 應用層（SEMI E30）

| 功能 | 說明 | 優先級 |
|------|------|--------|
| Communication State | ENABLED ↔ DISABLED, NOT COMMUNICATING → COMMUNICATING | P0 |
| Control State | OFFLINE (Eq/Host) → ONLINE (Local/Remote) | P0 |
| Equipment Constants (EC) | 讀取/寫入設備常數 | P0 |
| Status Variables (SV) | 讀取設備狀態變數 | P0 |
| Collection Events (CE) | 事件註冊、觸發、回報 | P0 |
| Reports | 定義 Report + 綁定 CE | P0 |
| Remote Command (RCMD) | S2F41 遠端指令執行 | P1 |
| Alarms | S5F1/S5F3 警報管理 | P1 |
| Process Program | S7Fx 配方上下載 | P2 |
| Spooling | 離線訊息佇列 | P2 |

### 4. 常用訊息實作（P0 優先）

| Stream/Function | 名稱 | 用途 |
|----------------|------|------|
| S1F1/S1F2 | Are You There | 連線確認 |
| S1F3/S1F4 | Selected Equipment Status | 讀取 SV |
| S1F11/S1F12 | SV Namelist | SV 清單 |
| S1F13/S1F14 | Establish Communication | 建立通訊 |
| S1F15/S1F16 | Request OFF-LINE | 離線請求 |
| S1F17/S1F18 | Request ON-LINE | 上線請求 |
| S2F13/S2F14 | Equipment Constant | 讀取 EC |
| S2F15/S2F16 | New Equipment Constant | 寫入 EC |
| S2F29/S2F30 | EC Namelist | EC 清單 |
| S2F33/S2F34 | Define Report | 定義報告 |
| S2F35/S2F36 | Link Event Report | 綁定事件 |
| S2F37/S2F38 | Enable/Disable Event | 啟停事件 |
| S2F41/S2F42 | Host Command Send | 遠端指令 |
| S5F1/S5F2 | Alarm Report | 警報回報 |
| S6F11/S6F12 | Event Report Send | 事件回報 |
| S6F15/S6F16 | Event Report Request | 事件請求 |

## 專案結構

```
go-factory-io/
├── pkg/
│   ├── transport/            # 傳輸層抽象
│   │   ├── transport.go      # Transport interface
│   │   └── hsms/             # HSMS 實作
│   │       ├── conn.go       # TCP 連線管理
│   │       ├── message.go    # HSMS 訊息結構
│   │       ├── session.go    # Session 狀態機
│   │       └── timeout.go    # T3-T8 timeout
│   ├── message/              # 訊息編解碼抽象
│   │   ├── codec.go          # Codec interface
│   │   ├── types.go          # 通用 Message 結構
│   │   └── secs2/            # SECS-II 實作
│   │       ├── encode.go     # 編碼
│   │       ├── decode.go     # 解碼
│   │       ├── item.go       # 資料項（List, ASCII, Int, Float...）
│   │       └── sml.go        # SML 文字格式（P2）
│   ├── driver/               # 設備驅動抽象
│   │   ├── driver.go         # Driver interface
│   │   └── gem/              # GEM 實作
│   │       ├── state.go      # Communication + Control 狀態機
│   │       ├── variable.go   # EC + SV 管理
│   │       ├── event.go      # CE + Report 管理
│   │       ├── command.go    # RCMD 處理
│   │       ├── alarm.go      # Alarm 管理（P1）
│   │       └── handler.go    # S{x}F{y} 訊息分派
│   └── session/              # 多設備連線管理
│       ├── manager.go        # 連線池 + 設備註冊
│       └── reconnect.go      # 重連策略
├── cmd/
│   └── secsgem/              # CLI daemon
│       └── main.go           # 啟動入口 + config 載入
├── api/                      # 對外 API（給 smart-factory-demo 呼叫）
│   ├── grpc/                 # gRPC service 定義
│   │   └── proto/
│   │       └── factory.proto
│   └── rest/                 # REST API（輕量替代方案）
│       └── handler.go
├── config/
│   └── config.go             # YAML/TOML 設定檔結構
├── examples/
│   ├── simple/               # 最簡連線範例
│   ├── simulator/            # 設備模擬器（開發測試用）
│   └── smart-factory/        # 整合 smart-factory-demo 範例
├── internal/
│   └── logger/               # structured logging (slog)
├── test/
│   ├── integration/          # 整合測試（需要模擬器）
│   └── testdata/             # 測試用的 SECS-II binary dump
├── go.mod
├── go.sum
├── Makefile                  # build, test, lint, cross-compile
├── Dockerfile                # 多階段建構
└── README.md
```

## 核心 Interface 設計

```go
// Transport — 傳輸層抽象
type Transport interface {
    Connect(ctx context.Context) error
    Send(ctx context.Context, msg []byte) error
    Receive(ctx context.Context) ([]byte, error)
    Close() error
    State() TransportState
}

// Codec — 訊息編解碼抽象
type Codec interface {
    Encode(msg *Message) ([]byte, error)
    Decode(data []byte) (*Message, error)
}

// Driver — 設備驅動抽象
type Driver interface {
    Init(cfg *Config) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    SendMessage(ctx context.Context, stream, function int, body interface{}) (*Message, error)
    OnEvent(eventID uint32, handler EventHandler)
}
```

## 跟 smart-factory-demo 整合方式

```
┌─────────────┐      gRPC / REST       ┌──────────────────┐
│ smart-factory│ ◄──────────────────── │  go-factory-io   │
│  (FastAPI)   │                        │  (Go daemon)     │
└─────────────┘                        └────────┬─────────┘
                                                │ HSMS/TCP
                                       ┌────────▼─────────┐
                                       │   Equipment #1   │
                                       │   Equipment #2   │
                                       │   Equipment #N   │
                                       └──────────────────┘
```

- Go daemon 跑在同一台 RPi / gateway 上
- FastAPI 透過 gRPC（推薦）或 REST 呼叫 Go daemon
- Go daemon 維護所有設備的 HSMS 連線
- Collection Event 透過 gRPC streaming 或 WebSocket 即時推送

## Phase 1 不包含

| 項目 | 原因 | 預計 Phase |
|------|------|-----------|
| OPC-UA driver | 有現成 gopcua library，整合即可 | Phase 2 |
| Modbus TCP/RTU | 成熟套件多，加一個 transport 實作 | Phase 2 |
| MQTT bridge | 需要先定義 topic 規範 | Phase 2 |
| Web UI | smart-factory-demo 已有前端 | 不需要 |
| 資料庫儲存 | 由 smart-factory-demo 後端處理 | 不需要 |

## 影響範圍

### 新增（go-factory-io repo）
- `pkg/transport/hsms/` — HSMS 傳輸層
- `pkg/message/secs2/` — SECS-II 編解碼
- `pkg/driver/gem/` — GEM 狀態機
- `pkg/session/` — 多設備連線管理
- `cmd/secsgem/` — CLI daemon
- `api/` — gRPC + REST API
- `examples/simulator/` — 設備模擬器

### 修改（smart-factory-demo）
- 新增 Go daemon 呼叫的 API client（Python gRPC stub）
- 設備監控頁面串接即時資料

## 測試計畫

| 層級 | 方式 | 涵蓋範圍 |
|------|------|---------|
| 單元測試 | `go test` | SECS-II 編解碼、HSMS 訊息解析、GEM 狀態轉移 |
| 整合測試 | 內建模擬器 | 完整 HSMS session 建立 → GEM 通訊 → 事件回報 |
| 手動測試 | 接實體設備或第三方模擬器 | 真實場景驗證 |
| 效能測試 | benchmark | 編解碼吞吐量、並行連線數、記憶體使用 |

### 測試目標
- SECS-II 編解碼：100K msg/sec（單核）
- HSMS 連線：穩定維持 100+ 並行設備
- 記憶體：daemon 常駐 < 30MB（無負載）
- 重連：斷線後 < 3 秒恢復

## Checklist

### HSMS 傳輸層
- [ ] TCP Active mode 連線
- [ ] TCP Passive mode 監聽
- [ ] Select.req / Select.rsp
- [ ] Deselect.req / Deselect.rsp
- [ ] Linktest.req / Linktest.rsp
- [ ] Separate
- [ ] T3/T5/T6/T7/T8 timeout
- [ ] 斷線自動重連（exponential backoff）
- [ ] 單元測試覆蓋率 > 80%

### SECS-II 編解碼
- [ ] 所有資料型別 encode/decode
- [ ] Nested List 遞迴處理
- [ ] Stream/Function 路由
- [ ] 邊界測試（空 list、最大深度、溢位）
- [ ] Benchmark > 100K msg/sec

### GEM 狀態機
- [ ] Communication State Model
- [ ] Control State Model
- [ ] EC 讀寫（S2F13-S2F16）
- [ ] SV 讀取（S1F3/S1F4）
- [ ] CE + Report 定義與綁定（S2F33-S2F38）
- [ ] Event Report 發送（S6F11/S6F12）
- [ ] 狀態轉移單元測試

### 設備模擬器
- [ ] 模擬 Equipment 端 HSMS passive 模式
- [ ] 回應 S1F1, S1F13 等基本訊息
- [ ] 定時產生 Collection Event
- [ ] 可設定 EC/SV 值

### API + 整合
- [ ] gRPC service 定義（.proto）
- [ ] REST API fallback
- [ ] smart-factory-demo 呼叫範例
- [ ] Docker 多階段建構
- [ ] Makefile（build / test / lint / cross-compile）

### 文件
- [ ] README（安裝、使用、設定）
- [ ] API 文件
- [ ] 架構圖
