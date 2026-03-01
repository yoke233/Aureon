package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestWebhook_VerifySignature_Success(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-signature-success",
		Name:        "webhook-signature-success",
		RepoPath:    filepath.Join(t.TempDir(), "repo-signature-success"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store:       store,
		AuthEnabled: true,
		BearerToken: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := readWebhookFixture(t, "github_issues_opened.json")
	resp := doWebhookRequest(t, ts, payload, "issues", signWebhookPayload("webhook-secret", payload))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d, body=%s", resp.StatusCode, string(body))
	}
}

func TestWebhook_VerifySignature_Invalid_Returns401(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-invalid-signature",
		Name:        "webhook-invalid-signature",
		RepoPath:    filepath.Join(t.TempDir(), "repo-invalid-signature"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store:       store,
		AuthEnabled: true,
		BearerToken: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := readWebhookFixture(t, "github_issues_opened.json")
	resp := doWebhookRequest(t, ts, payload, "issues", "sha256=invalid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 401, got %d, body=%s", resp.StatusCode, string(body))
	}
}

func TestWebhook_ProjectRouting_UsesOwnerRepo(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-routing",
		Name:        "webhook-routing",
		RepoPath:    filepath.Join(t.TempDir(), "repo-routing"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store:       store,
		BearerToken: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	matchedPayload := readWebhookFixture(t, "github_issues_opened.json")
	matchedResp := doWebhookRequest(t, ts, matchedPayload, "issues", signWebhookPayload("webhook-secret", matchedPayload))
	defer matchedResp.Body.Close()
	if matchedResp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(matchedResp.Body)
		t.Fatalf("expected matched payload to return 202, got %d, body=%s", matchedResp.StatusCode, string(body))
	}

	unmatchedPayload := withRepositoryOwnerRepo(t, matchedPayload, "other-org", "ai-workflow")
	unmatchedResp := doWebhookRequest(t, ts, unmatchedPayload, "issues", signWebhookPayload("webhook-secret", unmatchedPayload))
	defer unmatchedResp.Body.Close()
	if unmatchedResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(unmatchedResp.Body)
		t.Fatalf("expected unmatched owner/repo to return 404, got %d, body=%s", unmatchedResp.StatusCode, string(body))
	}
}

func TestWebhook_UnsupportedEvent_Returns202(t *testing.T) {
	store := newTestStore(t)
	project := core.Project{
		ID:          "proj-webhook-unsupported-event",
		Name:        "webhook-unsupported-event",
		RepoPath:    filepath.Join(t.TempDir(), "repo-unsupported-event"),
		GitHubOwner: "acme",
		GitHubRepo:  "ai-workflow",
	}
	if err := store.CreateProject(&project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	srv := NewServer(Config{
		Store:       store,
		BearerToken: "webhook-secret",
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	payload := readWebhookFixture(t, "github_issue_comment_created.json")
	resp := doWebhookRequest(t, ts, payload, "pull_request_review", signWebhookPayload("webhook-secret", payload))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202 for unsupported event, got %d, body=%s", resp.StatusCode, string(body))
	}
}

func doWebhookRequest(t *testing.T, ts *httptest.Server, payload []byte, event, signature string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/webhook", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create webhook request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-Hub-Signature-256", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send webhook request: %v", err)
	}
	return resp
}

func readWebhookFixture(t *testing.T, name string) []byte {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return content
}

func signWebhookPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func withRepositoryOwnerRepo(t *testing.T, payload []byte, owner, repo string) []byte {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	repositoryRaw, ok := body["repository"]
	if !ok {
		t.Fatal("payload does not contain repository field")
	}
	repository, ok := repositoryRaw.(map[string]any)
	if !ok {
		t.Fatal("payload repository field has unexpected shape")
	}

	ownerRaw, ok := repository["owner"]
	if !ok {
		t.Fatal("payload repository does not contain owner field")
	}
	repositoryOwner, ok := ownerRaw.(map[string]any)
	if !ok {
		t.Fatal("payload repository owner field has unexpected shape")
	}

	repositoryOwner["login"] = owner
	repository["name"] = repo

	updated, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal updated payload: %v", err)
	}
	return updated
}
