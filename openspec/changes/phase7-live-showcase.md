---
title: Phase 7 - go-factory-io Live Showcase (smart-factory-demo integration)
type: feature
status: planned
created: 2026-03-28
---

# Phase 7 - go-factory-io Live Showcase

## 目標

讓訪客在 factory.dashai.dev 上直接看到 go-factory-io 的完整能力。
兩個交付物：B（擴充現有 TV Equipment）+ C（全新獨立展示頁）。

## 跨 Repo 影響

| Repo | 修改 |
|------|------|
| smart-factory-demo | 前端 UI (Vue 3 + PrimeVue) |
| dashai-api | 後端模擬器 API (FastAPI) |
| go-factory-io | 無（本 spec 為規劃文件） |

## B - 擴充 TV Equipment (`/tv/equipment`)

在現有頁面下方新增三個區塊，讓 Phase 5 功能可視化。

### B1: FOUP 載體狀態面板

```
┌─ Load Port 1 ──────────┐  ┌─ Load Port 2 ──────────┐
│  ┌─────────────────┐    │  │                         │
│  │  FOUP-001       │    │  │    READY TO LOAD        │
│  │  LOT-A / 25 pcs │    │  │    (empty)              │
│  │  ██████████████  │    │  │                         │
│  │  IN_ACCESS       │    │  │                         │
│  └─────────────────┘    │  │                         │
│  Port: TRANSFER_READY   │  │  Port: READY_TO_LOAD    │
└─────────────────────────┘  └─────────────────────────┘
```

- 兩個 Load Port 卡片，顯示 carrier state + port state
- Slot map: 25 格的水平條，顏色表示 occupied/empty/error
- Carrier state badge 隨狀態切換動態變色（綠=InAccess, 藍=Complete, 灰=NotAccessed）
- 模擬器每 5 秒自動推進 carrier lifecycle

### B2: OEE 儀表板

```
┌─────────────────────────────────────────────┐
│   ┌───┐     ┌───┐     ┌───┐     ┌───┐      │
│   │ A │     │ P │     │ Q │     │OEE│      │
│   │92%│     │87%│     │99%│     │79%│      │
│   └───┘     └───┘     └───┘     └───┘      │
│  Availability Performance Quality  Overall   │
│                                              │
│  State: BUSY   Units: 1,247   Defects: 12   │
└──────────────────────────────────────────────┘
```

- 四個圓形 gauge（animated SVG arc）
- 底部顯示目前 EPT state、產出數、不良數
- 數值每 3 秒更新，gauge 有平滑過渡動畫

### B3: Process Job 時間軸

```
┌──────────────────────────────────────────────────────────┐
│ PJ-001 [RECIPE-A]  ████████████████████████░░░░  85%    │
│ PJ-002 [RECIPE-B]  ████████████░░░░░░░░░░░░░░░  45%    │
│ PJ-003 [RECIPE-A]  ██░░░░░░░░░░░░░░░░░░░░░░░░░  SETUP  │
│ PJ-004 [RECIPE-C]  ░░░░░░░░░░░░░░░░░░░░░░░░░░░  QUEUED │
└──────────────────────────────────────────────────────────┘
```

- 進度條 + 狀態 badge
- 進度條有動畫填充效果
- Complete 的 job 淡出，新 job 從底部滑入

### B 後端 API（dashai-api 新增）

| Endpoint | 回傳 |
|----------|------|
| GET /equipment/carriers | Carrier list + port states |
| GET /equipment/oee | EPT state + OEE metrics |
| GET /equipment/jobs | Process job list + progress |

模擬器邏輯：
- Carrier: 每 5 秒推進 state（NotAccessed → WaitingForHost → InAccess → ... → ReadyToUnload → 新 carrier）
- OEE: Busy/Idle 隨機切換，累計 availability/performance/quality
- Jobs: 4 個 PJ 同時跑，各自進度隨時間遞增

---

## C - 全新展示頁 (`/showcase` 或 `/go-factory-io`)

一頁式互動展示，展現 go-factory-io 所有差異化功能。這是給訪客的第一印象頁面。

### 設計原則

- 暗色主題（深藍/黑底），工業科技感
- 每個區塊佔滿一個 viewport section，scroll-snap 切換
- 所有數據即時跳動（3 秒 poll），不是靜態截圖
- 動畫用 CSS transitions + SVG，不加重 JS 框架

### Section 1: Hero - Equipment Cross-Section

```
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│                    go-factory-io                             │
│         Open-Source SECS/GEM Driver for Semiconductor        │
│                                                              │
│    ┌──────┐     ┌────────┐     ┌──────┐     ┌──────┐       │
│    │ FOUP ├────►│ Robot  ├────►│ Etch ├────►│ FOUP │       │
│    │ Port1│◄────┤  Arm   │◄────┤Chamber│◄────┤ Port2│       │
│    └──────┘     └────────┘     └──────┘     └──────┘       │
│         ▲                          │                         │
│         │    ◄── wafer flow ──►    │                         │
│         └──── SECS/GEM data ───────┘                         │
│                                                              │
│   [View Live Demo]              [GitHub]                     │
└──────────────────────────────────────────────────────────────┘
```

- 設備剖面圖用 SVG 繪製
- 晶圓（小圓點）沿路徑動態移動（CSS animation path）
- SECS/GEM 資料封包用虛線脈衝表示
- 背景微粒子效果（純 CSS，不用 canvas）

### Section 2: Live GEM State Machine

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Communication          Control                      │
│  ┌──────────┐          ┌──────────────┐             │
│  │ DISABLED │          │ OFFLINE/EQUIP│             │
│  └────┬─────┘          └──────┬───────┘             │
│       ▼                       ▼                      │
│  ┌──────────┐          ┌──────────────┐             │
│  │ WAIT_CRA │──────►   │ONLINE/REMOTE │ ◄── LIVE   │
│  └────┬─────┘          └──────────────┘             │
│       ▼                                              │
│  ┌──────────────┐    Current: COMMUNICATING          │
│  │COMMUNICATING │◄── ████████████████  ONLINE        │
│  └──────────────┘                                    │
│                                                      │
│  S1F13 ── S1F14 ── S2F41 ── S6F11  (message flow)  │
└──────────────────────────────────────────────────────┘
```

- State machine 圖用 SVG，當前狀態 glow 效果（box-shadow pulse）
- 箭頭用 animated dash offset 表示資料流向
- 底部顯示即時 SECS message 交換（S1F13, S6F11 等），像 terminal 一樣滾動

### Section 3: FOUP Carrier Animation

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  ┌─────────────┐          ┌─────────────┐           │
│  │  Load Port 1│          │  Load Port 2│           │
│  │  ┌───────┐  │          │  ┌───────┐  │           │
│  │  │ FOUP  │  │    ───►  │  │       │  │           │
│  │  │ ▓▓▓▓▓ │  │  robot   │  │       │  │           │
│  │  │ ▓▓▓▓▓ │  │   arm    │  │       │  │           │
│  │  │ ▓▓▓░░ │  │          │  │       │  │           │
│  │  └───────┘  │          │  └───────┘  │           │
│  │  IN_ACCESS  │          │ READY_LOAD  │           │
│  └─────────────┘          └─────────────┘           │
│                                                      │
│  Slot Map: ██ ██ ██ ██ ██ ██ ██ ░░ ░░ ░░ ... (25)  │
│  Carrier: FOUP-001  Lot: LOT-A  Wafers: 18/25      │
└──────────────────────────────────────────────────────┘
```

- FOUP 用 CSS 3D transform 呈現（微傾斜的長方體）
- 晶圓 slot 用色塊排列，occupied = 亮色，empty = 暗色
- Robot arm 動畫：拾取晶圓 → 移動 → 放入 chamber → 返回
- Carrier state badge 會隨生命週期自動推進
- 整個動畫循環約 15 秒完成一輪

### Section 4: OEE Real-Time Dashboard

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│     ╭───╮       ╭───╮       ╭───╮       ╭───╮      │
│    ╱ 92% ╲     ╱ 87% ╲     ╱ 99% ╲     ╱ 79% ╲     │
│   │  ███  │   │  ███  │   │  ███  │   │  ███  │    │
│    ╲     ╱     ╲     ╱     ╲     ╱     ╲     ╱     │
│     ╰───╯       ╰───╯       ╰───╯       ╰───╯      │
│   Availability  Performance   Quality     OEE       │
│                                                      │
│  EPT State: ████ BUSY ████████████████████████       │
│  Timeline:  IDLE ██ BUSY ████████ IDLE █ BUSY ██     │
│                                                      │
│  Units Processed: 1,247    Defects: 12 (0.96%)      │
└──────────────────────────────────────────────────────┘
```

- 四個圓形 gauge 用 SVG arc + CSS transition 動畫
- 數值每 3 秒平滑遞增（CSS counter 或 requestAnimationFrame）
- 底部 EPT timeline 是水平色條，即時往右延伸
- State 切換時有顏色脈衝效果

### Section 5: Multi-Protocol Data Flow

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Equipment          go-factory-io         MES/SCADA  │
│  ┌──────┐          ┌──────────┐          ┌───────┐  │
│  │ HSMS ├─ TCP ───►│          ├─ REST ──►│FastAPI│  │
│  │      │          │          ├─ gRPC ──►│       │  │
│  │ OPC  ├─ UA ────►│          ├─ MQTT ──►│Broker │  │
│  │      │          │          ├─ SSE ───►│Browser│  │
│  │ PLC  ├─Modbus──►│          │          │       │  │
│  └──────┘          └──────────┘          └───────┘  │
│                                                      │
│  ◄─── Southbound ──►  ◄──── Northbound ────►        │
│   5 protocols in         5 protocols out              │
│   one binary             one binary                   │
└──────────────────────────────────────────────────────┘
```

- 資料封包沿連線動態移動（小方塊從左到右，不同顏色代表不同協定）
- 每條連線有流量指示（粗細或亮度代表 throughput）
- Hover 某條連線高亮 + 顯示 tooltip（封包數/秒、延遲等）

### Section 6: Security Layers

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  IEC 62443 Security Level 4                          │
│                                                      │
│  ┌─ Transport ───────────────────────────────────┐   │
│  │  TLS 1.3  |  mTLS  |  IP Allowlist           │   │
│  │  ┌─ Access ────────────────────────────────┐  │   │
│  │  │  RBAC  |  S/F Policy  |  ReadOnly Mode  │  │   │
│  │  │  ┌─ Application ────────────────────┐   │  │   │
│  │  │  │  AES-256-GCM  |  Key Rotation    │   │  │   │
│  │  │  │  ┌─ Monitoring ──────────────┐   │   │  │   │
│  │  │  │  │  Audit | Webhook | SIEM   │   │   │  │   │
│  │  │  │  └───────────────────────────┘   │   │  │   │
│  │  │  └──────────────────────────────────┘   │  │   │
│  │  └─────────────────────────────────────────┘  │   │
│  └───────────────────────────────────────────────┘   │
│                                                      │
│  Auth Failures: 0   Rate Limits: 3   Uptime: 99.9%  │
└──────────────────────────────────────────────────────┘
```

- 同心矩形用 CSS border + 漸層色，由外到內亮度遞增
- 即時計數器從 E191 API 拉數據
- Hover 每層顯示詳細說明

### Section 7: Comparison + CTA

- 開源競品對比表（從 README 搬過來，互動版）
- 「5 分鐘跑起來」的 terminal 動畫（打字機效果顯示 curl 指令和回應）
- GitHub star button + Live Demo button

### C 後端 API（dashai-api 新增）

B 的三個 endpoint 加上：

| Endpoint | 回傳 |
|----------|------|
| GET /equipment/substrates | 晶圓位置 + 移動中狀態 |
| GET /equipment/protocols | 各協定連線狀態 + throughput |
| GET /equipment/security/status | E191 安全狀態（已有） |

---

## 實作順序

```
Step 1: dashai-api 後端 (0.5 天)
  - 新增 carriers, oee, jobs, substrates, protocols API
  - 模擬器自動推進 state

Step 2: B - 擴充 TV Equipment (0.5 天)
  - FOUP 面板 + OEE gauge + Job timeline
  - 串接 Step 1 的 API

Step 3: C - Showcase 頁面 (1.5 天)
  - 7 個 section 逐一實作
  - SVG 動畫 + CSS transitions
  - 響應式（desktop > tablet > 不做 mobile，showcase 頁面適合大螢幕）

Step 4: 路由 + 導航 + SEO (0.5 天)
  - smart-factory-demo router 加 /showcase
  - go-factory-io README 連結更新
  - OG image + meta description
```

## 設計規範

- 暗色主題：bg `#0a0e1a`, card `#111827`, accent `#3b82f6` (blue-500)
- 字型：系統 monospace for 數據，sans-serif for 標題
- 動畫：所有 transition 300ms ease, gauge 用 1s ease-out
- 間距：4px 基數 (8/16/24/32/48)
- 資料更新：3 秒 poll，數值用 CSS transition 平滑過渡

## Checklist

### B - TV Equipment 擴充
- [ ] FOUP 載體狀態面板（2 ports + slot map）
- [ ] OEE 圓形 gauge（A/P/Q/OEE）
- [ ] Process Job 時間軸
- [ ] dashai-api: /equipment/carriers API
- [ ] dashai-api: /equipment/oee API
- [ ] dashai-api: /equipment/jobs API

### C - Showcase 頁面
- [ ] Section 1: Hero + 設備剖面動畫
- [ ] Section 2: GEM State Machine (live)
- [ ] Section 3: FOUP Carrier Animation
- [ ] Section 4: OEE Dashboard
- [ ] Section 5: Multi-Protocol Data Flow
- [ ] Section 6: Security Layers
- [ ] Section 7: Comparison + CTA
- [ ] dashai-api: /equipment/substrates API
- [ ] dashai-api: /equipment/protocols API
- [ ] Router + SEO meta
- [ ] 三語言 i18n (zh-TW / en / ja)
