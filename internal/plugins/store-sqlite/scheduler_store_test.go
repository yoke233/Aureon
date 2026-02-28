package storesqlite

import (
	"testing"
	"time"

	"github.com/user/ai-workflow/internal/core"
)

func TestSchedulerListRunnablePipelinesFIFO(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	project := &core.Project{ID: "proj-a", Name: "A", RepoPath: t.TempDir()}
	if err := s.CreateProject(project); err != nil {
		t.Fatal(err)
	}

	base := time.Now().Add(-1 * time.Hour)
	pipelines := []*core.Pipeline{
		{
			ID:        "pipe-1",
			ProjectID: project.ID,
			Name:      "one",
			Template:  "quick",
			Status:    core.StatusCreated,
			QueuedAt:  base.Add(1 * time.Minute),
			Stages:    []core.StageConfig{{Name: core.StageImplement, Agent: "codex"}},
		},
		{
			ID:        "pipe-2",
			ProjectID: project.ID,
			Name:      "two",
			Template:  "quick",
			Status:    core.StatusCreated,
			QueuedAt:  base.Add(2 * time.Minute),
			Stages:    []core.StageConfig{{Name: core.StageImplement, Agent: "codex"}},
		},
		{
			ID:        "pipe-3",
			ProjectID: project.ID,
			Name:      "three",
			Template:  "quick",
			Status:    core.StatusRunning,
			QueuedAt:  base.Add(3 * time.Minute),
			Stages:    []core.StageConfig{{Name: core.StageImplement, Agent: "codex"}},
		},
	}
	for _, p := range pipelines {
		p.CreatedAt = time.Now()
		p.UpdatedAt = time.Now()
		if err := s.SavePipeline(p); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.ListRunnablePipelines(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 runnable pipelines, got %d", len(got))
	}
	if got[0].ID != "pipe-1" || got[1].ID != "pipe-2" {
		t.Fatalf("expected FIFO order [pipe-1, pipe-2], got [%s, %s]", got[0].ID, got[1].ID)
	}
}

func TestSchedulerCountRunningByProject(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	projectA := &core.Project{ID: "proj-a", Name: "A", RepoPath: t.TempDir()}
	projectB := &core.Project{ID: "proj-b", Name: "B", RepoPath: t.TempDir()}
	if err := s.CreateProject(projectA); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProject(projectB); err != nil {
		t.Fatal(err)
	}

	savePipeline := func(id, projectID string, status core.PipelineStatus) {
		t.Helper()
		p := &core.Pipeline{
			ID:        id,
			ProjectID: projectID,
			Name:      id,
			Template:  "quick",
			Status:    status,
			Stages:    []core.StageConfig{{Name: core.StageImplement, Agent: "codex"}},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.SavePipeline(p); err != nil {
			t.Fatal(err)
		}
	}

	savePipeline("a-running-1", projectA.ID, core.StatusRunning)
	savePipeline("a-running-2", projectA.ID, core.StatusRunning)
	savePipeline("a-created", projectA.ID, core.StatusCreated)
	savePipeline("b-running-1", projectB.ID, core.StatusRunning)

	countA, err := s.CountRunningPipelinesByProject(projectA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if countA != 2 {
		t.Fatalf("expected project A running count=2, got %d", countA)
	}

	countB, err := s.CountRunningPipelinesByProject(projectB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if countB != 1 {
		t.Fatalf("expected project B running count=1, got %d", countB)
	}
}

func TestSchedulerTryMarkRunningCAS(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	project := &core.Project{ID: "proj-a", Name: "A", RepoPath: t.TempDir()}
	if err := s.CreateProject(project); err != nil {
		t.Fatal(err)
	}

	p := &core.Pipeline{
		ID:        "pipe-1",
		ProjectID: project.ID,
		Name:      "one",
		Template:  "quick",
		Status:    core.StatusCreated,
		Stages:    []core.StageConfig{{Name: core.StageImplement, Agent: "codex"}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.SavePipeline(p); err != nil {
		t.Fatal(err)
	}

	ok, err := s.TryMarkPipelineRunning(p.ID, core.StatusCreated)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected first CAS mark to succeed")
	}

	ok, err = s.TryMarkPipelineRunning(p.ID, core.StatusCreated)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected second CAS mark to fail when status already running")
	}

	got, err := s.GetPipeline(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != core.StatusRunning {
		t.Fatalf("expected running status, got %s", got.Status)
	}
	if got.RunCount != 1 {
		t.Fatalf("expected run_count=1, got %d", got.RunCount)
	}
}
