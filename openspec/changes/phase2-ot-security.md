---
title: Phase 2 - OT Security (IEC 62443 / SEMI E187)
type: feature
status: completed
created: 2026-03-27
---

# Phase 2 - OT Security Hardening

## 目標

依據 IEC 62443、SEMI E187、NIST SP 800-82 工業安全規範，為 go-factory-io 加入安全防護。
目標達到 IEC 62443 Security Level 2 (SL2)：防禦有意圖但資源有限的攻擊者。

## 規範依據

| 規範 | 用途 | 來源 |
|------|------|------|
| IEC 62443-3-3 | 系統安全需求 (67 SR) | ot-security-mcp skill |
| IEC 62443-4-2 | 元件安全需求 (51 CR) | ot-security-mcp skill |
| SEMI E187 | 晶圓廠設備資安 (12 requirements) | 2022 SEMI standard |
| SEMI S2 | 設備安全 (alarm severity mapping) | SEMI safety standard |
| NIST SP 800-82 Rev 3 | OT 安全指南 | NIST 2023 |
| Secured SECS/GEM | AES-GCM 加密機制 | IJACSA 2021 論文 |

## P0 - 必做 (SL2 基線)

### 1. TLS 傳輸加密

**規範**: IEC 62443 SR 4.1 (Data Confidentiality), SEMI E187 Network Security, NIST SC

```go
// hsms.Config 新增欄位
type Config struct {
    // ...existing fields...

    // TLS: nil = plaintext (development only), non-nil = encrypted
    TLSConfig *tls.Config

    // RequireTLS: if true, refuse plaintext connections
    RequireTLS bool
}
```

**實作方式**: TLS Wrapper (Option A)
- 用 Go 標準庫 `crypto/tls` 包裝 TCP 連線
- 對 HSMS 協議層透明，不改訊息格式
- 支援 mTLS (mutual TLS) 雙向驗證
- TLS 1.2+ only, 禁用 SSLv3/TLS 1.0/1.1
- 預設 cipher suite: TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256

**為什麼不用 Application-layer 加密**:
- TLS wrapper 用 Go battle-tested 的 crypto 庫
- 不需要改協議格式，向下相容
- 大部分晶圓廠也是在網路層加密（VPN/TLS termination）
- Application-layer (AES-GCM on payload) 留 P2 做 optional middleware

### 2. Peer 驗證與 IP 白名單

**規範**: IEC 62443 FR1, NIST IA, SEMI E187

```go
type Config struct {
    // ...existing fields...

    // AllowedPeers: IP allowlist, nil = accept all
    AllowedPeers []net.IP

    // MaxConnections: max concurrent connections (passive mode)
    MaxConnections int
}
```

- mTLS 證書驗證（SL2: 裝置對裝置身份認證）
- IP 白名單（在 Accept 階段擋掉未授權連線）
- 最大連線數限制（passive mode）

### 3. Security Event Logging

**規範**: IEC 62443 FR6, NIST AU, SEMI E187 Security Monitoring

```go
// SecurityEvent represents a security-relevant event for audit logging.
type SecurityEvent struct {
    Time      time.Time
    Level     SecurityLevel  // Info, Warning, Critical
    Category  string         // auth, access, integrity, availability
    Event     string         // connection_rejected, auth_failed, rate_limited, malformed_message
    Source    string         // Remote IP/session ID
    Details   map[string]interface{}
}

// SecurityEventHandler processes security events.
type SecurityEventHandler func(event SecurityEvent)
```

必須記錄的事件：
- 連線被拒（IP 不在白名單、超過連線數上限）
- 認證失敗（TLS handshake 失敗、證書驗證失敗）
- 訊息格式錯誤（HSMS/SECS-II decode 失敗）
- 速率超限
- Select 被拒
- 未授權的 S/F 訊息

### 4. Rate Limiting

**規範**: IEC 62443 FR7 (Resource Availability), NIST SP 800-82

```go
type Config struct {
    // ...existing fields...

    // MaxMessageRate: max messages per second per connection, 0 = unlimited
    MaxMessageRate int

    // MaxMessageSize: max HSMS message size in bytes, 0 = default (16MB)
    MaxMessageSize int
}
```

- Per-connection token bucket rate limiter
- 超限時回 Reject.req 而非直接斷線
- Max message size 防止 OOM

### 5. Secure Defaults

**規範**: SEMI E187 OS Security (Hardening Baselines)

- `RequireTLS` 預設 `false` (向下相容)，但 production config helper 預設 `true`
- `MaxConnections` 預設 10
- `MaxMessageRate` 預設 1000 msg/sec
- `MaxMessageSize` 預設 16MB
- `T7` (Not Selected Timeout) 已有合理預設 10s
- 新增 `SessionTTL` 預設 24h，超時自動斷線

### 6. REST API Authentication

**規範**: IEC 62443 SR 1.1, NIST AC

```go
type APIConfig struct {
    // AuthMode: none, bearer-token, mutual-tls
    AuthMode string

    // BearerToken: shared secret for bearer token auth
    BearerToken string

    // TLSConfig: for mutual TLS on the API server
    TLSConfig *tls.Config
}
```

- Bearer token 驗證（從 env var 讀取，不 hardcode）
- CORS 限制 origin（production 不用 `*`）
- Rate limit on API endpoints

## P1 - 應做 (SL3 進階)

### 7. SECS Stream/Function RBAC

```go
type SessionPolicy struct {
    // AllowedMessages: S/F pairs this session can send, nil = allow all
    AllowedMessages []SFPair

    // ReadOnly: if true, only allow read operations (S1Fx, S2F13, etc.)
    ReadOnly bool
}
```

- Per-session 可發送的 S/F 對清單
- 唯讀模式（只允許讀取 SV/EC，不能 S2F15 寫入或 S2F41 遠端指令）
- 控制設備端不接受未授權的 RCMD

### 8. Safety Interlock (SEMI S2)

```go
type SafetyConfig struct {
    // CriticalAlarmAction: what to do when severity >= EquipSafety
    CriticalAlarmAction AlarmAction // LogOnly, ForceOffline, ForceIdle

    // OnSafetyAlarm: callback for safety-critical alarms
    OnSafetyAlarm func(alarm *Alarm)
}
```

- Alarm severity >= `AlarmEquipSafety` 時自動執行安全動作
- 可設定為 LogOnly / ForceOffline / ForceIdle
- SEMI S2 要求: 受影響的設備活動必須被禁止

### 9. Message Recording (Forensic)

- Interceptor middleware: 所有 SECS 訊息寫入檔案/buffer
- 格式: timestamp + direction + S/F + raw hex
- 可搭配外部 SIEM 系統

### 10. Security Event Webhook

- 安全事件推送到外部系統（Syslog/Webhook/MQTT）
- 對應 SEMI E187 "Report events to fab security systems"

## P2 - 未來 (SL4)

| 項目 | 說明 |
|------|------|
| App-layer AES-GCM | Secured SECS/GEM 論文的加密方式，middleware 實作 |
| Anomaly Detection | 訊息模式分析介面，接外部 ML 系統 |
| HSM Key Storage | PKCS#11 介面，TLS key 存在硬體安全模組 |
| Certificate Revocation | CRL/OCSP 檢查 |
| SEMI E191 | Cybersecurity Status Reporting 端點 |

## 影響範圍

### 修改
- `pkg/transport/hsms/config.go` — 新增 TLS/security 欄位
- `pkg/transport/hsms/session.go` — TLS wrapper, IP 白名單, rate limit, security logging
- `pkg/driver/gem/handler.go` — RBAC 檢查, safety interlock
- `pkg/driver/gem/alarm.go` — Safety action 整合
- `api/rest/handler.go` — Bearer token auth, CORS 限制
- `cmd/secsgem/main.go` — Security config flags

### 新增
- `pkg/security/` — SecurityEvent, SecurityEventHandler, rate limiter
- `pkg/security/tls.go` — TLS helper (cert loading, default config)
- `pkg/security/audit.go` — Audit logger
- `pkg/security/ratelimit.go` — Token bucket rate limiter

## 測試計畫

| 項目 | 方式 |
|------|------|
| TLS 連線 | Active/Passive 雙方 TLS + mTLS 測試 |
| IP 白名單 | 允許/拒絕連線測試 |
| Rate limit | 超限時正確拒絕 + 不超限時正常通過 |
| Security logging | 各安全事件都有對應 log 輸出 |
| REST API auth | 有/無 token 的 401/200 測試 |
| RBAC | 允許/拒絕特定 S/F 測試 |
| Safety interlock | 高嚴重度 alarm 觸發正確動作 |
| 向下相容 | 不設 TLS 時仍正常運作（plaintext） |

## Checklist

### P0 - TLS + 基礎安全
- [x] TLS wrapper for HSMS (Active + Passive)
- [x] mTLS (mutual TLS) 雙向驗證
- [x] TLS helper: cert/key 載入, 預設安全 cipher suite, 動態測試 CA
- [x] IP 白名單 (connection accept 階段)
- [x] Max connections 限制 (config field)
- [x] SecurityEvent 結構 + Auditor (IEC 62443 FR1-FR7 categories)
- [x] 安全事件 logging (auth fail, rejected, malformed, rate limited, unauthorized)
- [x] Rate limiter (token bucket, per-connection)
- [x] Max message size 限制
- [x] Session TTL (auto-disconnect)
- [x] REST API bearer token auth (/health 公開)
- [x] SecureConfig() helper 一鍵 SL2 預設
- [x] 向下相容測試 (plaintext mode 仍正常)

### P1 - RBAC + Safety
- [x] SECS S/F RBAC (per-session policy, allowlist + denylist)
- [x] ReadOnly mode (封鎖所有寫入/控制 S/F)
- [x] MonitorPolicy (僅允許讀取操作)
- [x] Safety interlock (alarm severity -> ForceOffline/ForceIdle/LogOnly)
- [x] OnSafetyAlarm callback
- [x] Message recording middleware (forensic, hex dump, file writer)
- [x] Security event webhook (Phase 4 完成: HTTP POST + Syslog)
