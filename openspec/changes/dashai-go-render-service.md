---
title: Deploy dashai-go on Render Free (Go API Gateway)
type: feature
status: blocked
created: 2026-03-29
blocked_until: 2026-04-01
---

# dashai-go Render Free Service

## Status

BLOCKED: Render Free instance quota exhausted for 2026-03. Resume after 2026-04-01.

## What

Deploy go-factory-io as `dashai-go` on Render Free tier (Singapore region).
Replace the Python shim in dashai-api with the real Go SECSGEM Studio backend.

## Why

- Go version has real SECS/GEM validator (schema + state + timing), Python shim is simulated
- WebSocket real-time vs 3-second REST polling
- Single 19MB binary, ~15MB RAM vs Python ~150MB
- No dual maintenance (Go logic + Python copy)

## Ready to deploy

Everything is already done:
- Dockerfile at repo root (port 10000)
- `/health` endpoint on studio server
- `studio` subcommand in CLI
- All tests pass
- studio-site JS has Go version ready (change `API_BASE` back)

## Steps (2026-04-01)

1. Render Dashboard -> New Web Service
   - Name: `dashai-go`
   - Repo: `seikaikyo/go-factory-io`
   - Branch: `main`
   - Language: Docker
   - Region: Singapore
   - Plan: Free
2. Wait for Docker build (~2 min)
3. Verify: `curl https://dashai-go.onrender.com/health`
4. Update studio-site/studio.js: change `API_BASE` to dashai-go, switch to WebSocket
5. `cd studio-site && npx vercel --prod`
6. Verify studio.dashai.dev connects via WebSocket
7. (Optional) Remove Python shim endpoints from dashai-api

## Checklist

- [ ] Render Free quota available (2026-04-01)
- [ ] Create dashai-go service on Render
- [ ] Verify /health returns OK
- [ ] Switch studio-site JS to WebSocket + dashai-go
- [ ] Redeploy Vercel
- [ ] Verify studio.dashai.dev end-to-end
