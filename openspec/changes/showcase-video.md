---
title: go-factory-io 60s Showcase Video (Remotion)
type: feature
status: in-progress
created: 2026-03-28
---

# go-factory-io 60s Showcase Video

## 目標

用 Remotion 製作 60 秒產品介紹影片，面向面試官和技術主管，展現 go-factory-io 的技術含金量。

## 結構（30fps, 1800 frames）

| 場景 | 秒數 | Frames | 內容 |
|------|------|--------|------|
| Hook | 0-8s | 240 | 問題：半導體設備說的是 1980 年代的協定 |
| Architecture | 8-20s | 360 | 設備協定 → Go binary → 現代 API 動畫 |
| Live Data | 20-35s | 450 | OEE gauge + FOUP + 封包流動 |
| Security | 35-48s | 390 | 安全層堆疊 + 競品對比表 |
| CTA | 48-60s | 360 | GitHub star + showcase 連結 |

## 影響範圍

- `video/remotion/src/GoFactoryVideo.tsx` (新)
- `video/remotion/src/scenes/GfHookScene.tsx` (新)
- `video/remotion/src/scenes/GfArchScene.tsx` (新)
- `video/remotion/src/scenes/GfLiveScene.tsx` (新)
- `video/remotion/src/scenes/GfSecurityScene.tsx` (新)
- `video/remotion/src/scenes/GfCtaScene.tsx` (新)
- `video/remotion/src/audioConfig.gofactory.ts` (新)
- `video/remotion/src/Root.tsx` (修改)
- `video/remotion/package.json` (修改)

## Checklist

- [ ] audioConfig timing
- [ ] GfHookScene
- [ ] GfArchScene
- [ ] GfArchScene
- [ ] GfLiveScene
- [ ] GfSecurityScene
- [ ] GfCtaScene
- [ ] GoFactoryVideo composition
- [ ] Root.tsx 註冊
- [ ] Remotion Studio 預覽通過
