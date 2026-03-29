---
title: SECSGEM Studio 90-120s Showcase Video (Remotion)
type: feature
status: completed
created: 2026-03-28
updated: 2026-03-29
---

# SECSGEM Studio Showcase Video

## 目標

用 Remotion + Google TTS (en-US-Neural2-J) 製作 90-120 秒 SECSGEM Studio 介紹影片，
展示 studio.dashai.dev 的四大功能：Simulator、Validator、SML Editor、Report。

## 規格

- 1920x1080, 30fps, en-US-Neural2-J voice
- 9 scenes，實際截圖 + 動畫轉場
- 合規：AI disclosure, SEMI disclaimer, implements (not compliant)

## 結構（30fps, ~3450 frames）

| # | Scene | 秒數 | Frames | 內容 |
|---|-------|------|--------|------|
| 1 | Logo + Intro | 8s | 240 | go-factory-io logo + SECSGEM Studio tagline |
| 2 | Dashboard | 12s | 360 | Live equipment feed, real-time message stream |
| 3 | Simulator | 15s | 450 | Quick messages, E5/E37 protocol simulation |
| 4 | SML Editor | 15s | 450 | Custom SML message authoring and send |
| 5 | Validator | 15s | 450 | State machine validation, pass/fail results |
| 6 | Report | 20s | 600 | Coverage metrics, standard compliance summary |
| 7 | Architecture | 15s | 450 | System diagram: Go binary + WebSocket + REST |
| 8 | CTA | 10s | 300 | studio.dashai.dev + GitHub link |
| 9 | Disclaimer | 5s | 150 | AI disclosure + SEMI standards disclaimer |

## 影響範圍

位置：`/Users/dash/github/smart-factory-demo/video/remotion/`

### 新增檔案
- `video/narration.studio.en.tsv` — 旁白稿
- `remotion/generate-audio-studio.py` — TTS 生成腳本
- `remotion/src/audioConfig.studio.ts` — 自動生成音頻配置
- `remotion/src/StudioVideo.tsx` — 主影片元件
- `remotion/src/scenes/StIntroScene.tsx` — Scene 1
- `remotion/src/scenes/StDashboardScene.tsx` — Scene 2
- `remotion/src/scenes/StSimulatorScene.tsx` — Scene 3
- `remotion/src/scenes/StEditorScene.tsx` — Scene 4
- `remotion/src/scenes/StValidatorScene.tsx` — Scene 5
- `remotion/src/scenes/StReportScene.tsx` — Scene 6
- `remotion/src/scenes/StArchScene.tsx` — Scene 7
- `remotion/src/scenes/StCtaScene.tsx` — Scene 8
- `remotion/src/scenes/StDisclaimerScene.tsx` — Scene 9
- `remotion/public/shots/studio-*.png` — 截圖素材

### 修改檔案
- `remotion/src/Root.tsx` — 註冊 Studio-Showcase composition
- `remotion/package.json` — 新增 render:studio script

## 截圖清單

| 檔案名 | 來源 | 內容 |
|--------|------|------|
| studio-dashboard.png | studio.dashai.dev Dashboard tab | Message log + connection status |
| studio-simulator.png | studio.dashai.dev Simulator tab | Quick message buttons + response |
| studio-editor.png | studio.dashai.dev SML Editor tab | SML textarea + send button |
| studio-validator.png | studio.dashai.dev Validator tab | Validation results table |

## 合規 Checklist

- [ ] 旁白使用 "implements" 不用 "compliant" 或 "certified"
- [ ] 效能數據標明 "simulator environment"
- [ ] Scene 9 包含 AI disclosure
- [ ] Scene 9 包含 SEMI standards disclaimer
- [ ] 不宣稱 "production ready"

## 測試計畫

1. Remotion Studio 預覽所有 9 個 scene
2. 音頻同步無明顯延遲
3. 字幕與旁白內容匹配
4. 截圖清晰可辨
5. 合規文字完整顯示
