# Self-Bootstrapping Design: ai-workflow develops itself

## Goal

ai-workflow uses its own issue/run/review/merge pipeline to develop itself.
Claude Code acts as outer orchestrator via A2A protocol, progressively
handing off control as the system matures.

## Architecture

```
User (you)
 ↕ conversation
Claude Code (outer orchestrator)
 ↕ A2A protocol (JSON-RPC 2.0, message/send + tasks/get)
ai-workflow server (Web primary, CLI auxiliary, no TUI)
 ↕ ACP protocol (JSON-RPC over stdio)
Codex/Claude agent (writes code)
 ↕ git
ai-workflow repo (self-modification target)
```

## Entry Points

- **Web Dashboard** — primary UI for issue creation, run monitoring, timeline
- **CLI** (`ai-flow`) — auxiliary for server startup, debugging, ad-hoc runs
- **A2A** — Claude Code sends tasks programmatically
- **GitHub Webhook** — external trigger (Wave 5)
- **TUI** — removed

## Trust Model

- Profile: `fast_release` (lightweight review, auto-merge)
- Safety net: `go test ./...` must pass before merge
- Claude Code supervises early waves, progressively withdraws

## Current Gaps (3 blockers)

| Gap | Severity | Fix |
|-----|----------|-----|
| `A2ABridge` not injected at server startup | HIGH | ~5 lines in `cmd/ai-flow/commands.go` |
| Event persistence (in-memory only) | HIGH | Add `run_events` table + DB subscriber on EventBus |
| Review → auto-merge bridge missing | HIGH | Event handler on `RunDone` → check `AutoMerge` → `SCM.MergePR()` |

## Already Working

- Core domain models (Issue, WorkflowRun, WorkflowProfile)
- ACP client + Run Engine executor (agent invocation via stdio)
- SQLite Store with full CRUD (runs, issues, checkpoints, reviews)
- REST API V2 routes (issues, runs, profiles, timeline)
- A2A handlers (message/send, stream, tasks/get, tasks/cancel)
- A2A → Issue 1:1 mapping with state translation
- Team Leader orchestration + Review Orchestrator
- GitHub integration (webhook, PR, slash commands)
- Git operations (branch, commit, push, merge, worktree)
- 100+ tests passing

## Wave Plan

### Wave 1: Infrastructure (A2A + Event Persistence + Remove TUI)

**Goal**: A2A endpoint functional, events persisted to DB.

Work:
- Fix `A2ABridge` injection in `cmd/ai-flow/commands.go` (~5 lines)
- Add `run_events` SQLite table (reuse `chat_run_events` schema or extend)
- Add persistent subscriber to EventBus: `run_*` events auto-saved
- Add `SaveRunEvent` / `ListRunEvents` to Store interface
- Delete `internal/tui/` package and `tui` CLI subcommand

Acceptance:
- `a2a-smoke` sends message → Issue created → events persisted
- `GET /sessions/{id}/runs/events` returns DB-backed events
- `internal/tui/` no longer exists

### Wave 2: End-to-End Run via A2A

**Goal**: Claude Code sends A2A task → agent actually executes code task.

Work:
- Claude Code calls `message/send` with a real coding task for ai-workflow repo
- Verify: Issue created → Team Leader routes → Run starts → ACP agent executes → Run terminal state
- Fix integration issues (prompt templates, workdir, stage timeouts)
- Verify event trail in DB

Acceptance:
- A2A task "add A2A docs to README" → agent executes → run reaches `done`
- Full event trail queryable via API

**Bootstrap acceleration point**: From here, Wave 3+ tasks can be dispatched as A2A tasks.

### Wave 3: Review + Auto-Merge

**Goal**: Run success → PR → test → merge, fully automatic.

Work:
- Add event handler: `RunDone` → check `Issue.AutoMerge` → `SCM.CreatePR()` → `SCM.MergePR()`
- `fast_release` ReviewOrchestrator: single reviewer, quick pass
- Pre-merge gate: `go test ./...` must pass
- Write `auto_merged` timeline event

Acceptance:
- A2A task → agent writes code → PR created → tests pass → auto-merge → issue `done`
- Code actually in `main` branch

### Wave 4: Web Observability

**Goal**: Real-time visibility in browser.

Work:
- Web Dashboard: issue list, run status, event stream, timeline view
- WebSocket push V2 events to frontend
- React frontend renders A2A-driven task lifecycle

Acceptance:
- User sees live run progress in browser while Claude Code drives via A2A

### Wave 5: GitHub External Loop + Handoff

**Goal**: System operates independently from GitHub triggers.

Work:
- GitHub webhook `issues.opened` / `issue_comment.created` → Issue → Run → PR → merge
- Slash commands (`/run`, `/review`, `/cancel`) functional
- Claude Code role degrades to "monitor exceptions only"

Acceptance:
- Open GitHub Issue on ai-workflow repo → system auto-completes → PR merged → Issue closed
- Claude Code only intervenes on failure

## Key Design Decisions

1. **A2A over direct Web API**: Validates the protocol itself; provides natural supervision layer.
2. **No TUI**: Web is the primary UI; TUI adds maintenance burden for marginal value.
3. **fast_release trust**: Speed over caution; `go test` is the minimum safety net.
4. **Progressive handoff**: Claude Code starts fully engaged (Wave 1-2), semi-autonomous (Wave 3-4), observer-only (Wave 5).
5. **Bottom-up waves**: Each wave validates the layer below before building on it.
