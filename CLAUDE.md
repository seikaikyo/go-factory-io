# go-factory-io

Go 寫的 SECS/GEM 設備驅動程式庫與單一執行檔，含 HSMS、Modbus TCP、OPC-UA、MQTT 傳輸與 IEC 62443 對應的安全控制。功能與使用方式見 `README.md`。

## 目錄

| 路徑 | 內容 |
|------|------|
| `cmd/secsgem/` | 執行檔進入點 |
| `pkg/` | 對外的 Go 套件：`driver`（GEM 狀態機）、`message`（SECS-II 編解碼）、`transport`（HSMS、Modbus、OPC-UA）、`session`、`security`、`metrics`、`validator`、`simulator`、`bridge`、`studio` |
| `internal/logger/` | 內部日誌 |
| `api/` | `grpc` 與 `rest` 介面定義 |
| `clients/python/` | Python 用戶端 |
| `examples/` | 範例程式 |
| `test/integration/` | 整合測試 |
| `studio-site/` | SECSGEM Studio 靜態網站（studio.seikai.dev），部署在 Vercel |
| `openspec/changes/` | 變更提案，改動 20 行以上先在這裡建提案 |

## 使用端與部署

- module 路徑是 `github.com/dashfactory/go-factory-io`，repo 實際在 `github.com/seikaikyo/go-factory-io`。使用端（dashai-go、go-edge-gateway）以 git insteadOf 或 `replace` 指到這個 repo
- `studio-site/` 推到 main 後由 Vercel 自動部署
- `render.yaml` 與 `Dockerfile` 留著但 Render 上目前沒有這個服務

## 開發

- Go 版本照 `go.mod`（go 1.26.1，toolchain go1.26.10）
- 驗證：`make build`、`make test`，或 `go build ./...`、`go vet ./...`、`go test ./...`
- 這是公開 repo，不放本機路徑、帳號、金鑰
