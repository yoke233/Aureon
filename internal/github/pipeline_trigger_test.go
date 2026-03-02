package github

import (
	"context"
	"testing"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
	storesqlite "github.com/yoke233/ai-workflow/internal/plugins/store-sqlite"
)

func TestPipelineTrigger_LabelMapping_SelectsTemplate(t *testing.T) {
	store := newPipelineTriggerTestStore(t)
	defer store.Close()
	projectID := seedPipelineTriggerProject(t, store)

	createCalls := 0
	trigger := NewPipelineTrigger(store, func(projectID, name, description, template string) (*core.Pipeline, error) {
		createCalls++
		return &core.Pipeline{
			ID:              "pipe-trigger-1",
			ProjectID:       projectID,
			Name:            name,
			Description:     description,
			Template:        template,
			Status:          core.StatusCreated,
			Stages:          []core.StageConfig{},
			Artifacts:       map[string]string{},
			Config:          map[string]any{},
			MaxTotalRetries: 5,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}, nil
	})

	pipeline, err := trigger.TriggerFromIssue(context.Background(), IssueTriggerInput{
		ProjectID:            projectID,
		IssueNumber:          201,
		IssueTitle:           "issue trigger",
		IssueBody:            "from label mapping",
		Labels:               []string{"type:feature"},
		LabelTemplateMapping: map[string]string{"type:feature": "feature"},
	})
	if err != nil {
		t.Fatalf("TriggerFromIssue() error = %v", err)
	}
	if pipeline.Template != "feature" {
		t.Fatalf("expected template feature, got %q", pipeline.Template)
	}
	if createCalls != 1 {
		t.Fatalf("expected create called once, got %d", createCalls)
	}
}

func TestPipelineTrigger_Idempotent_NoDuplicatePipelineForSameIssue(t *testing.T) {
	store := newPipelineTriggerTestStore(t)
	defer store.Close()
	projectID := seedPipelineTriggerProject(t, store)

	existing := &core.Pipeline{
		ID:              "pipe-existing",
		ProjectID:       projectID,
		Name:            "existing",
		Description:     "existing",
		Template:        "standard",
		Status:          core.StatusCreated,
		Stages:          []core.StageConfig{},
		Artifacts:       map[string]string{"issue_number": "202"},
		Config:          map[string]any{"issue_number": 202},
		MaxTotalRetries: 5,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := store.SavePipeline(existing); err != nil {
		t.Fatalf("SavePipeline(existing) error = %v", err)
	}

	createCalls := 0
	trigger := NewPipelineTrigger(store, func(projectID, name, description, template string) (*core.Pipeline, error) {
		createCalls++
		return &core.Pipeline{
			ID:              "pipe-new",
			ProjectID:       projectID,
			Name:            name,
			Description:     description,
			Template:        template,
			Status:          core.StatusCreated,
			Stages:          []core.StageConfig{},
			Artifacts:       map[string]string{},
			Config:          map[string]any{},
			MaxTotalRetries: 5,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}, nil
	})

	pipeline, err := trigger.TriggerFromIssue(context.Background(), IssueTriggerInput{
		ProjectID:   projectID,
		IssueNumber: 202,
		IssueTitle:  "same issue",
	})
	if err != nil {
		t.Fatalf("TriggerFromIssue() error = %v", err)
	}
	if pipeline.ID != "pipe-existing" {
		t.Fatalf("expected existing pipeline, got %q", pipeline.ID)
	}
	if createCalls != 0 {
		t.Fatalf("expected create not called, got %d", createCalls)
	}
}

func TestPipelineTrigger_CommandRun_UsesExplicitTemplate(t *testing.T) {
	store := newPipelineTriggerTestStore(t)
	defer store.Close()
	projectID := seedPipelineTriggerProject(t, store)

	trigger := NewPipelineTrigger(store, func(projectID, name, description, template string) (*core.Pipeline, error) {
		return &core.Pipeline{
			ID:              "pipe-command-1",
			ProjectID:       projectID,
			Name:            name,
			Description:     description,
			Template:        template,
			Status:          core.StatusCreated,
			Stages:          []core.StageConfig{},
			Artifacts:       map[string]string{},
			Config:          map[string]any{},
			MaxTotalRetries: 5,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}, nil
	})

	pipeline, err := trigger.TriggerFromCommand(context.Background(), CommandTriggerInput{
		ProjectID:   projectID,
		IssueNumber: 203,
		Template:    "hotfix",
		Message:     "/run hotfix",
	})
	if err != nil {
		t.Fatalf("TriggerFromCommand() error = %v", err)
	}
	if pipeline.Template != "hotfix" {
		t.Fatalf("expected template hotfix, got %q", pipeline.Template)
	}
}

func newPipelineTriggerTestStore(t *testing.T) *storesqlite.SQLiteStore {
	t.Helper()
	store, err := storesqlite.New(":memory:")
	if err != nil {
		t.Fatalf("create sqlite store: %v", err)
	}
	return store
}

func seedPipelineTriggerProject(t *testing.T, store core.Store) string {
	t.Helper()
	project := &core.Project{
		ID:       "proj-pipeline-trigger",
		Name:     "proj-pipeline-trigger",
		RepoPath: t.TempDir(),
	}
	if err := store.CreateProject(project); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	return project.ID
}
