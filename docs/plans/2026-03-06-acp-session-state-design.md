# ACP Session State: Commands, ConfigOptions, and SessionUpdate Optimization

**Date**: 2026-03-06
**Status**: Approved

## Summary

Three changes in one pass:

1. **SessionUpdate structure optimization** — replace `RawUpdateJSON`/`RawContentJSON` strings with a single `json.RawMessage` field; extract `Commands`/`ConfigOptions` at decode time
2. **Available commands** — capture `available_commands_update` into session state, expose via API and WebSocket, frontend `/` command palette
3. **Config options** — capture `config_option_update` and `session/new` response, expose via API and WebSocket, frontend config selector, support `session/set_config_option`

## Part 1: SessionUpdate Structure

### Before

```go
type SessionUpdate struct {
    SessionID      string `json:"sessionId"`
    Type           string `json:"type"`
    Text           string `json:"text,omitempty"`
    Status         string `json:"status,omitempty"`
    RawUpdateJSON  string `json:"rawUpdateJson,omitempty"`  // redundant string
    RawContentJSON string `json:"rawContentJson,omitempty"` // unused
}
```

### After

```go
type SessionUpdate struct {
    SessionID string            `json:"sessionId"`
    Type      string            `json:"type"`
    Text      string            `json:"text,omitempty"`
    Status    string            `json:"status,omitempty"`
    RawJSON   json.RawMessage   `json:"raw"`

    Commands      []acpproto.AvailableCommand          `json:"-"`
    ConfigOptions []acpproto.SessionConfigOptionSelect  `json:"-"`
}
```

### Changes

- Delete `RawUpdateJSON` (string) and `RawContentJSON` (string, never consumed)
- Add `RawJSON` (`json.RawMessage`) — zero-copy from `json.Marshal(notification.Update)`
- Add `Commands` — populated when `Type == "available_commands_update"`
- Add `ConfigOptions` — populated when `Type == "config_option_update"`
- `decodeACPNotificationFromStruct`: extract structured fields from SDK types
- Consumers: `json.Unmarshal([]byte(update.RawUpdateJSON), ...)` → `json.Unmarshal(update.RawJSON, ...)`

### Fixture format

Before (redundant):
```json
{
  "offset_ms": 2216,
  "update": {
    "sessionId": "...",
    "type": "agent_message_chunk",
    "text": "_OK",
    "rawUpdateJson": "{...}",
    "rawContentJson": "{...}"
  }
}
```

After (raw ACP notification only):
```json
{
  "offset_ms": 2216,
  "raw": {
    "sessionId": "...",
    "update": {
      "sessionUpdate": "agent_message_chunk",
      "content": {"type": "text", "text": "_OK"}
    }
  }
}
```

- fixture_agent sends `raw` directly as `session/update` params
- No extracted/duplicated fields

## Part 2: Available Commands

### Backend

**Session state**: `pooledChatSession` gains:
```go
commandsMu sync.RWMutex
commands   []acpproto.AvailableCommand
```

Updated when `ACPHandler.HandleSessionUpdate` receives `available_commands_update` (reads `update.Commands`).

**API**:
```
GET /api/v1/chat/sessions/{id}/commands → []AvailableCommand
```

Returns current commands list from session memory. 404 if session not found.

**WebSocket**: already broadcasts `RawJSON` via existing `EventRunUpdate` path — frontend receives the full `available_commands_update` payload.

### Frontend

**Store** (`chatStore`):
```typescript
commandsBySessionId: Record<string, AvailableCommand[]>
```

Updated on:
- WS message with `acp.sessionUpdate === "available_commands_update"`
- Initial fetch via GET on session connect/reconnect

**UI**: Input box `/` trigger → popup list of commands (name + description). Selection fills input with `/commandName [hint]`.

## Part 3: Config Options

### Backend

**Session state**: `pooledChatSession` gains:
```go
configOptions []acpproto.SessionConfigOptionSelect
```

Updated when:
- `config_option_update` notification received
- `session/new` or `session/load` response contains `configOptions`

**API**:
```
GET  /api/v1/chat/sessions/{id}/config-options → []ConfigOptionSelect
POST /api/v1/chat/sessions/{id}/config-options → {configId, value} → []ConfigOptionSelect
```

POST calls `client.SetConfigOption()` (new method on `acpclient.Client`), which sends `session/set_config_option` to the agent. Agent returns complete config state; backend updates session and returns it.

**`acpclient.Client` new method**:
```go
func (c *Client) SetConfigOption(ctx context.Context, req acpproto.SetConfigOptionRequest) ([]acpproto.SessionConfigOptionSelect, error)
```

### Frontend

**Store** (`chatStore`):
```typescript
configOptionsBySessionId: Record<string, ConfigOption[]>
```

**UI**: Config selector in chat header/toolbar. Each option rendered as a dropdown (`type: "select"`). Change triggers POST → updates store from response.

## LoadSession Suppress Refinement

The existing `ACPHandler.SetSuppressEvents(true)` blocks WS broadcast and persistence. But during `LoadSession`, replayed `available_commands_update` and `config_option_update` must still update session memory state.

Suppress behavior:
- suppress=true: **write** commands/configOptions to session state, **skip** publish + record
- suppress=false: write + publish + record (normal)

## Boundary Cases

| Scenario | Handling |
|----------|----------|
| `session/new` response includes `configOptions` | Parse and initialize session state |
| `LoadSession` replays `available_commands_update` | Write to state (suppress skips broadcast) |
| Agent pushes `config_option_update` (e.g. model fallback) | Update state + WS broadcast |
| `SetConfigOption` agent error | Return error to frontend, no local state change |
| Session not found or closed | 404 |
| Empty commands/configOptions arrays | Frontend hides respective UI |

## Out of Scope

- No DB persistence for commands/configOptions (memory, tied to session lifecycle)
- No command argument input UI (`input.hint` shown as placeholder text only)
- No custom configOption types (only `select` per protocol)
- No eventbus refactor for ACP notification handling
