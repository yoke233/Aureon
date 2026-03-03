package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskActionRouteRemoved(t *testing.T) {
	store := newTestStore(t)
	srv := NewServer(Config{Store: store})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	rawBody := []byte(`{"action":"skip"}`)
	resp, err := http.Post(
		ts.URL+"/api/v1/projects/proj-task/plans/plan-1/tasks/task-1/action",
		"application/json",
		bytes.NewReader(rawBody),
	)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/{pid}/plans/{id}/tasks/{tid}/action: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if strings.TrimSpace(body.String()) != "404 page not found" {
		t.Fatalf("expected route-level 404 body, got %q", body.String())
	}
}
