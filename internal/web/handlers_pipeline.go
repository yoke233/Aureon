package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/user/ai-workflow/internal/core"
	"github.com/user/ai-workflow/internal/engine"
)

type pipelineHandlers struct {
	store core.Store
}

type createPipelineRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Template    string         `json:"template"`
	Config      map[string]any `json:"config"`
}

type pipelineListResponse struct {
	Items  []core.Pipeline `json:"items"`
	Total  int             `json:"total"`
	Offset int             `json:"offset"`
}

func registerPipelineRoutes(r chi.Router, store core.Store) {
	h := &pipelineHandlers{store: store}
	r.Get("/pipelines/{id}", h.getPipelineByID)
	r.Get("/projects/{projectID}/pipelines", h.listPipelines)
	r.Post("/projects/{projectID}/pipelines", h.createPipeline)
	r.Get("/projects/{projectID}/pipelines/{id}", h.getPipelineByProject)
}

func (h *pipelineHandlers) listPipelines(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	if projectID == "" {
		writeAPIError(w, http.StatusBadRequest, "project id is required", "PROJECT_ID_REQUIRED")
		return
	}

	if _, err := h.store.GetProject(projectID); err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("project %s not found", projectID), "PROJECT_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load project", "GET_PROJECT_FAILED")
		return
	}

	limit, offset, err := parsePaginationParams(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), "INVALID_QUERY_PARAM")
		return
	}

	items, err := h.store.ListPipelines(projectID, core.PipelineFilter{
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to list pipelines", "LIST_PIPELINES_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, pipelineListResponse{
		Items:  items,
		Total:  len(items),
		Offset: offset,
	})
}

func (h *pipelineHandlers) createPipeline(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	if projectID == "" {
		writeAPIError(w, http.StatusBadRequest, "project id is required", "PROJECT_ID_REQUIRED")
		return
	}
	if _, err := h.store.GetProject(projectID); err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("project %s not found", projectID), "PROJECT_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load project", "GET_PROJECT_FAILED")
		return
	}

	var req createPipelineRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid json body", "INVALID_JSON")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Template = strings.TrimSpace(req.Template)
	if req.Name == "" {
		writeAPIError(w, http.StatusBadRequest, "name is required", "NAME_REQUIRED")
		return
	}
	if req.Template == "" {
		req.Template = "standard"
	}
	stages, err := buildPipelineStages(req.Template)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), "INVALID_TEMPLATE")
		return
	}
	if req.Config == nil {
		req.Config = map[string]any{}
	}

	now := time.Now()
	pipeline := &core.Pipeline{
		ID:              engine.NewPipelineID(),
		ProjectID:       projectID,
		Name:            req.Name,
		Description:     req.Description,
		Template:        req.Template,
		Status:          core.StatusCreated,
		Stages:          stages,
		Artifacts:       map[string]string{},
		Config:          req.Config,
		MaxTotalRetries: 5,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := h.store.SavePipeline(pipeline); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to create pipeline", "CREATE_PIPELINE_FAILED")
		return
	}

	created, err := h.store.GetPipeline(pipeline.ID)
	if err != nil && !isNotFoundError(err) {
		writeAPIError(w, http.StatusInternalServerError, "pipeline created but reload failed", "PIPELINE_RELOAD_FAILED")
		return
	}
	if created == nil {
		created = pipeline
	}

	writeJSON(w, http.StatusCreated, created)
}

func (h *pipelineHandlers) getPipelineByProject(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" || id == "" {
		writeAPIError(w, http.StatusBadRequest, "project id and pipeline id are required", "INVALID_PATH_PARAM")
		return
	}

	pipeline, err := h.store.GetPipeline(id)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("pipeline %s not found", id), "PIPELINE_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load pipeline", "GET_PIPELINE_FAILED")
		return
	}
	if pipeline.ProjectID != projectID {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("pipeline %s not found in project %s", id, projectID), "PIPELINE_NOT_FOUND")
		return
	}
	writeJSON(w, http.StatusOK, pipeline)
}

func (h *pipelineHandlers) getPipelineByID(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "pipeline id is required", "PIPELINE_ID_REQUIRED")
		return
	}

	pipeline, err := h.store.GetPipeline(id)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("pipeline %s not found", id), "PIPELINE_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load pipeline", "GET_PIPELINE_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, pipeline)
}

func parsePaginationParams(r *http.Request) (int, int, error) {
	limit := 20
	offset := 0

	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return 0, 0, fmt.Errorf("limit must be a positive integer")
		}
		limit = parsed
	}

	if rawOffset := strings.TrimSpace(r.URL.Query().Get("offset")); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			return 0, 0, fmt.Errorf("offset must be a non-negative integer")
		}
		offset = parsed
	}

	return limit, offset, nil
}

func buildPipelineStages(template string) ([]core.StageConfig, error) {
	stageIDs, ok := engine.Templates[template]
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", template)
	}

	stages := make([]core.StageConfig, len(stageIDs))
	for i, stageID := range stageIDs {
		stages[i] = defaultPipelineStageConfig(stageID)
	}
	return stages, nil
}

func defaultPipelineStageConfig(id core.StageID) core.StageConfig {
	cfg := core.StageConfig{
		Name:           id,
		PromptTemplate: string(id),
		Timeout:        30 * time.Minute,
		MaxRetries:     1,
		OnFailure:      core.OnFailureHuman,
	}

	switch id {
	case core.StageRequirements, core.StageSpecGen, core.StageSpecReview, core.StageCodeReview:
		cfg.Agent = "claude"
	case core.StageImplement, core.StageFixup:
		cfg.Agent = "codex"
	case core.StageWorktreeSetup, core.StageMerge, core.StageCleanup:
		cfg.Timeout = 2 * time.Minute
	}
	return cfg
}
