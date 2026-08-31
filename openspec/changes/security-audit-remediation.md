# Security Audit Remediation

- **type**: fix
- **branch**: `fix/security-audit-remediation`
- **status**: in progress

## Background

An audit of the repository found that the shipped default entry point
(`Dockerfile` CMD `studio --port 10000`) exposes an unauthenticated
WebSocket control channel, that the REST/gRPC bearer-token code path is
dead in the built binary, that HSMS accepts data messages before Select,
that the RBAC policy engine has no production call site, that CORS is
wide open, and that device-supplied SECS-II ASCII reaches `innerHTML`
unescaped. The README also claims a security posture the code does not
back.

## Changes

### 1. Studio WebSocket origin + command authorization (Critical)

`pkg/studio/server.go`

- Drop `websocket.AcceptOptions.InsecureSkipVerify`. Same-origin is the
  default; extra origins come from `Config.AllowedOrigins` and are passed
  as `OriginPatterns`.
- Add `Config.Token`. `/ws` and `/api/*` require it when set.
- Fail-closed default: with no token configured, the WebSocket serves
  read-only commands (`get_state`, `get_report`, `clear_trace`) and
  rejects the mutating ones (`send`, `quick_send`, `fault`, `run_script`).

### 2. Wire the REST/gRPC bearer token (Critical)

`cmd/secsgem/main.go`, `api/rest/handler.go`, new `pkg/security/apitoken.go`

- Read tokens from flags with environment fallback:
  `--api-token` / `SECSGEM_API_TOKEN` (read scope),
  `--api-write-token` / `SECSGEM_API_WRITE_TOKEN` (write scope),
  `--studio-token` / `SECSGEM_STUDIO_TOKEN`.
- Binding a non-loopback address without a token is a startup error
  (fail-closed).
- Compare with `crypto/subtle.ConstantTimeCompare`.
- Split authorization: `PUT /api/ec/{ecid}` and `POST /api/command`
  require the write token; every other `/api/*` route accepts either.
  A read token alone leaves the write routes denied.
- Pass the same read token to `grpcapi.NewServer` instead of `""`.

### 3. HSMS Selected precondition + T7 (High)

`pkg/transport/hsms/session.go`

- `handleMessage` rejects `STypeDataMessage` unless the session state is
  `Selected`; replies `Reject.req` and audits the attempt.
- Start a T7 not-selected timer once TCP is established; drop the
  connection if Select does not arrive in time. `Config.T7 <= 0` disables it.

### 4. RBAC default-deny (High)

`pkg/driver/gem/handler.go`

- `NewHandler` installs `security.MonitorPolicy()` (read-only allowlist).
- Write access requires an explicit `SetPolicy(security.FullAccessPolicy())`.
- The bundled simulator gains a `--policy monitor|full` flag, default
  `monitor`; the studio's embedded sandbox equipment opts into full access
  explicitly.

### 5. CORS allowlist (High)

`api/rest/handler.go`, `pkg/studio/server.go`

- Replace `Access-Control-Allow-Origin: *` with a configured allowlist,
  default empty (same-origin only). Applies to the SSE handler too.

### 6. DOM XSS (High)

`pkg/studio/web/studio.js`, `studio-site/studio.js`, `pkg/message/secs2/item.go`

- Escape every interpolated value before it reaches `innerHTML`.
- `Item.String()` strips angle brackets and C0/C1 control characters from
  ASCII payloads so the SML rendering cannot smuggle markup.

### 7. README accuracy

`README.md`

- Remove the unqualified "IEC 62443 SL4" claim and the E187
  "Implemented" label; state which controls are on by default, which are
  opt-in, and which are interfaces only.

## Scope

| Area | Files |
|------|-------|
| Studio server | `pkg/studio/server.go`, `pkg/studio/server_test.go` |
| REST API | `api/rest/handler.go`, `api/rest/handler_test.go` |
| Token helper | `pkg/security/apitoken.go`, `pkg/security/apitoken_test.go` |
| HSMS | `pkg/transport/hsms/session.go`, `pkg/transport/hsms/hsms_test.go` |
| GEM RBAC | `pkg/driver/gem/handler.go`, `pkg/driver/gem/policy_default_test.go` |
| CLI wiring | `cmd/secsgem/main.go`, `examples/simulator/simulator.go` |
| Front end | `pkg/studio/web/studio.js`, `studio-site/studio.js` |
| SECS-II | `pkg/message/secs2/item.go` |
| Docs | `README.md` |

Out of scope: `go-common` (no cross-module release), TLS certificate
provisioning, and the Python backend behind `studio-site`.

## Test plan

| Control | Test |
|---------|------|
| Missing/wrong bearer token is rejected | `api/rest`: table test over every route x {no token, wrong token, read token, write token} |
| Write routes deny a read-only token | same table |
| Constant-time compare and env fallback | `pkg/security`: `TestCompareToken`, `TestResolveAPIToken` |
| Non-loopback bind without a token fails | `pkg/security`: `TestResolveAPIToken` fail-closed cases |
| CORS echoes only allowlisted origins | `api/rest`: origin table test |
| Studio WS rejects a foreign Origin | `pkg/studio`: `TestWSOriginRejected` |
| Studio WS refuses mutating commands without a token | `pkg/studio`: `TestWSCommandAuthorization` |
| HSMS drops data messages before Select | `pkg/transport/hsms`: `TestDataMessageBeforeSelect` |
| T7 closes a never-selected connection | `pkg/transport/hsms`: `TestT7NotSelectedTimeout` |
| GEM handler defaults to read-only | `pkg/driver/gem`: `TestDefaultPolicyDeniesWrites` |
| ASCII rendering strips markup | `pkg/message/secs2`: `TestASCIIStringSanitized` |

Acceptance: `go build ./...` clean, `go vet ./...` clean, and
`go test ./...` green across every package with no pre-existing test
removed or weakened.
