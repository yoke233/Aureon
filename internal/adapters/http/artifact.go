package api

import (
	"net/http"

	"github.com/yoke233/zhanggui/internal/core"
)

func runToDeliverableResponse(run *core.Run, assets []*core.Resource) map[string]any {
	resp := map[string]any{
		"id":              run.ID,
		"run_id":          run.ID,
		"action_id":       run.ActionID,
		"work_item_id":    run.WorkItemID,
		"result_markdown": run.ResultMarkdown,
		"metadata":        run.ResultMetadata,
		"assets":          assets,
		"created_at":      run.CreatedAt,
	}
	if deliverable := core.RunResultToDeliverable(run); deliverable != nil {
		resp["deliverable"] = map[string]any{
			"kind":          deliverable.Kind,
			"title":         deliverable.Title,
			"summary":       deliverable.Summary,
			"payload":       deliverable.Payload,
			"producer_type": deliverable.ProducerType,
			"producer_id":   deliverable.ProducerID,
			"status":        deliverable.Status,
			"created_at":    deliverable.CreatedAt,
		}
	}
	if artifact := core.NormalizeArtifactMetadata(run.ResultMetadata); artifact != nil {
		resp["artifact"] = artifact
	}
	return resp
}

func deliverableToResponse(deliverable *core.Deliverable, run *core.Run, assets []*core.Resource) map[string]any {
	resp := map[string]any{
		"id":            deliverable.ID,
		"kind":          deliverable.Kind,
		"title":         deliverable.Title,
		"summary":       deliverable.Summary,
		"payload":       deliverable.Payload,
		"producer_type": deliverable.ProducerType,
		"producer_id":   deliverable.ProducerID,
		"status":        deliverable.Status,
		"created_at":    deliverable.CreatedAt,
		"deliverable": map[string]any{
			"id":            deliverable.ID,
			"kind":          deliverable.Kind,
			"title":         deliverable.Title,
			"summary":       deliverable.Summary,
			"payload":       deliverable.Payload,
			"producer_type": deliverable.ProducerType,
			"producer_id":   deliverable.ProducerID,
			"status":        deliverable.Status,
			"created_at":    deliverable.CreatedAt,
		},
	}
	if deliverable.WorkItemID != nil {
		resp["work_item_id"] = *deliverable.WorkItemID
	} else if run != nil {
		resp["work_item_id"] = run.WorkItemID
	}
	if deliverable.ThreadID != nil {
		resp["thread_id"] = *deliverable.ThreadID
	}
	if run != nil {
		resp["run_id"] = run.ID
		resp["action_id"] = run.ActionID
	}
	if markdown := core.DeliverablePayloadMarkdown(deliverable.Payload); markdown != "" {
		resp["result_markdown"] = markdown
	}
	if metadata := core.DeliverablePayloadMetadata(deliverable.Payload); len(metadata) > 0 {
		resp["metadata"] = metadata
	}
	if artifact := core.NormalizeDeliverableArtifact(deliverable.Payload); artifact != nil {
		resp["artifact"] = artifact
	}
	if len(assets) > 0 {
		resp["assets"] = assets
	}
	return resp
}

func (h *Handler) getDeliverable(w http.ResponseWriter, r *http.Request) {
	id, ok := urlParamInt64(r, "artifactID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid artifact ID", "BAD_ID")
		return
	}

	deliverable, err := h.store.GetDeliverable(r.Context(), id)
	switch {
	case err == nil:
		var (
			run    *core.Run
			assets []*core.Resource
		)
		if deliverable.ProducerType == core.DeliverableProducerRun {
			run, err = h.store.GetRun(r.Context(), deliverable.ProducerID)
			if err != nil && err != core.ErrNotFound {
				writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
				return
			}
			if run != nil {
				assets, err = h.store.ListResourcesByRun(r.Context(), run.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
					return
				}
			}
		}
		writeJSON(w, http.StatusOK, deliverableToResponse(deliverable, run, assets))
		return
	case err != nil && err != core.ErrNotFound:
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	// Backward-compatible fallback: treat artifact IDs as Run IDs when no stored deliverable exists.
	run, err := h.store.GetRun(r.Context(), id)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "artifact not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if !run.HasResult() {
		writeError(w, http.StatusNotFound, "artifact not found", "NOT_FOUND")
		return
	}
	assets, err := h.store.ListResourcesByRun(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	deliverables, err := h.store.ListDeliverablesByProducer(r.Context(), core.DeliverableProducerRun, run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if len(deliverables) > 0 {
		writeJSON(w, http.StatusOK, deliverableToResponse(deliverables[0], run, assets))
		return
	}
	writeJSON(w, http.StatusOK, runToDeliverableResponse(run, assets))
}

func (h *Handler) getLatestDeliverable(w http.ResponseWriter, r *http.Request) {
	actionID, ok := urlParamInt64(r, "actionID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid action ID", "BAD_ID")
		return
	}

	run, err := h.store.GetLatestRunWithResult(r.Context(), actionID)
	if err == core.ErrNotFound {
		writeError(w, http.StatusNotFound, "no artifact for this action", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	assets, err := h.store.ListResourcesByRun(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	deliverables, err := h.store.ListDeliverablesByProducer(r.Context(), core.DeliverableProducerRun, run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if len(deliverables) > 0 {
		writeJSON(w, http.StatusOK, deliverableToResponse(deliverables[0], run, assets))
		return
	}
	writeJSON(w, http.StatusOK, runToDeliverableResponse(run, assets))
}

func (h *Handler) listDeliverablesByRun(w http.ResponseWriter, r *http.Request) {
	runID, ok := urlParamInt64(r, "runID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid run ID", "BAD_ID")
		return
	}

	run, err := h.store.GetRun(r.Context(), runID)
	if err == core.ErrNotFound {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	assets, err := h.store.ListResourcesByRun(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	deliverables, err := h.store.ListDeliverablesByProducer(r.Context(), core.DeliverableProducerRun, run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if len(deliverables) > 0 {
		items := make([]map[string]any, 0, len(deliverables))
		for _, deliverable := range deliverables {
			items = append(items, deliverableToResponse(deliverable, run, assets))
		}
		writeJSON(w, http.StatusOK, items)
		return
	}

	// Return the run's inline result as a single-element array for backward compat.
	if !run.HasResult() {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, []map[string]any{runToDeliverableResponse(run, assets)})
}
