package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/user/ai-workflow/internal/core"
)

type projectHandlers struct {
	store core.Store
}

type createProjectRequest struct {
	Name     string `json:"name"`
	RepoPath string `json:"repo_path"`
	GitHub   struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	} `json:"github"`
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

func registerProjectRoutes(r chi.Router, store core.Store) {
	h := &projectHandlers{store: store}
	r.Get("/projects", h.listProjects)
	r.Post("/projects", h.createProject)
	r.Get("/projects/{id}", h.getProject)
}

func (h *projectHandlers) listProjects(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	filter := core.ProjectFilter{
		NameContains: strings.TrimSpace(r.URL.Query().Get("q")),
	}
	items, err := h.store.ListProjects(filter)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to list projects", "LIST_PROJECTS_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *projectHandlers) getProject(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "project id is required", "PROJECT_ID_REQUIRED")
		return
	}

	project, err := h.store.GetProject(id)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("project %s not found", id), "PROJECT_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load project", "GET_PROJECT_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (h *projectHandlers) createProject(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	var req createProjectRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid json body", "INVALID_JSON")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.RepoPath = strings.TrimSpace(req.RepoPath)
	req.GitHub.Owner = strings.TrimSpace(req.GitHub.Owner)
	req.GitHub.Repo = strings.TrimSpace(req.GitHub.Repo)
	if req.Name == "" {
		writeAPIError(w, http.StatusBadRequest, "name is required", "NAME_REQUIRED")
		return
	}
	if req.RepoPath == "" {
		writeAPIError(w, http.StatusBadRequest, "repo_path is required", "REPO_PATH_REQUIRED")
		return
	}

	project := &core.Project{
		ID:          uuid.NewString(),
		Name:        req.Name,
		RepoPath:    req.RepoPath,
		GitHubOwner: req.GitHub.Owner,
		GitHubRepo:  req.GitHub.Repo,
	}

	if err := h.store.CreateProject(project); err != nil {
		if isConflictError(err) {
			writeAPIError(w, http.StatusConflict, "project already exists", "PROJECT_ALREADY_EXISTS")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to create project", "CREATE_PROJECT_FAILED")
		return
	}

	created, err := h.store.GetProject(project.ID)
	if err != nil && !isNotFoundError(err) {
		writeAPIError(w, http.StatusInternalServerError, "project created but reload failed", "PROJECT_RELOAD_FAILED")
		return
	}
	if created == nil {
		created = project
	}

	writeJSON(w, http.StatusCreated, created)
}

func writeAPIError(w http.ResponseWriter, statusCode int, message, code string) {
	writeJSON(w, statusCode, apiError{
		Error: message,
		Code:  code,
	})
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var target interface{ NotFound() bool }
	if errors.As(err, &target) {
		return target.NotFound()
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint")
}
