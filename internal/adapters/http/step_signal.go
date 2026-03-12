package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/yoke233/ai-workflow/internal/core"
)

// stepDecisionRequest is the request body for POST /steps/{stepID}/decision.
type stepDecisionRequest struct {
	Decision      string  `json:"decision"`                 // approve | reject | complete | need_help
	Reason        string  `json:"reason"`                   // required
	RejectTargets []int64 `json:"reject_targets,omitempty"` // for reject only
}

func (h *Handler) stepDecision(w http.ResponseWriter, r *http.Request) {
	stepID, ok := urlParamInt64(r, "stepID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid step ID", "BAD_ID")
		return
	}

	var req stepDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		writeError(w, http.StatusBadRequest, "reason is required", "MISSING_REASON")
		return
	}

	var sigType core.SignalType
	switch strings.ToLower(strings.TrimSpace(req.Decision)) {
	case "approve":
		sigType = core.SignalApprove
	case "reject":
		sigType = core.SignalReject
	case "complete":
		sigType = core.SignalComplete
	case "need_help":
		sigType = core.SignalNeedHelp
	default:
		writeError(w, http.StatusBadRequest, "decision must be one of: approve, reject, complete, need_help", "INVALID_DECISION")
		return
	}

	step, err := h.store.GetStep(r.Context(), stepID)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "step not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	// Only allow decisions on running or blocked steps.
	if step.Status != core.StepRunning && step.Status != core.StepBlocked {
		writeError(w, http.StatusConflict, "step is not in a decidable state", "INVALID_STATE")
		return
	}

	payload := map[string]any{"reason": req.Reason}
	if sigType == core.SignalReject && len(req.RejectTargets) > 0 {
		targets := make([]any, len(req.RejectTargets))
		for i, t := range req.RejectTargets {
			targets[i] = t
		}
		payload["reject_targets"] = targets
	}

	sig := &core.StepSignal{
		StepID:    stepID,
		IssueID:   step.IssueID,
		Type:      sigType,
		Source:    core.SignalSourceHuman,
		Payload:   payload,
		Actor:     "human",
		CreatedAt: time.Now().UTC(),
	}
	id, err := h.store.CreateStepSignal(r.Context(), sig)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	sig.ID = id

	// Publish event for engine to pick up.
	h.bus.Publish(r.Context(), core.Event{
		Type:      core.EventStepSignal,
		IssueID:   step.IssueID,
		StepID:    stepID,
		Timestamp: time.Now().UTC(),
		Data:      map[string]any{"signal_id": id, "type": string(sigType), "source": "human"},
	})

	writeJSON(w, http.StatusCreated, sig)
}

// stepUnblockRequest is the request body for POST /steps/{stepID}/unblock.
type stepUnblockRequest struct {
	Reason string `json:"reason"` // required
}

func (h *Handler) stepUnblock(w http.ResponseWriter, r *http.Request) {
	stepID, ok := urlParamInt64(r, "stepID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid step ID", "BAD_ID")
		return
	}

	var req stepUnblockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		writeError(w, http.StatusBadRequest, "reason is required", "MISSING_REASON")
		return
	}

	step, err := h.store.GetStep(r.Context(), stepID)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "step not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if step.Status != core.StepBlocked {
		writeError(w, http.StatusConflict, "step is not blocked", "INVALID_STATE")
		return
	}

	// Create unblock signal.
	sig := &core.StepSignal{
		StepID:    stepID,
		IssueID:   step.IssueID,
		Type:      core.SignalUnblock,
		Source:    core.SignalSourceHuman,
		Payload:   map[string]any{"reason": req.Reason},
		Actor:     "human",
		CreatedAt: time.Now().UTC(),
	}
	sigID, err := h.store.CreateStepSignal(r.Context(), sig)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	sig.ID = sigID

	// Transition step back to pending for retry.
	step.Status = core.StepPending
	if err := h.store.UpdateStep(r.Context(), step); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	h.bus.Publish(r.Context(), core.Event{
		Type:      core.EventStepUnblocked,
		IssueID:   step.IssueID,
		StepID:    stepID,
		Timestamp: time.Now().UTC(),
		Data:      map[string]any{"signal_id": sigID, "reason": req.Reason},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "unblocked",
		"signal": sig,
		"step":   step,
	})
}

func (h *Handler) listPendingDecisions(w http.ResponseWriter, r *http.Request) {
	issueID, hasIssue := queryInt64(r, "issue_id")

	var steps []*core.Step
	var err error
	if hasIssue {
		steps, err = h.store.ListPendingHumanSteps(r.Context(), issueID)
	} else {
		steps, err = h.store.ListAllPendingHumanSteps(r.Context())
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if steps == nil {
		steps = []*core.Step{}
	}
	writeJSON(w, http.StatusOK, steps)
}

func (h *Handler) listStepSignals(w http.ResponseWriter, r *http.Request) {
	stepID, ok := urlParamInt64(r, "stepID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid step ID", "BAD_ID")
		return
	}
	signals, err := h.store.ListStepSignals(r.Context(), stepID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if signals == nil {
		signals = []*core.StepSignal{}
	}
	writeJSON(w, http.StatusOK, signals)
}
