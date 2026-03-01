package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestListPipelinesInvalidLimitReturns400(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-limit",
		Name:     "project-limit",
		RepoPath: filepath.Join(t.TempDir(), "repo-limit"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/projects/proj-limit/pipelines?limit=bad")
	if err != nil {
		t.Fatalf("GET /api/v1/projects/{pid}/pipelines: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid limit, got %d", resp.StatusCode)
	}
}

func TestCreatePipelineThenGetPipelineByProjectAndGlobal(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-pipe",
		Name:     "project-pipe",
		RepoPath: filepath.Join(t.TempDir(), "repo-pipe"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	createBody := map[string]any{
		"name":        "pipeline-one",
		"description": "pipeline for api test",
		"template":    "quick",
	}
	rawBody, err := json.Marshal(createBody)
	if err != nil {
		t.Fatalf("marshal create pipeline body: %v", err)
	}

	createResp, err := http.Post(
		ts.URL+"/api/v1/projects/proj-pipe/pipelines",
		"application/json",
		bytes.NewReader(rawBody),
	)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/{pid}/pipelines: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	var created core.Pipeline
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created pipeline: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created pipeline id")
	}
	if created.ProjectID != "proj-pipe" {
		t.Fatalf("expected project_id proj-pipe, got %s", created.ProjectID)
	}

	getByProjectResp, err := http.Get(ts.URL + "/api/v1/projects/proj-pipe/pipelines/" + created.ID)
	if err != nil {
		t.Fatalf("GET /api/v1/projects/{pid}/pipelines/{id}: %v", err)
	}
	defer getByProjectResp.Body.Close()
	if getByProjectResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getByProjectResp.StatusCode)
	}

	getByGlobalResp, err := http.Get(ts.URL + "/api/v1/pipelines/" + created.ID)
	if err != nil {
		t.Fatalf("GET /api/v1/pipelines/{id}: %v", err)
	}
	defer getByGlobalResp.Body.Close()
	if getByGlobalResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getByGlobalResp.StatusCode)
	}
}
