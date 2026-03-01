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

func TestProjectsRequiresAuthWhenEnabled(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-auth",
		Name:     "auth-project",
		RepoPath: filepath.Join(t.TempDir(), "repo-auth"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store:       store,
		AuthEnabled: true,
		BearerToken: "secret-token",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/projects")
	if err != nil {
		t.Fatalf("GET /api/v1/projects without token: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/projects", nil)
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret-token")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/projects with token: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d", resp.StatusCode)
	}
}

func TestCreateProjectThenGetProject(t *testing.T) {
	store := newTestStore(t)
	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := map[string]any{
		"name":      "demo-project",
		"repo_path": filepath.Join(t.TempDir(), "repo-demo"),
		"github": map[string]string{
			"owner": "acme",
			"repo":  "ai-workflow",
		},
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	resp, err := http.Post(ts.URL+"/api/v1/projects", "application/json", bytes.NewReader(rawBody))
	if err != nil {
		t.Fatalf("POST /api/v1/projects: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var created core.Project
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created project: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created project id")
	}
	if created.Name != "demo-project" {
		t.Fatalf("expected name demo-project, got %s", created.Name)
	}

	getResp, err := http.Get(ts.URL + "/api/v1/projects/" + created.ID)
	if err != nil {
		t.Fatalf("GET /api/v1/projects/{id}: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getResp.StatusCode)
	}

	var got core.Project
	if err := json.NewDecoder(getResp.Body).Decode(&got); err != nil {
		t.Fatalf("decode get project: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected id %s, got %s", created.ID, got.ID)
	}
}
