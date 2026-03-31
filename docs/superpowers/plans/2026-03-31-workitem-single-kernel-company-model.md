# WorkItem Single-Kernel Company Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild task orchestration around `WorkItem` as the only task truth, with profile reporting chains, unified `Deliverable`, CEO task decomposition, and `active_profile_id`-driven pending views.

**Architecture:** Add organization-chain fields to agent profiles, extend `WorkItem` into the sole responsibility carrier, introduce one shared `Deliverable` model for `Run`/`Thread`/`WorkItem`, and cut read/write paths from `metadata["ceo"]` + `ActionSignal` pending queries over to `WorkItem` + `Deliverable` + `Journal`. Keep `Action`/`Run` as execution infrastructure only and move CEO orchestration to the new fields immediately after backfill.

**Tech Stack:** Go, SQLite + Gorm AutoMigrate, existing `internal/application/*` services, Cobra CLI, HTTP adapters, React + TypeScript frontend, existing PowerShell test scripts.

---

## File Map

### Existing files to modify

- `internal/core/agent.go`
  Add `ManagerProfileID` and expose the organization-chain field on `AgentProfile`.
- `internal/core/workitem.go`
  Replace legacy status model with the single-kernel status set and add responsibility/result fields.
- `internal/core/store.go`
  Extend aggregate store interfaces for deliverables and richer work-item queries.
- `internal/core/journal.go`
  Add the minimal event fields/filters needed for `WorkItem` responsibility history.
- `internal/platform/config/types.go`
  Add `manager_profile_id` to runtime profile config.
- `internal/platform/config/role_driven.go`
  Validate manager references and prevent broken profile trees.
- `internal/platform/config/defaults.toml`
  Seed CEO org-chain defaults and at least one manageable subordinate profile relation.
- `internal/adapters/store/sqlite/models.go`
  Add new SQLite columns/models for work-item responsibility and unified deliverables.
- `internal/adapters/store/sqlite/schema.go`
  Register new models, indexes, and idempotent backfill hooks.
- `internal/adapters/store/sqlite/workitem.go`
  Persist/read new work-item fields and add pending query helpers.
- `internal/adapters/store/sqlite/journal.go`
  Persist/read richer work-item responsibility events.
- `internal/adapters/store/sqlite/action_signal.go`
  Retire pending-human query semantics or downgrade them to compatibility wrappers.
- `internal/application/workitemapp/contracts.go`
  Extend contracts for richer work-item updates, deliverable adoption, and backfill support.
- `internal/application/workitemapp/service.go`
  Create/update work items using the new fields and final deliverable semantics.
- `internal/application/flow/engine.go`
  Drive status transitions into the new state machine and emit work-item events.
- `internal/application/flow/recovery.go`
  Update recovery paths to new status names.
- `internal/application/flow/resolver.go`
  Continue resolving execution profiles, but never treat that as task truth.
- `internal/application/orchestrateapp/contracts.go`
  Switch CEO orchestration to work-item responsibility fields and deliverable-centric results.
- `internal/application/orchestrateapp/service.go`
  Remove `ceo_journal` / assigned-profile metadata writes and use active responsibility + journal events.
- `internal/platform/appcmd/orchestrate.go`
  Expose new orchestration verbs/JSON payloads around active owner, escalation, and deliverable adoption.
- `internal/adapters/http/agents.go`
  Expose `manager_profile_id` over the profile admin API.
- `internal/adapters/http/handler.go`
  Wire deliverable handlers and new work-item pending views.
- `internal/adapters/http/workitem.go`
  Add `active_profile_id`-based filtering and parent/root work-item fields to the public API.
- `internal/adapters/http/action_signal.go`
  Downgrade `/pending-decisions` to a compatibility wrapper or remove it from the main task truth path.
- `internal/adapters/http/artifact.go`
  Either evolve into deliverable handlers or delegate to a new deliverable surface.
- `web/src/types/api-v2/workflow.ts`
  Replace work-item DTOs/statuses with the single-kernel model.
- `web/src/lib/apiClient.workflow.ts`
  Add pending/escalation/final-deliverable API calls.
- `web/src/lib/apiClient.agentAdmin.ts`
  Include `manager_profile_id` in profile CRUD payloads.
- `web/src/types/api-v2/agent-admin.ts`
  Extend profile DTOs with `manager_profile_id`.
- `web/src/pages/AgentsPage.tsx`
  Surface the reporting chain in the profile management UI.
- `web/src/components/agents/CreateProfileDialog.tsx`
  Allow editing/selecting `manager_profile_id`.
- `web/src/pages/chat/useChatSessionController.ts`
  Reflect real profile hierarchy labels where CEO/operator views need them.
- `web/src/pages/MobileHomePage.tsx`
  Update work-item and profile-derived labels if this page surfaces the same profile list.

### New files to create

- `internal/core/deliverable.go`
  Shared deliverable domain model, kinds, producer types, and store/query interfaces.
- `internal/adapters/store/sqlite/deliverable.go`
  SQLite CRUD/query implementation for unified deliverables.
- `internal/application/workitemapp/backfill.go`
  Explicit backfill logic from legacy work-item state/metadata/run results into the new model.
- `internal/application/workitemapp/reporting_chain.go`
  Organization-chain resolver and active escalation-path calculator.
- `internal/application/workitemapp/deliverable.go`
  Service helpers for adopting a deliverable as a work item’s final result.
- `internal/adapters/http/deliverable.go`
  HTTP read/write surface for deliverables and work-item deliverable adoption.
- `web/src/types/api-v2/deliverable.ts`
  Frontend deliverable DTOs.

### Existing tests to extend

- `internal/platform/config/config_test.go`
- `internal/platform/bootstrap/bootstrap_registry_test.go`
- `internal/adapters/store/sqlite/store_test.go`
- `internal/application/workitemapp/service_test.go`
- `internal/application/orchestrateapp/service_test.go`
- `internal/application/flow/engine_test.go`
- `internal/application/flow/recovery_test.go`
- `internal/adapters/http/artifact_test.go`
- `web/src/lib/apiClient.test.ts`
- `web/src/pages/AgentsPage.test.tsx`

### New tests to create

- `internal/adapters/store/sqlite/deliverable_test.go`
- `internal/application/workitemapp/backfill_test.go`
- `internal/application/workitemapp/reporting_chain_test.go`
- `internal/adapters/http/deliverable_test.go`

---

### Task 1: Add Profile Reporting Chains

**Files:**
- Modify: `internal/core/agent.go`
- Modify: `internal/platform/config/types.go`
- Modify: `internal/platform/config/role_driven.go`
- Modify: `internal/platform/config/defaults.toml`
- Modify: `internal/adapters/store/sqlite/models.go`
- Modify: `internal/platform/bootstrap/bootstrap_registry.go`
- Modify: `internal/platform/config/config_test.go`
- Modify: `internal/platform/bootstrap/bootstrap_registry_test.go`

- [ ] **Step 1: Write the failing profile config/registry tests**

```go
func TestValidateRuntimeProfilesRejectsMissingManager(t *testing.T) {
	cfg := minimalRuntimeConfig()
	cfg.Runtime.Agents.Profiles = append(cfg.Runtime.Agents.Profiles,
		RuntimeProfileConfig{ID: "ceo", Driver: "codex", Role: "lead"},
		RuntimeProfileConfig{ID: "lead", Driver: "codex", Role: "lead", ManagerProfileID: "missing"},
	)
	if err := ValidateRuntime(cfg); err == nil {
		t.Fatal("expected manager validation error")
	}
}
```

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `go test ./internal/platform/config ./internal/platform/bootstrap -run 'TestValidateRuntimeProfilesRejectsMissingManager|TestBootstrapRegistrySeedsOnlyCEO' -count=1`

Expected: FAIL because `manager_profile_id` does not exist yet.

- [ ] **Step 3: Add `manager_profile_id` through config, core, SQLite, and seed defaults**

```go
type AgentProfile struct {
	ID               string `json:"id"`
	ManagerProfileID string `json:"manager_profile_id,omitempty"`
	// ...
}

type RuntimeProfileConfig struct {
	ID               string `toml:"id" json:"id"`
	ManagerProfileID string `toml:"manager_profile_id" json:"manager_profile_id"`
}
```

- [ ] **Step 4: Re-run the config/bootstrap tests**

Run: `go test ./internal/platform/config ./internal/platform/bootstrap -count=1`

Expected: PASS, including manager validation and CEO seed assertions.

- [ ] **Step 5: Commit**

```bash
git add internal/core/agent.go internal/platform/config/types.go internal/platform/config/role_driven.go internal/platform/config/defaults.toml internal/adapters/store/sqlite/models.go internal/platform/bootstrap/bootstrap_registry.go internal/platform/config/config_test.go internal/platform/bootstrap/bootstrap_registry_test.go
git commit -m "feat(profile): add manager reporting chain fields"
```

---

### Task 2: Introduce Unified Deliverables

**Files:**
- Create: `internal/core/deliverable.go`
- Create: `internal/adapters/store/sqlite/deliverable.go`
- Modify: `internal/core/store.go`
- Modify: `internal/adapters/store/sqlite/models.go`
- Modify: `internal/adapters/store/sqlite/schema.go`
- Modify: `internal/core/artifact_contract.go`
- Create: `internal/adapters/store/sqlite/deliverable_test.go`
- Modify: `internal/adapters/http/artifact_test.go`

- [ ] **Step 1: Write the failing deliverable store test**

```go
func TestDeliverableStoreCreateAndListByWorkItem(t *testing.T) {
	store := newTestStore(t)
	id, err := store.CreateDeliverable(context.Background(), &core.Deliverable{
		WorkItemID:    ptr[int64](12),
		Kind:          core.DeliverablePullRequest,
		Title:         "Open PR",
		Summary:       "PR ready for review",
		ProducerType:  core.DeliverableProducerRun,
		ProducerID:    33,
		Status:        core.DeliverableFinal,
	})
	if err != nil || id == 0 {
		t.Fatalf("CreateDeliverable() id=%d err=%v", id, err)
	}
}
```

- [ ] **Step 2: Run the store tests to confirm they fail**

Run: `go test ./internal/adapters/store/sqlite -run 'TestDeliverableStoreCreateAndListByWorkItem|TestArtifactResponseNormalizesMetadata' -count=1`

Expected: FAIL because the deliverable model/store does not exist.

- [ ] **Step 3: Add a shared deliverable model and SQLite persistence**

```go
type Deliverable struct {
	ID           int64
	WorkItemID   *int64
	ThreadID     *int64
	Kind         DeliverableKind
	Title        string
	Summary      string
	Payload      map[string]any
	ProducerType DeliverableProducerType
	ProducerID   int64
	Status       DeliverableStatus
	CreatedAt    time.Time
}
```

- [ ] **Step 4: Re-run deliverable + artifact compatibility tests**

Run: `go test ./internal/adapters/store/sqlite ./internal/adapters/http -run 'TestDeliverable|TestArtifact' -count=1`

Expected: PASS, with legacy artifact HTTP still readable through normalized deliverable semantics.

- [ ] **Step 5: Commit**

```bash
git add internal/core/deliverable.go internal/adapters/store/sqlite/deliverable.go internal/core/store.go internal/adapters/store/sqlite/models.go internal/adapters/store/sqlite/schema.go internal/core/artifact_contract.go internal/adapters/store/sqlite/deliverable_test.go internal/adapters/http/artifact_test.go
git commit -m "feat(deliverable): add unified deliverable model"
```

---

### Task 3: Expand WorkItem Into The Single-Kernel Task Model

**Files:**
- Modify: `internal/core/workitem.go`
- Modify: `internal/adapters/store/sqlite/models.go`
- Modify: `internal/adapters/store/sqlite/workitem.go`
- Modify: `internal/application/workitemapp/contracts.go`
- Modify: `internal/application/workitemapp/service.go`
- Modify: `internal/application/workitemapp/service_test.go`
- Modify: `internal/adapters/http/workitem.go`
- Modify: `internal/adapters/http/handler_test.go`
- Modify: `web/src/types/api-v2/workflow.ts`

- [ ] **Step 1: Write the failing work-item create/update tests**

```go
func TestServiceCreateWorkItemPersistsActiveProfileAndFinalDeliverable(t *testing.T) {
	svc := newTestService(t)
	item, err := svc.CreateWorkItem(context.Background(), CreateWorkItemInput{
		Title:             "Implement login",
		Metadata:          map[string]any{},
		ExecutorProfileID: "lead",
		ReviewerProfileID: "ceo",
		ActiveProfileID:   "lead",
		SponsorProfileID:  "ceo",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem: %v", err)
	}
	if item.ActiveProfileID != "lead" {
		t.Fatalf("ActiveProfileID = %q, want lead", item.ActiveProfileID)
	}
}
```

- [ ] **Step 2: Run the focused work-item tests and TypeScript typecheck target**

Run: `go test ./internal/application/workitemapp ./internal/adapters/store/sqlite -run 'TestServiceCreateWorkItemPersistsActiveProfileAndFinalDeliverable|TestStoreWorkItemCRUD' -count=1`

Expected: FAIL because the new fields/statuses are undefined.

- [ ] **Step 3: Add single-kernel fields/statuses and persistence**

```go
type WorkItem struct {
	// ...
	ExecutorProfileID  string   `json:"executor_profile_id,omitempty"`
	ReviewerProfileID  string   `json:"reviewer_profile_id,omitempty"`
	ActiveProfileID    string   `json:"active_profile_id,omitempty"`
	SponsorProfileID   string   `json:"sponsor_profile_id,omitempty"`
	CreatedByProfileID string   `json:"created_by_profile_id,omitempty"`
	ParentWorkItemID   *int64   `json:"parent_work_item_id,omitempty"`
	RootWorkItemID     *int64   `json:"root_work_item_id,omitempty"`
	EscalationPath     []string `json:"escalation_path,omitempty"`
	FinalDeliverableID *int64   `json:"final_deliverable_id,omitempty"`
}
```

- [ ] **Step 4: Re-run work-item tests**

Run: `go test ./internal/core ./internal/application/workitemapp ./internal/adapters/store/sqlite ./internal/adapters/http -count=1`

Expected: PASS for updated domain/store/service tests.

- [ ] **Step 5: Commit**

```bash
git add internal/core/workitem.go internal/adapters/store/sqlite/models.go internal/adapters/store/sqlite/workitem.go internal/application/workitemapp/contracts.go internal/application/workitemapp/service.go internal/application/workitemapp/service_test.go internal/adapters/http/workitem.go internal/adapters/http/handler_test.go web/src/types/api-v2/workflow.ts
git commit -m "feat(workitem): add single-kernel responsibility fields"
```

---

### Task 4: Add Reporting-Chain Resolution And Legacy Backfill

**Files:**
- Create: `internal/application/workitemapp/reporting_chain.go`
- Create: `internal/application/workitemapp/reporting_chain_test.go`
- Create: `internal/application/workitemapp/backfill.go`
- Create: `internal/application/workitemapp/backfill_test.go`
- Modify: `internal/adapters/store/sqlite/schema.go`
- Modify: `internal/application/workitemapp/service.go`
- Modify: `internal/core/journal.go`
- Modify: `internal/adapters/store/sqlite/journal.go`

- [ ] **Step 1: Write failing reporting-chain and backfill tests**

```go
func TestBuildEscalationPathUsesCurrentManagerChain(t *testing.T) {
	path, err := BuildEscalationPath("worker", registry)
	if err != nil {
		t.Fatalf("BuildEscalationPath: %v", err)
	}
	want := []string{"lead", "ceo", "human"}
	if !reflect.DeepEqual(path, want) {
		t.Fatalf("path = %#v, want %#v", path, want)
	}
}
```

- [ ] **Step 2: Run the targeted tests to confirm they fail**

Run: `go test ./internal/application/workitemapp ./internal/adapters/store/sqlite -run 'TestBuildEscalationPathUsesCurrentManagerChain|TestBackfillLegacyWorkItems' -count=1`

Expected: FAIL because no resolver/backfill exists.

- [ ] **Step 3: Implement organization-chain resolver, journal event contract, and idempotent backfill**

```go
func BackfillLegacyWorkItems(ctx context.Context, store Store, registry core.AgentRegistry) error {
	items, _ := store.ListWorkItems(ctx, core.WorkItemFilter{Limit: 1000})
	for _, item := range items {
		patch := deriveLegacyPatch(item)
		if patch.NeedsManualMigration {
			continue
		}
		if err := store.UpdateWorkItem(ctx, patch.Apply(item)); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Re-run the resolver/backfill/journal tests**

Run: `go test ./internal/application/workitemapp ./internal/adapters/store/sqlite ./internal/core ./internal/adapters/http -count=1`

Expected: PASS, including explicit legacy-state mapping and manual-migration markers.

- [ ] **Step 5: Commit**

```bash
git add internal/application/workitemapp/reporting_chain.go internal/application/workitemapp/reporting_chain_test.go internal/application/workitemapp/backfill.go internal/application/workitemapp/backfill_test.go internal/adapters/store/sqlite/schema.go internal/application/workitemapp/service.go internal/core/journal.go internal/adapters/store/sqlite/journal.go
git commit -m "feat(workitem): add reporting chain and legacy backfill"
```

---

### Task 5: Refactor CEO Orchestration To The New WorkItem Model

**Files:**
- Modify: `internal/application/orchestrateapp/contracts.go`
- Modify: `internal/application/orchestrateapp/service.go`
- Modify: `internal/application/orchestrateapp/service_test.go`
- Modify: `internal/platform/appcmd/orchestrate.go`
- Modify: `cmd/ai-flow/orchestrate_cmd.go`
- Modify: `configs/prompts/ceo_orchestrator.tmpl`
- Modify: `internal/skills/builtin/ceo-manage/SKILL.md`

- [ ] **Step 1: Write the failing CEO orchestration tests**

```go
func TestServiceCreateTaskSeedsExecutorReviewerAndSponsor(t *testing.T) {
	result, err := svc.CreateTask(ctx, CreateTaskInput{
		Title:            "Ship login",
		ParentWorkItemID: ptr[int64](7),
		RootWorkItemID:   ptr[int64](3),
		ExecutorProfile:  "lead",
		ReviewerProfile:  "ceo",
		SponsorProfile:   "ceo",
		SourceGoalRef:    "goal:login",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if result.WorkItem.ActiveProfileID != "lead" {
		t.Fatalf("active owner = %q, want lead", result.WorkItem.ActiveProfileID)
	}
}

func TestServiceReassignTaskDoesNotReadAssignedProfileFromLegacyMetadata(t *testing.T) {}
func TestServiceFollowUpTaskUsesActiveProfileAndFinalDeliverable(t *testing.T) {}
func TestServiceEscalateThreadDoesNotAppendCEOJournal(t *testing.T) {}
func TestServiceCreateTaskDedupeUsesDedicatedFields(t *testing.T) {}
```

- [ ] **Step 2: Run the orchestration CLI/service tests**

Run: `go test ./internal/application/orchestrateapp ./internal/platform/appcmd ./cmd/ai-flow -count=1`

Expected: FAIL because orchestration still writes metadata-based assignment/journal state.

- [ ] **Step 3: Rewrite orchestration to use responsibility fields + journal events**

```go
entry := core.WorkItemJournalEntry{
	Kind:          "workitem.assigned",
	ActorProfileID: input.ActorProfile,
	FromProfileID:  oldActive,
	ToProfileID:    newActive,
}
```

- [ ] **Step 3.5: Delete old metadata read paths as part of the same cutover**

```go
// remove:
assignedProfileFromMetadata(workItem.Metadata)
appendCEOJournal(...)
withAssignedProfile(...)
```

- [ ] **Step 4: Re-run orchestration tests**

Run: `go test ./internal/application/orchestrateapp ./internal/platform/appcmd ./cmd/ai-flow -count=1`

Expected: PASS, with no new writes to `ceo_journal` or assigned-profile metadata.

- [ ] **Step 5: Commit**

```bash
git add internal/application/orchestrateapp/contracts.go internal/application/orchestrateapp/service.go internal/application/orchestrateapp/service_test.go internal/platform/appcmd/orchestrate.go cmd/ai-flow/orchestrate_cmd.go configs/prompts/ceo_orchestrator.tmpl internal/skills/builtin/ceo-manage/SKILL.md
git commit -m "refactor(ceo): move orchestration to workitem truth"
```

---

### Task 6: Rewire Execution And Human-Intervention Paths

**Files:**
- Modify: `internal/application/flow/engine.go`
- Modify: `internal/application/flow/recovery.go`
- Modify: `internal/application/flow/resolver.go`
- Modify: `internal/adapters/store/sqlite/action_signal.go`
- Modify: `internal/adapters/store/sqlite/store_test.go`
- Modify: `internal/application/flow/engine_test.go`
- Modify: `internal/application/flow/recovery_test.go`

- [ ] **Step 1: Write the failing engine/recovery tests**

```go
func TestEnginePromotesReviewerToActiveProfileAfterExecution(t *testing.T) {
	// setup omitted
	if got := updated.ActiveProfileID; got != "ceo" {
		t.Fatalf("ActiveProfileID = %q, want ceo reviewer", got)
	}
}
```

- [ ] **Step 2: Run focused engine/recovery tests**

Run: `go test ./internal/application/flow -run 'TestEnginePromotesReviewerToActiveProfileAfterExecution|TestRecoveryRequeuesNewStatuses' -count=1`

Expected: FAIL because the flow engine still uses legacy statuses and action-signal pending semantics.

- [ ] **Step 3: Update engine transitions and downgrade pending-human queries**

```go
switch workItem.Status {
case core.WorkItemPendingExecution:
	// ...
case core.WorkItemPendingReview:
	workItem.ActiveProfileID = workItem.ReviewerProfileID
}
```

- [ ] **Step 4: Re-run flow + store tests**

Run: `go test ./internal/application/flow ./internal/adapters/store/sqlite -count=1`

Expected: PASS, with action signals remaining as execution events only.

- [ ] **Step 5: Commit**

```bash
git add internal/application/flow/engine.go internal/application/flow/recovery.go internal/application/flow/resolver.go internal/adapters/store/sqlite/action_signal.go internal/adapters/store/sqlite/store_test.go internal/application/flow/engine_test.go internal/application/flow/recovery_test.go
git commit -m "refactor(flow): drive execution from workitem state"
```

---

### Task 7: Add Deliverable Adoption For Runs And Threads

**Files:**
- Create: `internal/application/workitemapp/deliverable.go`
- Create: `internal/adapters/http/deliverable.go`
- Create: `internal/adapters/http/deliverable_test.go`
- Modify: `internal/adapters/http/handler.go`
- Modify: `internal/adapters/http/artifact.go`
- Modify: `internal/application/threadapp/contracts.go`
- Modify: `internal/adapters/http/thread.go`
- Modify: `internal/application/threadapp/service.go`
- Modify: `internal/application/threadapp/service_test.go`
- Modify: `web/src/lib/apiClient.workflow.ts`
- Create: `web/src/types/api-v2/deliverable.ts`

- [ ] **Step 1: Write the failing deliverable adoption tests**

```go
func TestAdoptThreadDeliverableSetsFinalDeliverableID(t *testing.T) {
	// create thread deliverable, adopt into work item
	if workItem.FinalDeliverableID == nil {
		t.Fatal("expected final deliverable")
	}
}
```

- [ ] **Step 2: Run the targeted thread/deliverable HTTP tests**

Run: `go test ./internal/application/threadapp ./internal/adapters/http -run 'TestAdoptThreadDeliverableSetsFinalDeliverableID|TestGetLatestArtifactByAction' -count=1`

Expected: FAIL because no adoption API/service exists.

- [ ] **Step 3: Implement deliverable adoption and expose HTTP endpoints**

```go
func (s *Service) AdoptDeliverable(ctx context.Context, workItemID, deliverableID int64) error {
	return s.store.UpdateWorkItemFinalDeliverable(ctx, workItemID, deliverableID)
}
```

- [ ] **Step 4: Re-run thread/http tests**

Run: `go test ./internal/application/threadapp ./internal/adapters/http -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/application/workitemapp/deliverable.go internal/adapters/http/deliverable.go internal/adapters/http/deliverable_test.go internal/adapters/http/handler.go internal/adapters/http/artifact.go internal/application/threadapp/contracts.go internal/adapters/http/thread.go internal/application/threadapp/service.go internal/application/threadapp/service_test.go web/src/lib/apiClient.workflow.ts web/src/types/api-v2/deliverable.ts
git commit -m "feat(deliverable): support thread and workitem adoption"
```

---

### Task 8: Switch Frontend Views To `active_profile_id + status`

**Files:**
- Modify: `web/src/types/api-v2/workflow.ts`
- Modify: `web/src/types/api-v2/agent-admin.ts`
- Modify: `web/src/lib/apiClient.workflow.ts`
- Modify: `web/src/lib/apiClient.agentAdmin.ts`
- Modify: `web/src/lib/apiClient.test.ts`
- Modify: `web/src/pages/AgentsPage.tsx`
- Modify: `web/src/pages/AgentsPage.test.tsx`
- Modify: `web/src/components/agents/CreateProfileDialog.tsx`
- Modify: `web/src/pages/chat/useChatSessionController.ts`
- Modify: `web/src/pages/MobileHomePage.tsx`
- Modify: `internal/adapters/http/workitem.go`
- Modify: `internal/adapters/http/action_signal.go`
- Modify: `internal/adapters/http/integration_test.go`

- [ ] **Step 1: Write or extend failing frontend API/type tests**

```ts
it("listWorkItems returns active_profile_id and final_deliverable_id", async () => {
  const item = await api.listWorkItems({ limit: 1 });
  expect(item[0]?.active_profile_id).toBe("lead");
  expect(item[0]?.final_deliverable_id).toBe(9);
});

it("createProfile sends manager_profile_id", async () => {
  await api.createProfile({ id: "lead", manager_profile_id: "ceo" } as AgentProfile);
  expect(fetchMock.mock.calls[0]?.[1]?.body).toContain("\"manager_profile_id\":\"ceo\"");
});
```

- [ ] **Step 2: Run focused frontend tests**

Run: `pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1`

Expected: FAIL until DTOs and API clients are updated.

- [ ] **Step 3: Update DTOs, API clients, and UI labels to the new model**

```ts
export interface WorkItem {
  active_profile_id?: string;
  executor_profile_id?: string;
  reviewer_profile_id?: string;
  sponsor_profile_id?: string;
  parent_work_item_id?: number | null;
  final_deliverable_id?: number | null;
}
```

- [ ] **Step 4: Re-run frontend unit/build validation**

Run: `pwsh -NoProfile -File .\scripts\test\frontend-unit.ps1`

Run: `pwsh -NoProfile -File .\scripts\test\frontend-build.ps1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/types/api-v2/workflow.ts web/src/types/api-v2/deliverable.ts web/src/types/api-v2/agent-admin.ts web/src/lib/apiClient.workflow.ts web/src/lib/apiClient.agentAdmin.ts web/src/lib/apiClient.test.ts web/src/pages/AgentsPage.tsx web/src/pages/AgentsPage.test.tsx web/src/components/agents/CreateProfileDialog.tsx web/src/pages/chat/useChatSessionController.ts web/src/pages/MobileHomePage.tsx internal/adapters/http/workitem.go internal/adapters/http/action_signal.go internal/adapters/http/integration_test.go
git commit -m "feat(web): switch workitem views to single-kernel fields"
```

---

### Task 9: Full Cutover Verification And Regression Sweep

**Files:**
- Modify: `docs/superpowers/specs/2026-03-31-workitem-single-kernel-company-design.md`
  Update only if implementation requires clarifying the status mapping matrix or deliverable enums.
- Modify: `docs/spec/ai-company-domain-model.zh-CN.md`
  Add a short “implemented direction” note if terminology changed materially.

- [ ] **Step 1: Add the old-status mapping matrix and allowed deliverable kinds if implementation exposed ambiguity**

```markdown
| old status | new status |
| --- | --- |
| open | pending_execution |
| running | in_execution |
| done | completed |
```

- [ ] **Step 2: Run the backend regression suite**

Run: `pwsh -NoProfile -File .\scripts\test\backend-unit.ps1`

Expected: PASS.

- [ ] **Step 3: Run the backend integration suite**

Run: `pwsh -NoProfile -File .\scripts\test\backend-integration.ps1`

Expected: PASS, especially for HTTP/API and migration-sensitive paths.

- [ ] **Step 4: Run the cross-layer regression suite**

Run: `pwsh -NoProfile -File .\scripts\test\suite-p3.ps1`

Expected: PASS, or capture the remaining failures before merge.

- [ ] **Step 5: Run targeted smoke checks for CEO + work-item APIs**

Run: `go test ./internal/application/orchestrateapp ./internal/application/workitemapp ./internal/adapters/http -count=1`

Expected: PASS.

- [ ] **Step 6: Run backend E2E if the cutover changed public routes or startup backfill**

Run: `pwsh -NoProfile -File .\scripts\test\backend-e2e.ps1`

Expected: PASS, or record the exact blocking scenario before merge.

- [ ] **Step 7: Commit**

```bash
git add docs/superpowers/specs/2026-03-31-workitem-single-kernel-company-design.md docs/spec/ai-company-domain-model.zh-CN.md
git commit -m "test(plan): verify single-kernel company model rollout"
```
