package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yoke233/ai-workflow/internal/v2/core"
)

// createStepRequest is the request body for POST /flows/{flowID}/steps.
type createStepRequest struct {
	Name                 string         `json:"name"`
	Type                 core.StepType  `json:"type"`
	DependsOn            []int64        `json:"depends_on,omitempty"`
	AgentRole            string         `json:"agent_role,omitempty"`
	RequiredCapabilities []string       `json:"required_capabilities,omitempty"`
	AcceptanceCriteria   []string       `json:"acceptance_criteria,omitempty"`
	Timeout              string         `json:"timeout,omitempty"` // Go duration string
	MaxRetries           int            `json:"max_retries"`
	Config               map[string]any `json:"config,omitempty"`
}

func (h *Handler) createStep(w http.ResponseWriter, r *http.Request) {
	flowID, ok := urlParamInt64(r, "flowID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid flow ID", "BAD_ID")
		return
	}

	var req createStepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "MISSING_NAME")
		return
	}
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required", "MISSING_TYPE")
		return
	}

	var timeout time.Duration
	if req.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(req.Timeout)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid timeout duration", "BAD_TIMEOUT")
			return
		}
	}

	s := &core.Step{
		FlowID:               flowID,
		Name:                 req.Name,
		Type:                 req.Type,
		Status:               core.StepPending,
		DependsOn:            req.DependsOn,
		AgentRole:            req.AgentRole,
		RequiredCapabilities: req.RequiredCapabilities,
		AcceptanceCriteria:   req.AcceptanceCriteria,
		Timeout:              timeout,
		MaxRetries:           req.MaxRetries,
		Config:               req.Config,
	}
	id, err := h.store.CreateStep(r.Context(), s)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	s.ID = id
	writeJSON(w, http.StatusCreated, s)
}

func (h *Handler) listSteps(w http.ResponseWriter, r *http.Request) {
	flowID, ok := urlParamInt64(r, "flowID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid flow ID", "BAD_ID")
		return
	}

	steps, err := h.store.ListStepsByFlow(r.Context(), flowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if steps == nil {
		steps = []*core.Step{}
	}
	writeJSON(w, http.StatusOK, steps)
}

func (h *Handler) getStep(w http.ResponseWriter, r *http.Request) {
	id, ok := urlParamInt64(r, "stepID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid step ID", "BAD_ID")
		return
	}

	s, err := h.store.GetStep(r.Context(), id)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "step not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, s)
}
