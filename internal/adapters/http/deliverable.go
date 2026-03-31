package api

import (
	"encoding/json"
	"net/http"

	"github.com/yoke233/zhanggui/internal/core"
)

type adoptFinalDeliverableRequest struct {
	DeliverableID int64 `json:"deliverable_id"`
}

func (h *Handler) listWorkItemDeliverables(w http.ResponseWriter, r *http.Request) {
	workItemID, ok := urlParamInt64(r, "workItemID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid work item ID", "BAD_ID")
		return
	}

	items, err := h.workItemService().ListDeliverables(r.Context(), workItemID)
	if err != nil {
		if writeWorkItemAppError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if items == nil {
		items = []*core.Deliverable{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) adoptWorkItemFinalDeliverable(w http.ResponseWriter, r *http.Request) {
	workItemID, ok := urlParamInt64(r, "workItemID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid work item ID", "BAD_ID")
		return
	}

	var req adoptFinalDeliverableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if req.DeliverableID <= 0 {
		writeError(w, http.StatusBadRequest, "deliverable_id is required", "MISSING_DELIVERABLE_ID")
		return
	}

	workItem, err := h.workItemService().AdoptDeliverable(r.Context(), workItemID, req.DeliverableID)
	if err != nil {
		if writeWorkItemAppError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, workItem)
}

func (h *Handler) listThreadDeliverables(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	if _, err := h.store.GetThread(r.Context(), threadID); err != nil {
		if err == core.ErrNotFound {
			writeError(w, http.StatusNotFound, "thread not found", "THREAD_NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	items, err := h.store.ListDeliverablesByThread(r.Context(), threadID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if items == nil {
		items = []*core.Deliverable{}
	}
	writeJSON(w, http.StatusOK, items)
}
