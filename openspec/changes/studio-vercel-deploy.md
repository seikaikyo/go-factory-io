---
title: Studio Vercel Deploy + dashai-api Integration
type: feature
status: in-progress
created: 2026-03-29
---

# Studio Static Deploy (studio.dashai.dev)

## Summary

Deploy SECSGEM Studio web UI as static site on Vercel (studio.dashai.dev),
backed by existing dashai-api /factory Python simulator.

## Changes

### go-factory-io (this repo)
- `studio-site/` — standalone static site for Vercel deployment
  - `index.html` — adapted from pkg/studio/web/, REST polling instead of WebSocket
  - `studio.css` — same as embedded version
  - `studio.js` — rewritten for REST polling + CORS to dashai-api
  - `vercel.json` — Vercel config (rewrites, headers)
- `package.json` — minimal, for Vercel static deploy

### dashai-api (separate repo)
- `factory/routes/equipment_live.py` — add 3 studio endpoints:
  - `GET /equipment/studio/trace` — returns recent simulated message trace
  - `POST /equipment/studio/send` — simulate sending a message + validation
  - `GET /equipment/studio/report` — implementation coverage report

## Architecture

```
studio.dashai.dev (Vercel static)
    |
    |-- REST polling every 3s
    |
dashai-api.onrender.com/factory/api/v1/equipment/studio/*
    |
    |-- Python simulation (no real SECS/GEM needed)
```

## DNS Setup

CNAME: `studio` -> `cname.vercel-dns.com` (on dashai.dev)

## Test Plan

1. Local: open studio-site/index.html, verify tabs render
2. Deploy to Vercel, verify studio.dashai.dev loads
3. Verify REST endpoints return valid data
4. Verify CORS headers allow studio.dashai.dev -> dashai-api.onrender.com
