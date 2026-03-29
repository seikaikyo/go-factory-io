---
title: SECSGEM Studio AI Chat - Natural Language Equipment Interaction
type: feature
status: draft
created: 2026-03-29
---

# Studio AI Chat + MCP Server

Natural language interface for SECS/GEM equipment communication.
Engineers talk in human language, AI translates to S/F messages and back.

## Problem

SECS/GEM is machine protocol. Engineers need to memorize:
- S1F3 = query SV, S2F41 = remote command, S5F1 = alarm report
- HCACK=0 means OK, COMMACK=1 means rejected
- State machine transitions (DISABLED -> WAIT_CRA -> COMMUNICATING)

Nobody should have to remember this.

## Solution

Two deliverables:

### A. Studio Chat Tab (Web UI, BYOK)

New tab in studio.dashai.dev where users chat with equipment in natural language.

```
User: "Check the current temperature"
AI:   Sending S1F3 (SVID=1002)...
      Temperature is 350.2 C (normal range: 340-360 C)

User: "Why did that alarm trigger?"
AI:   Alarm ALID=1 (OverTemp) was set at 17:23:01.
      Temperature SV was 362.1 C, exceeding the 360 C threshold
      set in EC ProcessTemperature.

User: "Send a START command"
AI:   Sending S2F41 RCMD="START"...
      HCACK=0 (command accepted). Equipment is now processing.

User: "Run a full communication test"
AI:   Running E30 Communication Setup scenario...
      Step 1: S1F13 Establish Comm -> COMMACK=0 (OK)
      Step 2: S1F1 Are You There -> MDLN="STUDIO-EQUIP"
      Step 3: S1F17 Request Online -> ONLACK=0 (OK)
      All 3 steps passed.

User: "What does S6F11 mean?"
AI:   S6F11 is Event Report Send (E30).
      Equipment sends this to report a collection event.
      Structure: L:3 { U4 DATAID, U4 CEID, L:n { reports } }
```

### B. MCP Server (for Claude Code / Claude Desktop)

Python FastMCP server wrapping go-factory-io REST API. Engineers use
Claude Code to interact with equipment via MCP tools.

```
Claude Code:
> "Help me check the equipment status"
> Uses tool: get_equipment_status
> "Equipment is online (COMMUNICATING / ONLINE-REMOTE).
>  Temperature: 350.2C, Pressure: 759.3 Torr, Uptime: 4832s"
```

## Architecture

```
Studio Web UI (studio.dashai.dev)
├── Dashboard / Simulator / Validator / Report
├── Settings (API key config)       ← new
└── Chat Tab                        ← new
    │
    ├── User message
    ├── → AI API (user's own key, via proxy)
    ├── ← AI response with tool calls
    ├── → Execute SECS/GEM action (studio API)
    └── ← Format result in human language

MCP Server (separate, for Claude Code)
├── 10 MCP Tools
├── Wraps studio REST API
└── Claude handles the NLP part
```

## Settings UI (BYOK)

New Settings panel in Studio web UI:

```
Settings
─────────────────────────────
AI Provider:  [Anthropic ▼]  (Anthropic / OpenAI / Ollama)
API Key:      [sk-ant-***]
Model:        [claude-sonnet-4-6 ▼]
Endpoint:     [https://api.anthropic.com]  (custom for Ollama)

[Test Connection]  [Save]

API key is stored in your browser only.
It is never sent to our servers.
─────────────────────────────
```

Storage: browser `localStorage`
Providers supported:
- Anthropic (Claude) — default
- OpenAI (GPT) — alternative
- Ollama (local) — for air-gapped fabs (no internet)

### API Key Security

- Key stored in `localStorage`, never sent to our backend
- Chat requests go through a stateless proxy on dashai-api
  that forwards to AI provider with user's key from request header
- Proxy does NOT log or store the key
- Ollama mode: direct browser → localhost, no proxy needed

## Chat Tab Implementation

### System Prompt

```
You are a SECS/GEM equipment communication assistant for SECSGEM Studio.

Available tools:
- get_status: Get equipment communication and control state
- get_sv(svid): Read a status variable
- list_svs: List all status variables
- get_alarms: List current alarms
- send_command(name, params): Execute a remote command (S2F41)
- send_message(stream, function, body): Send any SECS-II message
- run_scenario(name): Run a built-in test scenario
- get_report: Get implementation coverage report
- explain_sf(stream, function): Explain what a S/F message does

When the user asks about equipment:
1. Use the appropriate tool to get data
2. Explain the result in plain language
3. Include the S/F message used (for learning)
4. Flag any anomalies (alarms, out-of-range values)

SEMI standards are referenced for interoperability.
This software implements the standards but is not certified by SEMI.
```

### Tool Definitions (shared by Chat + MCP)

| Tool | Input | Maps to | Output |
|------|-------|---------|--------|
| get_status | - | GET /equipment/status | comm/control state in plain text |
| get_sv | svid: int | GET /equipment/sv/{svid} | value + units + normal range |
| list_svs | - | GET /equipment/sv | all SVs formatted |
| get_alarms | - | GET /equipment/alarms | active alarms with severity |
| send_command | name, params | POST /equipment/command | result + HCACK explanation |
| send_message | stream, function, body | POST /equipment/studio/send | trace entry + validation |
| run_scenario | name: string | POST /equipment/studio/send (sequence) | step-by-step results |
| get_report | - | GET /equipment/studio/report | coverage summary |
| explain_sf | stream, function | (local lookup) | name, direction, structure, standard |
| get_trace | limit: int | GET /equipment/studio/trace | recent messages |

### Frontend (studio.js additions)

```javascript
// Chat tab: send user message → AI API → execute tools → display result
async function sendChat(userMessage) {
  const apiKey = localStorage.getItem('studio_api_key');
  const provider = localStorage.getItem('studio_provider') || 'anthropic';

  // Build messages with system prompt + tool definitions
  // Call AI API via proxy (or direct for Ollama)
  // Parse tool_use blocks → execute against studio API
  // Display results in chat UI
}
```

### Proxy Endpoint (dashai-api)

```python
@router.post('/equipment/studio/ai-proxy')
async def ai_proxy(request: Request):
    """Stateless proxy for AI API calls. Forwards user's API key."""
    body = await request.json()
    api_key = request.headers.get('X-API-Key')
    provider = body.get('provider', 'anthropic')

    # Forward to AI provider with user's key
    # Return response as-is
    # NO logging of API key
```

## MCP Server (Python)

Separate repo or directory: `mcp-server/`

```python
from fastmcp import FastMCP

mcp = FastMCP("secsgem")
STUDIO_URL = "https://dashai-api.onrender.com/factory/api/v1"

@mcp.tool()
def get_equipment_status() -> str:
    """Get current equipment communication and control state."""
    data = requests.get(f"{STUDIO_URL}/equipment/status").json()["data"]
    return f"Comm: {data['commState']}, Control: {data['controlState']}"

@mcp.tool()
def send_remote_command(command: str, params: dict = {}) -> str:
    """Send a remote command (RCMD) to equipment via S2F41."""
    data = requests.post(f"{STUDIO_URL}/equipment/command",
                         json={"command": command, **params}).json()["data"]
    return f"Command '{command}': {data['status']} (code={data['code']})"

# ... 8 more tools
```

## Implementation Phases

### Phase 1: Settings UI + AI Proxy
- Add Settings panel to studio-site (localStorage BYOK)
- Add `/equipment/studio/ai-proxy` to dashai-api
- Test with Anthropic API key

### Phase 2: Chat Tab
- Add Chat tab to studio-site
- System prompt + tool definitions
- Tool execution loop (AI calls tools → execute → return results)
- Chat history in memory (not persisted)

### Phase 3: MCP Server
- Create mcp-server/ directory (or separate repo)
- 10 MCP tools wrapping equipment REST API
- README with installation instructions for Claude Code

### Phase 4: Ollama Support
- Direct browser → localhost:11434 (no proxy)
- Model selection UI
- Test with llama3 / mistral

## Checklist

- [ ] Phase 1: Settings panel (provider, API key, model)
- [ ] Phase 1: AI proxy endpoint in dashai-api
- [ ] Phase 1: Test Connection button
- [ ] Phase 2: Chat tab HTML/CSS
- [ ] Phase 2: System prompt with tool definitions
- [ ] Phase 2: Tool execution loop
- [ ] Phase 2: Chat message rendering (user/assistant/tool results)
- [ ] Phase 3: MCP Server (10 tools)
- [ ] Phase 3: Claude Code integration test
- [ ] Phase 4: Ollama direct connection
- [ ] Legal: AI disclosure in chat responses
