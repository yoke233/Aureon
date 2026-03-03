package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func TestPlanRoutesCreateListGetAndDAG(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-plan-api",
		Name:     "plan-api",
		RepoPath: filepath.Join(t.TempDir(), "repo-plan-api"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	session := &core.ChatSession{
		ID:        "chat-20260301-planapi01",
		ProjectID: project.ID,
		Messages: []core.ChatMessage{
			{Role: "user", Content: "split oauth flow into issues"},
		},
	}
	if err := store.CreateChatSession(session); err != nil {
		t.Fatalf("seed chat session: %v", err)
	}

	createCalls := 0
	manager := &testPlanManager{
		createIssuesFn: func(_ context.Context, input IssueCreateInput) ([]core.Issue, error) {
			createCalls++
			issue := core.Issue{
				ID:         core.NewIssueID(),
				ProjectID:  input.ProjectID,
				SessionID:  input.SessionID,
				Title:      strings.TrimSpace(input.Name),
				Body:       "oauth breakdown",
				Template:   "standard",
				State:      core.IssueStateOpen,
				Status:     core.IssueStatusDraft,
				FailPolicy: input.FailPolicy,
			}
			if issue.Title == "" {
				issue.Title = issue.ID
			}
			if issue.FailPolicy == "" {
				issue.FailPolicy = core.FailBlock
			}
			if err := store.CreateIssue(&issue); err != nil {
				return nil, err
			}
			loaded, err := store.GetIssue(issue.ID)
			if err != nil {
				return nil, err
			}
			return []core.Issue{*loaded}, nil
		},
	}

	srv := NewServer(Config{Store: store, PlanManager: manager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	createResp := doIssuePost(t, ts, "/api/v1/projects/proj-plan-api/plans", map[string]any{
		"session_id": session.ID,
		"name":       "oauth-plan",
	})
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	var created struct {
		Issue core.Issue   `json:"issue"`
		Items []core.Issue `json:"items"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created issue: %v", err)
	}
	if created.Issue.ID == "" {
		t.Fatal("expected non-empty issue id")
	}
	if created.Issue.Status != core.IssueStatusDraft {
		t.Fatalf("expected status draft, got %s", created.Issue.Status)
	}
	if len(created.Items) != 1 || created.Items[0].ID != created.Issue.ID {
		t.Fatalf("unexpected create response items: %#v", created.Items)
	}
	if createCalls != 1 {
		t.Fatalf("expected CreateIssues called once, got %d", createCalls)
	}

	dependent := core.Issue{
		ID:         "issue-planapi-2",
		ProjectID:  project.ID,
		SessionID:  session.ID,
		Title:      "add auth state tests",
		Body:       "cover token refresh path",
		Template:   "standard",
		State:      core.IssueStateOpen,
		Status:     core.IssueStatusExecuting,
		DependsOn:  []string{created.Issue.ID},
		FailPolicy: core.FailBlock,
	}
	mustCreateIssue(t, store, dependent)

	listResp, err := http.Get(ts.URL + "/api/v1/projects/proj-plan-api/plans?status=draft&limit=10&offset=0")
	if err != nil {
		t.Fatalf("GET /api/v1/projects/{pid}/plans: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", listResp.StatusCode)
	}

	var listed issueListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listed.Total != 1 {
		t.Fatalf("expected total=1, got %d", listed.Total)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != created.Issue.ID {
		t.Fatalf("unexpected listed items: %#v", listed.Items)
	}

	getResp, err := http.Get(ts.URL + "/api/v1/projects/proj-plan-api/plans/" + created.Issue.ID)
	if err != nil {
		t.Fatalf("GET /api/v1/projects/{pid}/plans/{id}: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getResp.StatusCode)
	}

	var got core.Issue
	if err := json.NewDecoder(getResp.Body).Decode(&got); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if got.ID != created.Issue.ID {
		t.Fatalf("expected issue id %s, got %s", created.Issue.ID, got.ID)
	}

	dagResp, err := http.Get(ts.URL + "/api/v1/projects/proj-plan-api/plans/" + created.Issue.ID + "/dag")
	if err != nil {
		t.Fatalf("GET /api/v1/projects/{pid}/plans/{id}/dag: %v", err)
	}
	defer dagResp.Body.Close()
	if dagResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", dagResp.StatusCode)
	}

	var dag issueDAGResponse
	if err := json.NewDecoder(dagResp.Body).Decode(&dag); err != nil {
		t.Fatalf("decode dag response: %v", err)
	}
	if len(dag.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(dag.Nodes))
	}
	if len(dag.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(dag.Edges))
	}
	if dag.Edges[0].From != created.Issue.ID || dag.Edges[0].To != dependent.ID {
		t.Fatalf("unexpected edge: %#v", dag.Edges[0])
	}
	if dag.Stats.Total != 2 || dag.Stats.Pending != 1 || dag.Stats.Running != 1 {
		t.Fatalf("unexpected dag stats: %#v", dag.Stats)
	}
}

func TestPlanCreateUsesConfiguredIssueParserRole(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-plan-role-api",
		Name:     "plan-role-api",
		RepoPath: filepath.Join(t.TempDir(), "repo-plan-role-api"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	session := &core.ChatSession{
		ID:        "chat-20260302-planrole01",
		ProjectID: project.ID,
		Messages: []core.ChatMessage{
			{Role: "user", Content: "generate issues"},
		},
	}
	if err := store.CreateChatSession(session); err != nil {
		t.Fatalf("seed chat session: %v", err)
	}

	gotRole := ""
	manager := &testPlanManager{
		createIssuesFn: func(_ context.Context, input IssueCreateInput) ([]core.Issue, error) {
			gotRole = strings.TrimSpace(input.Request.Role)
			issue := core.Issue{
				ID:         "issue-20260302-role",
				ProjectID:  input.ProjectID,
				SessionID:  input.SessionID,
				Title:      "role-issue",
				Template:   "standard",
				State:      core.IssueStateOpen,
				Status:     core.IssueStatusDraft,
				FailPolicy: core.FailBlock,
			}
			if err := store.CreateIssue(&issue); err != nil {
				return nil, err
			}
			return []core.Issue{issue}, nil
		},
	}

	srv := NewServer(Config{
		Store:            store,
		PlanManager:      manager,
		PlanParserRoleID: "plan_parser_custom",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doIssuePost(t, ts, "/api/v1/projects/proj-plan-role-api/plans", map[string]any{
		"session_id": session.ID,
		"name":       "role-plan",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if gotRole != "plan_parser_custom" {
		t.Fatalf("expected role %q, got %q", "plan_parser_custom", gotRole)
	}
}

func TestPlanCreateFromFilesPassesSourceFilesAndReviewInput(t *testing.T) {
	store := newTestStore(t)

	repoRoot := filepath.Join(t.TempDir(), "repo-plan-from-files")
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "docs", "plan.md"), []byte("oauth notes"), 0o644); err != nil {
		t.Fatalf("write docs/plan.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("readme notes"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	project := core.Project{
		ID:       "proj-plan-from-files",
		Name:     "plan-from-files",
		RepoPath: repoRoot,
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	session := &core.ChatSession{
		ID:        "chat-20260302-planfiles01",
		ProjectID: project.ID,
		Messages: []core.ChatMessage{
			{Role: "user", Content: "extract issues from docs"},
		},
	}
	if err := store.CreateChatSession(session); err != nil {
		t.Fatalf("seed chat session: %v", err)
	}

	createCalls := 0
	submitCalls := 0
	var capturedCreateInput IssueCreateInput
	var capturedReviewInput IssueReviewInput

	manager := &testPlanManager{
		createIssuesFn: func(_ context.Context, input IssueCreateInput) ([]core.Issue, error) {
			createCalls++
			capturedCreateInput = input
			issue := core.Issue{
				ID:         "issue-20260302-fromfiles",
				ProjectID:  input.ProjectID,
				SessionID:  input.SessionID,
				Title:      "from-files-issue",
				Template:   "standard",
				State:      core.IssueStateOpen,
				Status:     core.IssueStatusDraft,
				FailPolicy: input.FailPolicy,
			}
			if err := store.CreateIssue(&issue); err != nil {
				return nil, err
			}
			return []core.Issue{issue}, nil
		},
		submitForReviewFn: func(_ context.Context, issueID string, input IssueReviewInput) (*core.Issue, error) {
			submitCalls++
			capturedReviewInput = input
			issue, err := store.GetIssue(issueID)
			if err != nil {
				return nil, err
			}
			issue.Status = core.IssueStatusReviewing
			if err := store.SaveIssue(issue); err != nil {
				return nil, err
			}
			return store.GetIssue(issueID)
		},
	}

	srv := NewServer(Config{Store: store, PlanManager: manager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doIssuePost(t, ts, "/api/v1/projects/proj-plan-from-files/plans/from-files", map[string]any{
		"session_id": session.ID,
		"name":       "from-files-plan",
		"file_paths": []string{"docs/plan.md", "README.md"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var created struct {
		Issue        core.Issue        `json:"issue"`
		Items        []core.Issue      `json:"items"`
		SourceFiles  []string          `json:"source_files"`
		FileContents map[string]string `json:"file_contents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create-from-files response: %v", err)
	}
	if created.Issue.ID == "" {
		t.Fatal("expected non-empty issue id")
	}
	if created.Issue.Status != core.IssueStatusReviewing {
		t.Fatalf("expected issue status reviewing, got %s", created.Issue.Status)
	}
	if createCalls != 1 {
		t.Fatalf("expected CreateIssues called once, got %d", createCalls)
	}
	if submitCalls != 1 {
		t.Fatalf("expected SubmitForReview called once, got %d", submitCalls)
	}

	wantSourceFiles := []string{"docs/plan.md", "README.md"}
	wantFileContents := map[string]string{
		"docs/plan.md": "oauth notes",
		"README.md":    "readme notes",
	}
	if !reflect.DeepEqual(capturedCreateInput.SourceFiles, wantSourceFiles) {
		t.Fatalf("unexpected create input source files: %#v", capturedCreateInput.SourceFiles)
	}
	if !reflect.DeepEqual(capturedCreateInput.FileContents, wantFileContents) {
		t.Fatalf("unexpected create input file contents: %#v", capturedCreateInput.FileContents)
	}
	if !reflect.DeepEqual(created.SourceFiles, wantSourceFiles) {
		t.Fatalf("unexpected response source files: %#v", created.SourceFiles)
	}
	if !reflect.DeepEqual(created.FileContents, wantFileContents) {
		t.Fatalf("unexpected response file contents: %#v", created.FileContents)
	}
	if !reflect.DeepEqual(capturedReviewInput.FileContents, wantFileContents) {
		t.Fatalf("unexpected review input file contents: %#v", capturedReviewInput.FileContents)
	}
	if !strings.Contains(capturedReviewInput.Conversation, "extract issues from docs") {
		t.Fatalf("unexpected review conversation: %q", capturedReviewInput.Conversation)
	}
	if !strings.Contains(capturedReviewInput.ProjectContext, "project=plan-from-files") {
		t.Fatalf("unexpected review project context: %q", capturedReviewInput.ProjectContext)
	}
}

func TestPlanCreateFromFilesValidationReturnsBadRequest(t *testing.T) {
	store := newTestStore(t)
	repoRoot := filepath.Join(t.TempDir(), "repo-plan-from-files-bad")
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "docs", "ok.md"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write docs/ok.md: %v", err)
	}

	project := core.Project{
		ID:       "proj-plan-from-files-bad",
		Name:     "plan-from-files-bad",
		RepoPath: repoRoot,
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	session := &core.ChatSession{
		ID:        "chat-20260302-planfiles02",
		ProjectID: project.ID,
		Messages: []core.ChatMessage{
			{Role: "user", Content: "bad request test"},
		},
	}
	if err := store.CreateChatSession(session); err != nil {
		t.Fatalf("seed chat session: %v", err)
	}

	createCalls := 0
	manager := &testPlanManager{
		createIssuesFn: func(_ context.Context, _ IssueCreateInput) ([]core.Issue, error) {
			createCalls++
			return nil, errors.New("should not be called")
		},
		submitForReviewFn: func(_ context.Context, _ string, _ IssueReviewInput) (*core.Issue, error) {
			t.Fatal("SubmitForReview should not be called")
			return nil, nil
		},
	}

	srv := NewServer(Config{Store: store, PlanManager: manager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	cases := []struct {
		name string
		body map[string]any
	}{
		{
			name: "missing_file_paths",
			body: map[string]any{"session_id": session.ID},
		},
		{
			name: "empty_path",
			body: map[string]any{
				"session_id": session.ID,
				"file_paths": []string{""},
			},
		},
		{
			name: "path_traversal",
			body: map[string]any{
				"session_id": session.ID,
				"file_paths": []string{"../secret.md"},
			},
		},
		{
			name: "file_not_found",
			body: map[string]any{
				"session_id": session.ID,
				"file_paths": []string{"missing.md"},
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			resp := doIssuePost(t, ts, "/api/v1/projects/proj-plan-from-files-bad/plans/from-files", tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
		})
	}

	if createCalls != 0 {
		t.Fatalf("expected CreateIssues not called, got %d", createCalls)
	}
}

func TestPlanReviewDelegatesToIssueManager(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-review-api",
		Name:     "review-api",
		RepoPath: filepath.Join(t.TempDir(), "repo-review-api"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	session := &core.ChatSession{
		ID:        "chat-20260302-review01",
		ProjectID: project.ID,
		Messages: []core.ChatMessage{
			{Role: "user", Content: "need review"},
		},
	}
	if err := store.CreateChatSession(session); err != nil {
		t.Fatalf("seed chat session: %v", err)
	}
	issue := mustCreateIssue(t, store, core.Issue{
		ID:         "issue-20260302-reviewapi",
		ProjectID:  project.ID,
		SessionID:  session.ID,
		Title:      "review-issue",
		Template:   "standard",
		State:      core.IssueStateOpen,
		Status:     core.IssueStatusDraft,
		FailPolicy: core.FailBlock,
	})

	submitCalls := 0
	capturedIssueID := ""
	capturedInput := IssueReviewInput{}
	manager := &testPlanManager{
		submitForReviewFn: func(_ context.Context, issueID string, input IssueReviewInput) (*core.Issue, error) {
			submitCalls++
			capturedIssueID = issueID
			capturedInput = input
			loaded, err := store.GetIssue(issueID)
			if err != nil {
				return nil, err
			}
			loaded.Status = core.IssueStatusReviewing
			if err := store.SaveIssue(loaded); err != nil {
				return nil, err
			}
			return store.GetIssue(issueID)
		},
	}

	srv := NewServer(Config{Store: store, PlanManager: manager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(
		ts.URL+"/api/v1/projects/proj-review-api/plans/"+issue.ID+"/review",
		"application/json",
		bytes.NewReader([]byte("{}")),
	)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/{pid}/plans/{id}/review: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload issueStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode review response: %v", err)
	}
	if payload.Status != string(core.IssueStatusReviewing) {
		t.Fatalf("expected status %q, got %q", core.IssueStatusReviewing, payload.Status)
	}
	if submitCalls != 1 {
		t.Fatalf("expected SubmitForReview called once, got %d", submitCalls)
	}
	if capturedIssueID != issue.ID {
		t.Fatalf("expected issue id %q, got %q", issue.ID, capturedIssueID)
	}
	if !strings.Contains(capturedInput.Conversation, "need review") {
		t.Fatalf("unexpected conversation: %q", capturedInput.Conversation)
	}
	if !strings.Contains(capturedInput.ProjectContext, "project=review-api") {
		t.Fatalf("unexpected project context: %q", capturedInput.ProjectContext)
	}
}

func TestPlanActionRejectRequiresFeedbackAndDelegates(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-action-api",
		Name:     "action-api",
		RepoPath: filepath.Join(t.TempDir(), "repo-action-api"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	issue := mustCreateIssue(t, store, core.Issue{
		ID:         "issue-20260302-actionapi",
		ProjectID:  project.ID,
		Title:      "action-issue",
		Template:   "standard",
		State:      core.IssueStateOpen,
		Status:     core.IssueStatusReviewing,
		FailPolicy: core.FailBlock,
	})

	applyCalls := 0
	capturedAction := IssueAction{}
	manager := &testPlanManager{
		applyIssueActionFn: func(_ context.Context, issueID string, action IssueAction) (*core.Issue, error) {
			applyCalls++
			capturedAction = action
			loaded, err := store.GetIssue(issueID)
			if err != nil {
				return nil, err
			}
			loaded.Status = core.IssueStatusDraft
			if err := store.SaveIssue(loaded); err != nil {
				return nil, err
			}
			return store.GetIssue(issueID)
		},
	}

	srv := NewServer(Config{Store: store, PlanManager: manager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	invalidResp := doIssuePost(t, ts, "/api/v1/projects/proj-action-api/plans/"+issue.ID+"/action", map[string]any{
		"action": "reject",
	})
	invalidResp.Body.Close()
	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing feedback, got %d", invalidResp.StatusCode)
	}

	shortResp := doIssuePost(t, ts, "/api/v1/projects/proj-action-api/plans/"+issue.ID+"/action", map[string]any{
		"action": "reject",
		"feedback": map[string]any{
			"category": "coverage_gap",
			"detail":   "too short",
		},
	})
	shortResp.Body.Close()
	if shortResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for short feedback detail, got %d", shortResp.StatusCode)
	}

	validResp := doIssuePost(t, ts, "/api/v1/projects/proj-action-api/plans/"+issue.ID+"/action", map[string]any{
		"action": "reject",
		"feedback": map[string]any{
			"category":           "coverage_gap",
			"detail":             "please cover dependency branch and rollback behavior",
			"expected_direction": "split issue by auth state",
		},
	})
	defer validResp.Body.Close()
	if validResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for valid reject action, got %d", validResp.StatusCode)
	}
	if applyCalls != 1 {
		t.Fatalf("expected ApplyIssueAction called once, got %d", applyCalls)
	}
	if capturedAction.Action != "reject" {
		t.Fatalf("expected action reject, got %q", capturedAction.Action)
	}
	if capturedAction.Feedback == nil {
		t.Fatal("expected reject feedback to be forwarded")
	}
	if capturedAction.Feedback.Category != "coverage_gap" {
		t.Fatalf("unexpected feedback category: %q", capturedAction.Feedback.Category)
	}
}

func TestPlanActionApproveStatusConflictMapsTo409(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-action-conflict",
		Name:     "action-conflict",
		RepoPath: filepath.Join(t.TempDir(), "repo-action-conflict"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	issue := mustCreateIssue(t, store, core.Issue{
		ID:         "issue-20260302-conflict",
		ProjectID:  project.ID,
		Title:      "conflict-issue",
		Template:   "standard",
		State:      core.IssueStateOpen,
		Status:     core.IssueStatusDraft,
		FailPolicy: core.FailBlock,
	})

	manager := &testPlanManager{
		applyIssueActionFn: func(_ context.Context, _ string, _ IssueAction) (*core.Issue, error) {
			return nil, errors.New("approve requires reviewing status")
		},
	}

	srv := NewServer(Config{Store: store, PlanManager: manager})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp := doIssuePost(t, ts, "/api/v1/projects/proj-action-conflict/plans/"+issue.ID+"/action", map[string]any{
		"action": "approve",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestPlanListReturnsTotalWithPagination(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:       "proj-plan-total",
		Name:     "plan-total",
		RepoPath: filepath.Join(t.TempDir(), "repo-plan-total"),
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	mustCreateIssue(t, store, core.Issue{
		ID:         "issue-20260302-total01",
		ProjectID:  project.ID,
		Title:      "total-1",
		Template:   "standard",
		State:      core.IssueStateOpen,
		Status:     core.IssueStatusDraft,
		FailPolicy: core.FailBlock,
	})
	mustCreateIssue(t, store, core.Issue{
		ID:         "issue-20260302-total02",
		ProjectID:  project.ID,
		Title:      "total-2",
		Template:   "standard",
		State:      core.IssueStateOpen,
		Status:     core.IssueStatusDraft,
		FailPolicy: core.FailBlock,
	})

	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/projects/proj-plan-total/plans?status=draft&limit=1&offset=0")
	if err != nil {
		t.Fatalf("GET /api/v1/projects/{pid}/plans: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var listed issueListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listed.Total != 2 {
		t.Fatalf("expected total=2, got %d", listed.Total)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("expected one item due to limit=1, got %d", len(listed.Items))
	}
}

func doIssuePost(t *testing.T, ts *httptest.Server, path string, body map[string]any) *http.Response {
	t.Helper()
	rawBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	resp, err := http.Post(ts.URL+path, "application/json", bytes.NewReader(rawBody))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func mustCreateIssue(t *testing.T, store core.Store, issue core.Issue) core.Issue {
	t.Helper()
	if strings.TrimSpace(issue.ID) == "" {
		issue.ID = core.NewIssueID()
	}
	if strings.TrimSpace(issue.Template) == "" {
		issue.Template = "standard"
	}
	if strings.TrimSpace(string(issue.State)) == "" {
		issue.State = core.IssueStateOpen
	}
	if strings.TrimSpace(string(issue.Status)) == "" {
		issue.Status = core.IssueStatusDraft
	}
	if strings.TrimSpace(string(issue.FailPolicy)) == "" {
		issue.FailPolicy = core.FailBlock
	}
	if err := store.CreateIssue(&issue); err != nil {
		t.Fatalf("seed issue %s: %v", issue.ID, err)
	}
	loaded, err := store.GetIssue(issue.ID)
	if err != nil {
		t.Fatalf("reload issue %s: %v", issue.ID, err)
	}
	return *loaded
}

type testPlanManager struct {
	createIssuesFn     func(ctx context.Context, input IssueCreateInput) ([]core.Issue, error)
	createDraftFn      func(ctx context.Context, input IssueCreateInput) ([]core.Issue, error)
	submitForReviewFn  func(ctx context.Context, issueID string, input IssueReviewInput) (*core.Issue, error)
	submitReviewFn     func(ctx context.Context, issueID string, input IssueReviewInput) (*core.Issue, error)
	applyIssueActionFn func(ctx context.Context, issueID string, action IssueAction) (*core.Issue, error)
	applyActionFn      func(ctx context.Context, issueID string, action IssueAction) (*core.Issue, error)
}

func (m *testPlanManager) CreateIssues(ctx context.Context, input IssueCreateInput) ([]core.Issue, error) {
	switch {
	case m.createIssuesFn != nil:
		return m.createIssuesFn(ctx, input)
	case m.createDraftFn != nil:
		return m.createDraftFn(ctx, input)
	default:
		return nil, errors.New("create issues not implemented")
	}
}

func (m *testPlanManager) SubmitForReview(ctx context.Context, issueID string, input IssueReviewInput) (*core.Issue, error) {
	switch {
	case m.submitForReviewFn != nil:
		return m.submitForReviewFn(ctx, issueID, input)
	case m.submitReviewFn != nil:
		return m.submitReviewFn(ctx, issueID, input)
	default:
		return nil, errors.New("submit for review not implemented")
	}
}

func (m *testPlanManager) ApplyIssueAction(ctx context.Context, issueID string, action IssueAction) (*core.Issue, error) {
	switch {
	case m.applyIssueActionFn != nil:
		return m.applyIssueActionFn(ctx, issueID, action)
	case m.applyActionFn != nil:
		return m.applyActionFn(ctx, issueID, action)
	default:
		return nil, errors.New("apply issue action not implemented")
	}
}
