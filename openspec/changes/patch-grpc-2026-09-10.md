# 升級 grpc 修補三筆警示

- **type**: fix
- **建立日期**: 2026-09-10

## 背景

2026-09-10 起 seikaikyo/security-watch 的採集器（`.github/workflows/github-alerts.yml`）每日抓取 19 個 repo 的 Dependabot 警示。這是該機制上線後第一次量到本 repo 的未修數字，在此之前每日資安巡檢的 GitHub 段都記為「無法檢查」，等於這些警示長期無人監看。

本 repo 有 3 筆未修警示，high 2 筆、medium 1 筆，全部來自 `go.mod` 的 `google.golang.org/grpc`。目前版本 v1.82.1。

## 變更內容

三筆警示的最高修復版本是 v1.83.1（GHSA-2v4p-qf9q-27wj 需 1.82.2、GHSA-qc2q-p7wx-3px3 需 1.83.1）。做法是 `go get google.golang.org/grpc@v1.83.1` 加 `go mod tidy`。同一主版本內升級。

## 影響範圍

| 對象 | 影響 |
| --- | --- |
| 執行期行為 | 無。grpc 進入執行期，是本次三個 repo 裡唯一非建構期的相依。 |
| package.json 或 go.mod 的直接相依宣告 | `go.mod` 與 `go.sum` 會更新。 |
| 部署 | Render 依 git push 自動部署。 |

## UI 規格

無 UI 變更。

## 測試計畫

1. `make build` 與 `make test` 通過。
2. 建構通過才推送。
3. 推送後重新觸發 security-watch 的 github alerts workflow，確認本 repo 的 `open_total` 歸零或僅剩無修復版本者。
