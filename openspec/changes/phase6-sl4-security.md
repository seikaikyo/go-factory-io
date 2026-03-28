---
title: Phase 6 - SL4 Security (AES-GCM, CRL/OCSP, E191, Anomaly, HSM)
type: feature
status: completed
created: 2026-03-28
---

# Phase 6 - SL4 Security

## 目標

IEC 62443 Security Level 4: 防禦有組織且資源豐富的攻擊者。
補齊 Phase 2 P2 遺留的五項進階安全功能。

## 變更內容

### 1. App-layer AES-GCM Encryption Middleware
SECS-II payload 加密，基於 Secured SECS/GEM 論文 (IJACSA 2021)。
Middleware 模式：encrypt before send, decrypt after receive，對 HSMS 層透明。

### 2. CRL/OCSP Certificate Revocation
TLS 連線時檢查對方憑證是否已撤銷。CRL 本地快取 + OCSP stapling。

### 3. SEMI E191 Cybersecurity Status Reporting
設備資安狀態回報端點，回報 TLS 狀態、認證失敗次數、rate limit 觸發次數等。

### 4. Anomaly Detection Interface
定義介面讓外部 ML 系統接入，分析 SECS 訊息模式。不做 ML 本身。

### 5. HSM Key Storage Interface (PKCS#11)
定義介面讓 TLS private key 存在硬體安全模組。不實作硬體驅動。

## 影響範圍

### 新增
- `pkg/security/encryption.go` — AES-GCM middleware
- `pkg/security/encryption_test.go`
- `pkg/security/revocation.go` — CRL/OCSP checker
- `pkg/security/revocation_test.go`
- `pkg/security/e191.go` — SEMI E191 cybersecurity status
- `pkg/security/e191_test.go`
- `pkg/security/anomaly.go` — Anomaly detection interface
- `pkg/security/anomaly_test.go`
- `pkg/security/hsm.go` — HSM/PKCS#11 interface

### 修改
- `api/rest/handler.go` — /api/security/status endpoint (E191)
- `README.md`

## Checklist

### AES-GCM Encryption
- [x] MessageEncryptor (Encrypt/Decrypt SECS-II payload)
- [x] AES-256-GCM with random nonce (12 bytes)
- [x] Key rotation support (RotateKey)
- [x] GenerateKey helper
- [x] Unit tests (9 tests: encrypt/decrypt, empty, tamper, wrong key, short, rotation, invalid size, unique nonces)

### CRL/OCSP
- [x] CRL fetcher + local cache (configurable TTL)
- [x] OCSP checker (soft-fail on no response)
- [x] VerifyCertificate() integrating both
- [x] Unit tests (5 tests: no CRL, no OCSP, verify, cache clear, defaults)

### SEMI E191
- [x] SecurityStatus struct (atomic counters, TLS state, policy flags)
- [x] /api/security/status REST endpoint
- [x] AsEventHandler() auto-updates counters from Auditor events
- [x] Unit tests (4 tests: counters, report, event handler, format)

### Anomaly Detection
- [x] AnomalyDetector interface (Analyze + Train)
- [x] MessagePattern struct (S/F, source, size, timing, inter-arrival)
- [x] PatternCollector (stats, alert callback, inter-arrival tracking)
- [x] NoopDetector + ThresholdDetector reference implementations
- [x] Unit tests (7 tests: nil, noop, threshold, stats, inter-arrival, analyze, interface)

### HSM Interface
- [x] KeyStore interface (Sign, GetCertificate, GetSigner, ListKeys)
- [x] SoftwareKeyStore (in-memory: generate, import, sign, TLS cert)
- [x] NewHSMSigner (crypto.Signer wrapper for KeyStore)
- [x] Unit tests (7 tests: generate, sign+verify, import, TLS, not found, no cert, HSMSigner)
