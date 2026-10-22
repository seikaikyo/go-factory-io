# 升級 golang.org/x/net 與工具鏈修補已知漏洞

- **type**: fix
- **建立日期**: 2026-10-22

## 背景

2026-10-22 每日資安巡檢的 osv-scanner 對本 repo 回報 10 筆已知漏洞，來自 `go.mod` 的 `golang.org/x/net`（6 筆，需 v0.60.0）、`golang.org/x/text`（2 筆）與 `google.golang.org/grpc`（2 筆）。拉下 GitHub 上的 main 後，x/text v0.41.0 與 grpc v1.83.2 已在先前的提交修好，剩 x/net v0.58.0。

同日 Go 發布 go1.26.9（10/8，crypto/tls、net/http 等安全修補）與 go1.26.10（10/19，crypto/x509 封鎖遭入侵的 eMudhra 中繼憑證機構，CVE-2026-111943）。`go.mod` 的 `toolchain` 還寫 go1.26.7。

## 變更內容

1. `go get golang.org/x/net@v0.60.0` 加 `go mod tidy`。x/net v0.60.0 要求 x/text v0.42.0，連帶升級 x/text、x/sys v0.48.0、x/sync v0.23.0，都在同一主版本內。
2. `go mod edit -toolchain=go1.26.10`。

## 影響範圍

| 對象 | 影響 |
| --- | --- |
| 執行期行為 | 無功能變更。x/net 只升修補版。 |
| go.mod / go.sum | 兩檔更新。 |
| 下游 | dashai-go 以 module 方式引用本 repo，Docker 建構時解析，版本由 dashai-go 的 go.mod 決定，不受本次推送直接影響。 |
| 部署 | 依 git push 自動部署。 |

## UI 規格

無 UI 變更。

## 測試計畫

1. `go build ./...`、`go vet ./...`、`go test -count=1 ./...` 全部通過（2026-10-22 本機已跑過）。
2. 建構通過才推送。
3. 推送後重跑每日巡檢，確認 osv-details/go-factory-io.json 的漏洞數歸零或只剩沒有修復版本者。
