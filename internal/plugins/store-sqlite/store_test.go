package storesqlite

import (
	"reflect"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestProjectCRUD(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	p := &core.Project{ID: "test-1", Name: "Test", RepoPath: "/tmp/test"}
	if err := s.CreateProject(p); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetProject("test-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Test" {
		t.Errorf("expected Test, got %s", got.Name)
	}

	got.Name = "Updated"
	if err := s.UpdateProject(got); err != nil {
		t.Fatal(err)
	}

	got2, _ := s.GetProject("test-1")
	if got2.Name != "Updated" {
		t.Errorf("expected Updated, got %s", got2.Name)
	}

	if err := s.DeleteProject("test-1"); err != nil {
		t.Fatal(err)
	}
	_, err = s.GetProject("test-1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestPipelineSaveAndGet(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_ = s.CreateProject(&core.Project{ID: "proj-1", Name: "P", RepoPath: "/tmp/p"})

	pipe := &core.Pipeline{
		ID:        "20260228-aabbccddeeff",
		ProjectID: "proj-1",
		Name:      "test-pipe",
		Template:  "standard",
		Status:    core.StatusCreated,
		Stages:    []core.StageConfig{{Name: core.StageImplement, Agent: "claude"}},
		Artifacts: map[string]string{},

		MaxTotalRetries: 5,
	}
	if err := s.SavePipeline(pipe); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetPipeline("20260228-aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	if got.Template != "standard" {
		t.Errorf("expected standard, got %s", got.Template)
	}
}

func TestTaskPlanRoundTrip_PersistsContractMeta(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	project := &core.Project{ID: "proj-contract-meta", Name: "meta", RepoPath: t.TempDir()}
	if err := s.CreateProject(project); err != nil {
		t.Fatal(err)
	}

	plan := &core.TaskPlan{
		ID:               "plan-20260301-11223344",
		ProjectID:        project.ID,
		Name:             "contract-meta",
		Status:           core.PlanDraft,
		WaitReason:       core.WaitNone,
		FailPolicy:       core.FailBlock,
		SpecProfile:      "default",
		ContractVersion:  "v1",
		ContractChecksum: "sha256:11223344",
	}
	if err := s.SaveTaskPlan(plan); err != nil {
		t.Fatalf("save task plan: %v", err)
	}

	got, err := s.GetTaskPlan(plan.ID)
	if err != nil {
		t.Fatalf("get task plan: %v", err)
	}
	if got.SpecProfile != plan.SpecProfile || got.ContractVersion != plan.ContractVersion || got.ContractChecksum != plan.ContractChecksum {
		t.Fatalf("contract meta not persisted: got %#v", got)
	}
}

func TestTaskItemRoundTrip_PersistsInputsOutputsAcceptanceConstraints(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	project := &core.Project{ID: "proj-contract-task", Name: "task", RepoPath: t.TempDir()}
	if err := s.CreateProject(project); err != nil {
		t.Fatal(err)
	}

	plan := &core.TaskPlan{
		ID:         "plan-20260301-55667788",
		ProjectID:  project.ID,
		Name:       "contract-task",
		Status:     core.PlanDraft,
		WaitReason: core.WaitNone,
		FailPolicy: core.FailBlock,
	}
	if err := s.SaveTaskPlan(plan); err != nil {
		t.Fatalf("save task plan: %v", err)
	}

	item := &core.TaskItem{
		ID:          "task-55667788-1",
		PlanID:      plan.ID,
		Title:       "contract item",
		Description: "task with structured contract",
		Labels:      []string{"backend"},
		DependsOn:   []string{},
		Inputs:      []string{"oauth_app_id"},
		Outputs:     []string{"oauth_token"},
		Acceptance:  []string{"oauth callback returns 200"},
		Constraints: []string{"do not change existing api path"},
		Template:    "standard",
		Status:      core.ItemPending,
	}
	if err := s.CreateTaskItem(item); err != nil {
		t.Fatalf("create task item: %v", err)
	}

	got, err := s.GetTaskItem(item.ID)
	if err != nil {
		t.Fatalf("get task item: %v", err)
	}
	if !reflect.DeepEqual(got.Inputs, item.Inputs) || !reflect.DeepEqual(got.Outputs, item.Outputs) ||
		!reflect.DeepEqual(got.Acceptance, item.Acceptance) || !reflect.DeepEqual(got.Constraints, item.Constraints) {
		t.Fatalf("structured fields not persisted: got=%#v", got)
	}
}

func TestPipelineRoundTrip_PersistsTaskItemID(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	project := &core.Project{ID: "proj-pipeline-task-item", Name: "pipe", RepoPath: t.TempDir()}
	if err := s.CreateProject(project); err != nil {
		t.Fatal(err)
	}

	p := &core.Pipeline{
		ID:         "pipe-task-item-1",
		ProjectID:  project.ID,
		Name:       "pipeline-with-task",
		Template:   "standard",
		Status:     core.StatusCreated,
		TaskItemID: "task-55667788-1",
		Stages:     []core.StageConfig{{Name: core.StageImplement, Agent: "codex"}},
		Artifacts:  map[string]string{},
	}
	if err := s.SavePipeline(p); err != nil {
		t.Fatalf("save pipeline: %v", err)
	}

	got, err := s.GetPipeline(p.ID)
	if err != nil {
		t.Fatalf("get pipeline: %v", err)
	}
	if got.TaskItemID != p.TaskItemID {
		t.Fatalf("pipeline task_item_id mismatch: got=%q want=%q", got.TaskItemID, p.TaskItemID)
	}
}
