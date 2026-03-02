package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestTaskActionUpdatesTaskStatus(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-task-api",
		Name:     "task-api",
		RepoPath: filepath.Join(t.TempDir(), "repo-task-api"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	plan := &core.TaskPlan{
		ID:         "plan-20260301-taskapi",
		ProjectID:  project.ID,
		Name:       "task-plan",
		Status:     core.PlanExecuting,
		WaitReason: core.WaitNone,
		FailPolicy: core.FailBlock,
	}
	if err := store.CreateTaskPlan(plan); err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	task := &core.TaskItem{
		ID:          "task-taskapi-1",
		PlanID:      plan.ID,
		Title:       "实现 OAuth 回调",
		Description: "实现 OAuth 回调并补齐状态处理",
		Status:      core.ItemPending,
	}
	if err := store.CreateTaskItem(task); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	callTaskAction := func(action string) string {
		t.Helper()
		rawBody, err := json.Marshal(map[string]any{"action": action})
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		resp, err := http.Post(
			ts.URL+"/api/v1/projects/proj-task-api/plans/"+plan.ID+"/tasks/"+task.ID+"/action",
			"application/json",
			bytes.NewReader(rawBody),
		)
		if err != nil {
			t.Fatalf("POST /api/v1/projects/{pid}/plans/{id}/tasks/{tid}/action: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var out struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode task action response: %v", err)
		}
		return out.Status
	}

	if got := callTaskAction("skip"); got != string(core.ItemSkipped) {
		t.Fatalf("skip status = %s, want %s", got, core.ItemSkipped)
	}

	loaded, err := store.GetTaskItem(task.ID)
	if err != nil {
		t.Fatalf("reload task before retry: %v", err)
	}
	loaded.Status = core.ItemFailed
	if err := store.SaveTaskItem(loaded); err != nil {
		t.Fatalf("set task failed before retry: %v", err)
	}

	if got := callTaskAction("retry"); got != string(core.ItemReady) {
		t.Fatalf("retry status = %s, want %s", got, core.ItemReady)
	}

	loaded, err = store.GetTaskItem(task.ID)
	if err != nil {
		t.Fatalf("reload task before abort: %v", err)
	}
	loaded.Status = core.ItemRunning
	if err := store.SaveTaskItem(loaded); err != nil {
		t.Fatalf("set task running before abort: %v", err)
	}

	if got := callTaskAction("abort"); got != string(core.ItemFailed) {
		t.Fatalf("abort status = %s, want %s", got, core.ItemFailed)
	}

	updated, err := store.GetTaskItem(task.ID)
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if updated.Status != core.ItemFailed {
		t.Fatalf("expected persisted status failed, got %s", updated.Status)
	}
}

func TestTaskActionRequiresActionField(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-task-required",
		Name:     "task-required",
		RepoPath: filepath.Join(t.TempDir(), "repo-task-required"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	plan := &core.TaskPlan{
		ID:         "plan-20260301-taskrequired",
		ProjectID:  project.ID,
		Name:       "task-required-plan",
		Status:     core.PlanExecuting,
		WaitReason: core.WaitNone,
		FailPolicy: core.FailBlock,
	}
	if err := store.CreateTaskPlan(plan); err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	task := &core.TaskItem{
		ID:          "task-taskrequired-1",
		PlanID:      plan.ID,
		Title:       "补齐 token 测试",
		Description: "补齐 token 测试和用例断言",
		Status:      core.ItemPending,
	}
	if err := store.CreateTaskItem(task); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	rawBody, err := json.Marshal(map[string]any{"action": "   "})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	resp, err := http.Post(
		ts.URL+"/api/v1/projects/proj-task-required/plans/"+plan.ID+"/tasks/"+task.ID+"/action",
		"application/json",
		bytes.NewReader(rawBody),
	)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/{pid}/plans/{id}/tasks/{tid}/action: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var apiErr apiError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode api error: %v", err)
	}
	if apiErr.Code != "ACTION_REQUIRED" {
		t.Fatalf("expected code ACTION_REQUIRED, got %s", apiErr.Code)
	}
}

func TestTaskActionRejectsInvalidSourceStatus(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-task-invalid-status",
		Name:     "task-invalid-status",
		RepoPath: filepath.Join(t.TempDir(), "repo-task-invalid-status"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	plan := &core.TaskPlan{
		ID:         "plan-20260301-taskinvalid",
		ProjectID:  project.ID,
		Name:       "task-invalid-plan",
		Status:     core.PlanExecuting,
		WaitReason: core.WaitNone,
		FailPolicy: core.FailBlock,
	}
	if err := store.CreateTaskPlan(plan); err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	task := &core.TaskItem{
		ID:          "task-taskinvalid-1",
		PlanID:      plan.ID,
		Title:       "已完成任务",
		Description: "用于校验非法状态动作",
		Status:      core.ItemDone,
	}
	if err := store.CreateTaskItem(task); err != nil {
		t.Fatalf("seed task: %v", err)
	}

	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(
		ts.URL+"/api/v1/projects/proj-task-invalid-status/plans/"+plan.ID+"/tasks/"+task.ID+"/action",
		"application/json",
		bytes.NewReader([]byte(`{"action":"retry"}`)),
	)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/{pid}/plans/{id}/tasks/{tid}/action: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}

	var apiErr apiError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode api error: %v", err)
	}
	if apiErr.Code != "TASK_STATUS_INVALID" {
		t.Fatalf("expected TASK_STATUS_INVALID, got %s", apiErr.Code)
	}
}
