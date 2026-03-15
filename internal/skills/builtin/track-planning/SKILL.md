---
name: track-planning
description: Drive the planning phase of a WorkItemTrack. Read track and thread context, design an execution plan following plan-core principles, and submit the structured plan for review. Use when assigned to plan a track that is in draft or planning status.
---

# Track Planning

You are acting as the **planner** for a WorkItemTrack. Your job is to turn a discussion thread into a concrete, reviewable execution plan.

## Planning Workflow

Follow these steps in order:

### Step 1 — Gather Context

1. Read the track to understand the title, objective, and current status.
2. Read the primary thread's summary and recent messages to understand what was discussed.
3. If other threads are linked as source/context, read their summaries too.
4. Browse the relevant codebase if the objective involves code changes.

```bash
SKILL_HOME="${CODEX_HOME:-${CLAUDE_CONFIG_DIR:-}}/skills/track-planning"
bash "$SKILL_HOME/scripts/get-track.sh" <track-id>
bash "$SKILL_HOME/scripts/get-thread.sh" <thread-id>
bash "$SKILL_HOME/scripts/list-messages.sh" <thread-id> [limit]
```

```powershell
$skillHome = if ($env:CODEX_HOME) {
  Join-Path $env:CODEX_HOME "skills\track-planning"
} elseif ($env:CLAUDE_CONFIG_DIR) {
  Join-Path $env:CLAUDE_CONFIG_DIR "skills\track-planning"
} else { $null }

pwsh -NoProfile -File "$skillHome\scripts\get-track.ps1" <track-id>
pwsh -NoProfile -File "$skillHome\scripts\get-thread.ps1" <thread-id>
pwsh -NoProfile -File "$skillHome\scripts\list-messages.ps1" <thread-id> [limit]
```

### Step 2 — Check Available Capabilities

Before designing the plan, query the current agent profiles to understand what execution resources are available.

```bash
bash "$SKILL_HOME/scripts/list-profiles.sh"
```

```powershell
pwsh -NoProfile -File "$skillHome\scripts\list-profiles.ps1"
```

Each profile has:
- `id` — profile identifier (used as `agent_role` reference)
- `role` — `lead`, `worker`, `gate`, or `support`
- `capabilities` — capability tags (e.g. `["backend", "frontend", "test"]`)

**You must only assign `agent_role` and `required_capabilities` that existing profiles can satisfy.** Cross-check every step in your plan against the profile list before submitting.

If a step requires capabilities that no profile provides, you must **NOT** silently drop the step or proceed with an unfulfillable plan. Instead, follow the Capability Gap Escalation procedure below.

### Step 3 — Design the Plan

Apply the `plan-core` skill guidelines to design a DAG of execution steps:

1. Restate the track's objective in execution terms.
2. Identify the minimum viable set of steps.
3. For each step, define:
   - `name` — unique, lowercase, dash-separated
   - `type` — `exec` (implementation), `gate` (review), or `composite` (sub-workflow)
   - `description` — what must be accomplished
   - `agent_role` — `worker`, `gate`, `lead`, or `support`
   - `required_capabilities` — capability tags needed
   - `acceptance_criteria` — concrete conditions for completion
   - `depends_on` — names of upstream steps (if any)
4. Insert `gate` steps where review or quality validation is needed.
5. Keep steps in topological order (dependencies before dependents).
6. Maximize parallelism — only add `depends_on` for real prerequisites.

### Step 4 — Submit for Review

Once the plan is ready, submit it using the submit-review script. This advances the track from `draft`/`planning` to `reviewing`.

```bash
bash "$SKILL_HOME/scripts/submit-review.sh" <track-id> '<summary>' '<planner-output-json>'
```

```powershell
pwsh -NoProfile -File "$skillHome\scripts\submit-review.ps1" <track-id> '<summary>' '<planner-output-json>'
```

The `planner_output_json` must be a JSON object with a `steps` array:

```json
{
  "steps": [
    {
      "name": "implement-auth",
      "type": "exec",
      "description": "Implement JWT authentication middleware",
      "agent_role": "worker",
      "required_capabilities": ["backend"],
      "acceptance_criteria": ["JWT tokens are issued on login", "Token validation middleware works"],
      "depends_on": []
    },
    {
      "name": "review-auth",
      "type": "gate",
      "description": "Review the authentication implementation",
      "agent_role": "gate",
      "required_capabilities": ["review"],
      "acceptance_criteria": ["Code review passed", "No security vulnerabilities"],
      "depends_on": ["implement-auth"]
    }
  ],
  "analysis": "Brief analysis of the requirement and approach taken"
}
```

The `summary` should be a concise natural-language description of the plan for the reviewer and the user.

### Step 5 — Post a Planning Message

After submitting, post a message in the thread explaining the plan in human-readable form. Include:

1. What the plan covers
2. How many steps and their high-level flow
3. Key decisions or trade-offs made
4. Any open questions or risks

This message should include the track ID in metadata so the frontend can associate it with the track.

## Available Context Variables

| Variable | Meaning |
|---|---|
| `AI_WORKFLOW_SERVER_ADDR` | Backend API base URL |
| `AI_WORKFLOW_API_TOKEN` | Bearer token for API authentication |
| `AI_WORKFLOW_TRACK_ID` | The track ID being planned (if injected) |
| `AI_WORKFLOW_THREAD_ID` | The primary thread ID (if injected) |
| `CODEX_HOME` / `CLAUDE_CONFIG_DIR` | Agent home directory |

## Capability Gap Escalation

If you discover that one or more steps require a role or capability that no existing profile can satisfy, you must **stop and report** instead of submitting an incomplete plan.

Post a message in the thread with the following structure:

```
⚠️ 能力缺口报告

在为任务轨道"<track-title>"制定规划时，发现以下步骤所需的能力当前系统不具备：

| 步骤 | 所需角色 | 所需能力 | 缺口说明 |
|------|---------|---------|---------|
| <step-name> | <role> | <capabilities> | <哪个 profile 最接近但缺什么> |

当前可用 profiles:
- <profile-id>: role=<role>, capabilities=<caps>
- ...

请决定:
1. 调整需求范围，跳过这些步骤
2. 新增或更新 agent profile 以补充能力
3. 将缺口步骤标记为手动执行
```

**Do not call submit-review when there are unresolved capability gaps.** Wait for the user to respond in the thread before continuing.

After the user responds:
- If they adjust the scope → redesign the plan and submit.
- If they add a profile → re-run `list-profiles`, verify the gap is filled, then submit.
- If they mark steps as manual → include those steps with `agent_role: "manual"` and a note in the description, then submit.

## When to Ask for Clarification

Do NOT proceed with a plan if:

1. The objective is too vague to produce actionable steps.
2. Critical technical decisions are missing (e.g., which framework, which database).
3. The scope is unclear — could mean two very different things.

Instead, post a message in the thread asking the user for clarification. Keep the track in `planning` status until you have enough information.

## Quality Checklist

Before submitting, verify:

- [ ] Every step has a clear description and at least one acceptance criterion.
- [ ] Step names are unique and follow lowercase-dash convention.
- [ ] Dependencies are minimal and correct (no circular deps).
- [ ] Agent roles match the nature of the work.
- [ ] **Every step's `agent_role` + `required_capabilities` can be satisfied by at least one existing profile.**
- [ ] No unresolved capability gaps (all gaps have been escalated and resolved).
- [ ] The plan is immediately executable without further interpretation.
- [ ] Gate steps are placed at critical quality checkpoints.

## Rules

1. Do not skip context gathering — always read the track and thread first.
2. Prefer fewer, outcome-oriented steps over many procedural micro-steps.
3. Do not create the WorkItem yourself — that happens after review approval.
4. Submit exactly once per planning round. If rejected, read the reviewer feedback and replan.
5. Keep the summary concise but specific enough for the reviewer to evaluate.
