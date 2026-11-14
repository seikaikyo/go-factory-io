# 對外輸入的防護加固（2026-11-14 安全檢查）

- **type**: fix
- **status**: in progress
- **建立日期**: 2026-11-14

## 背景

2026-11-14 的安全檢查列出幾項由對外輸入觸發的問題：HSMS 對端送來的訊息、設備回覆、本機 REST API 與 Studio WebSocket。共通點是單一連線或單一訊息就能讓整個行程結束，或讓記憶體無上限成長。本提案只修這些點，不改公開 API 的函式簽章與既有預設值。

## 變更內容

### 1. GEM 處理器的空 ID 與非清單本體

`pkg/driver/gem/handler.go`

- 原本：S1F3、S2F13、S2F15、S2F33、S2F35、S2F37、S5F3 對 ID 項目直接取第一個值，ID 項目是空的就 panic；S2F15、S2F33、S2F35、S2F37、S2F41、S5F3 在本體或子項不是清單時，對 `ItemAt` 回傳的 nil 呼叫方法而 panic。
- 現在：空的 SVID、ECID 依 SEMI E5 回零長度項目（與查不到的 ID 相同）；其他訊息跳過空的 ID 或回該訊息既有的錯誤確認碼；S1F3、S2F13 本體不是清單時回錯誤。

### 2. SECS-II 解碼器的巢狀深度與清單項數

`pkg/message/secs2/decode.go`

- 原本：清單巢狀沒有深度上限，遞迴深度只受訊息大小限制，深到一定程度會造成 stack overflow（無法 recover 的致命錯誤）；清單依宣告項數先配置記憶體，項數最大約 1,677 萬。
- 現在：巢狀深度上限 64 層（套件內常數 `maxDecodeDepth`），超過回錯誤；清單宣告的項數超過剩餘資料可容納的數量（每個子項至少 2 位元組）時，先回錯誤再配置。

### 3. 訊息處理的 panic 不再結束行程

`pkg/driver/gem/handler.go`、`pkg/transport/hsms/session.go`、`examples/simulator/simulator.go`

- 原本：整個 repo 沒有 recover，處理單一訊息時的 panic 會結束整個行程。
- 現在：`Handler.HandleMessage` 攔下 panic，轉成錯誤回傳、記錄日誌與安全事件，並關閉該訊息所屬的 session；HSMS 讀取迴圈與模擬器的訊息迴圈也加 recover，只關閉該連線。

### 4. `secsgem simulate` 的 REST API 與 `/metrics`

`cmd/secsgem/main.go`

- 原本：沒有設寫入 token 也沒有設讀取 token 時，`PUT /api/ec/{ecid}` 與 `POST /api/command` 不需認證即可使用，與啟動訊息「寫入端點關閉」相反；不檢查 Host 標頭；寫入端點接受任何 Content-Type；`/metrics` 不經 token 驗證。
- 現在：沒有寫入 token 時寫入端點一律回 403，與啟動訊息一致；沒有任何 token 時只接受 loopback 的 Host（`localhost`、`127.0.0.1`、`[::1]`），其他回 403；寫入端點只接受 `application/json`；設了 token 時 `/metrics` 需帶讀取或寫入 token。`pkg` 下的 `rest` 函式庫行為不變。

### 5. Studio 綁 loopback 時檢查 Host

`pkg/studio/server.go`

- 原本：`LoopbackOnly` 且沒設 token 時，`/ws` 與 `/api/*` 不檢查 Host 標頭，只要 Origin 與 Host 一致就給特權 socket。
- 現在：`LoopbackOnly` 且沒設 token 時，Host 不是 loopback 名稱或位址就回 403。設了 token、或沒有 `LoopbackOnly` 的情況行為不變。

### 6. Studio trace 與 WebSocket 的資源上限

`pkg/studio/server.go`

- 原本：每則收發都附加到 trace，永不修剪；單則 trace 內容不限長度；WebSocket 單則訊息上限是函式庫預設 32 KB；同一條連線的訊息次數不受限制；`SimulatorDemo` 模式下任何人都能注入 `disconnect` 故障讓內建模擬器永久斷線；`clear_trace` 不需特權。
- 現在：
  - trace 最多保留 500 筆，所有筆的 SML 文字合計最多 4 MB，超過時丟最舊的；單筆 SML 文字超過 64 KB 時截斷並加註。
  - WebSocket 單則訊息上限 16 KB。
  - 每條連線每秒 10 則、突發 20 則，超過即以 policy violation 關閉該連線。
  - `SimulatorDemo` 模式拒絕 `disconnect` 故障，其他故障類型不變。
  - `clear_trace` 改為需要特權 socket（與 `send` 等指令相同）；`SimulatorDemo` 與 loopback 模式下匿名 socket 本來就有特權，不受影響。

### 7. 解碼器 fuzz 測試

`pkg/message/secs2/decode_fuzz_test.go`：新增 `FuzzDecode`，確認任意輸入不 panic，解碼成功的項目重新編碼後可再解碼。

## 影響範圍

| 對象 | 影響 |
| --- | --- |
| 公開 API | 函式簽章與匯出名稱不變。`gem.Handler.HandleMessage` 在 panic 時會關閉 session。 |
| SECS-II 解碼 | 超過 64 層巢狀或宣告項數超過資料量的訊息改回錯誤；實際 SEMI E5 訊息的巢狀深度遠低於此值。 |
| `secsgem simulate` | 未設寫入 token 時寫入端點改為關閉；未設 token 時非 loopback Host 被拒；`/metrics` 在設 token 時需認證。 |
| `secsgem studio` | loopback 模式只接受 loopback Host；trace 有上限；WebSocket 訊息有大小與速率上限。 |
| 以 `SimulatorDemo` 掛載 Studio 的服務 | 場景、快速訊息、SML 送出照常；`disconnect` 故障被拒並回錯誤訊息；trace 只保留最近 500 筆。 |
| 下游 | 使用 `pkg/simulator` Host 的程式（例如邊緣閘道器）收到過深的回覆時改為收到解碼錯誤。 |

## 未在本提案處理

- HSMS 被動端在連線結束後停止監聽：修法會改變 `hsms.Session` 被動模式 `Done()` 與 `Close()` 的既有語意（既有測試與 `pkg/session` 依賴它），另案決定。
- `secsgem studio` 的 `--host` 預設值與「非 loopback 沒有 token 拒絕啟動」：屬於預設值變更，另案決定。
- `rest` 函式庫在沒有 token 時的寫入端點行為：屬於函式庫預設值，本次只在 `secsgem simulate` 執行檔層收緊。

## UI 規格

無 UI 變更。Studio 網頁在被拒的指令上沿用既有的錯誤訊息顯示。

## 測試計畫

1. 每一項先寫會失敗的測試，確認改前失敗、改後通過：
   - `pkg/driver/gem`：空 ID 與非清單本體不 panic；S1F3、S2F13 空 ID 回零長度項目；自訂處理器 panic 轉成錯誤並關閉 session。
   - `pkg/message/secs2`：超過上限的巢狀回錯誤、等於上限可解碼；宣告項數過大回錯誤且不先配置大量記憶體。
   - `pkg/transport/hsms`：讀取迴圈內的 panic 只關閉該 session（子程序驗證行程存活）。
   - `examples/simulator`：處理器 panic 後模擬器行程存活（子程序驗證）。
   - `cmd/secsgem`：寫入端點、Host、Content-Type、`/metrics` 的認證。
   - `pkg/studio`：loopback Host 檢查、trace 上限、WebSocket 大小與速率上限、demo 拒絕 `disconnect`、`clear_trace` 需特權。
2. `FuzzDecode` 本機短跑 60 秒。
3. `go build ./...`、`go vet ./...`、`go test ./...` 不比改動前多失敗。
4. 以本機 127.0.0.1 起 `secsgem simulate` 與 `secsgem studio` 重打原始重現情境，確認行程存活。
