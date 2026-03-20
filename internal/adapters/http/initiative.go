package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/zhanggui/internal/application/initiativeapp"
	"github.com/yoke233/zhanggui/internal/core"
)

type createInitiativeRequest struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	CreatedBy   string         `json:"created_by"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type updateInitiativeRequest struct {
	Title       *string        `json:"title,omitempty"`
	Description *string        `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type initiativeItemRequest struct {
	WorkItemID int64  `json:"work_item_id"`
	Role       string `json:"role,omitempty"`
}

type approveInitiativeRequest struct {
	ApprovedBy string `json:"approved_by"`
}

type rejectInitiativeRequest struct {
	ReviewNote string `json:"review_note"`
}

type linkInitiativeThreadRequest struct {
	ThreadID     int64  `json:"thread_id"`
	RelationType string `json:"relation_type,omitempty"`
}

func registerInitiativeRoutes(r chi.Router, h *Handler) {
	r.Post("/initiatives", h.createInitiative)
	r.Get("/initiatives", h.listInitiatives)
	r.Get("/initiatives/{initiativeID}", h.getInitiative)
	r.Put("/initiatives/{initiativeID}", h.updateInitiative)
	r.Delete("/initiatives/{initiativeID}", h.deleteInitiative)
	r.Post("/initiatives/{initiativeID}/items", h.addInitiativeItem)
	r.Put("/initiatives/{initiativeID}/items/{workItemID}", h.updateInitiativeItem)
	r.Delete("/initiatives/{initiativeID}/items/{workItemID}", h.deleteInitiativeItem)
	r.Post("/initiatives/{initiativeID}/propose", h.proposeInitiative)
	r.Post("/initiatives/{initiativeID}/approve", h.approveInitiative)
	r.Post("/initiatives/{initiativeID}/reject", h.rejectInitiative)
	r.Post("/initiatives/{initiativeID}/cancel", h.cancelInitiative)
	r.Get("/initiatives/{initiativeID}/progress", h.getInitiativeProgress)
	r.Post("/initiatives/{initiativeID}/threads", h.linkInitiativeThread)
	r.Get("/initiatives/{initiativeID}/threads", h.listInitiativeThreads)
	r.Delete("/initiatives/{initiativeID}/threads/{threadID}", h.deleteInitiativeThread)
}

func (h *Handler) createInitiative(w http.ResponseWriter, r *http.Request) {
	var req createInitiativeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	initiative, err := h.initiativeService().CreateInitiative(r.Context(), initiativeapp.CreateInitiativeInput{
		Title:       req.Title,
		Description: req.Description,
		CreatedBy:   req.CreatedBy,
		Metadata:    req.Metadata,
	})
	if err != nil {
		writeInitiativeAppFailure(w, err, "CREATE_INITIATIVE_FAILED")
		return
	}
	writeJSON(w, http.StatusCreated, initiative)
}

func (h *Handler) listInitiatives(w http.ResponseWriter, r *http.Request) {
	filter := core.InitiativeFilter{}
	if status := r.URL.Query().Get("status"); status != "" {
		parsed := core.InitiativeStatus(status)
		filter.Status = &parsed
	}
	items, err := h.initiativeService().ListInitiatives(r.Context(), filter)
	if err != nil {
		writeInitiativeAppFailure(w, err, "LIST_INITIATIVES_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) getInitiative(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	detail, err := h.initiativeService().GetInitiativeDetail(r.Context(), initiativeID)
	if err != nil {
		writeInitiativeAppFailure(w, err, "GET_INITIATIVE_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) updateInitiative(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	var req updateInitiativeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	initiative, err := h.initiativeService().UpdateInitiative(r.Context(), initiativeapp.UpdateInitiativeInput{
		ID:          initiativeID,
		Title:       req.Title,
		Description: req.Description,
		Metadata:    req.Metadata,
	})
	if err != nil {
		writeInitiativeAppFailure(w, err, "UPDATE_INITIATIVE_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, initiative)
}

func (h *Handler) deleteInitiative(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	if err := h.initiativeService().DeleteInitiative(r.Context(), initiativeID); err != nil {
		writeInitiativeAppFailure(w, err, "DELETE_INITIATIVE_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) addInitiativeItem(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	var req initiativeItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	item, err := h.initiativeService().AddWorkItem(r.Context(), initiativeapp.AddInitiativeItemInput{
		InitiativeID: initiativeID,
		WorkItemID:   req.WorkItemID,
		Role:         req.Role,
	})
	if err != nil {
		writeInitiativeAppFailure(w, err, "ADD_INITIATIVE_ITEM_FAILED")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) updateInitiativeItem(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	workItemID, ok := urlParamInt64(r, "workItemID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid work item ID", "BAD_ID")
		return
	}
	var req initiativeItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	item, err := h.initiativeService().UpdateWorkItemRole(r.Context(), initiativeapp.UpdateInitiativeItemInput{
		InitiativeID: initiativeID,
		WorkItemID:   workItemID,
		Role:         req.Role,
	})
	if err != nil {
		writeInitiativeAppFailure(w, err, "UPDATE_INITIATIVE_ITEM_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) deleteInitiativeItem(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	workItemID, ok := urlParamInt64(r, "workItemID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid work item ID", "BAD_ID")
		return
	}
	if err := h.initiativeService().RemoveWorkItem(r.Context(), initiativeID, workItemID); err != nil {
		writeInitiativeAppFailure(w, err, "DELETE_INITIATIVE_ITEM_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) proposeInitiative(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	initiative, err := h.initiativeService().Propose(r.Context(), initiativeID)
	if err != nil {
		writeInitiativeAppFailure(w, err, "PROPOSE_INITIATIVE_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, initiative)
}

func (h *Handler) approveInitiative(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	var req approveInitiativeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	initiative, err := h.initiativeService().Approve(r.Context(), initiativeID, req.ApprovedBy)
	if err != nil {
		writeInitiativeAppFailure(w, err, "APPROVE_INITIATIVE_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, initiative)
}

func (h *Handler) rejectInitiative(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	var req rejectInitiativeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	initiative, err := h.initiativeService().Reject(r.Context(), initiativeID, req.ReviewNote)
	if err != nil {
		writeInitiativeAppFailure(w, err, "REJECT_INITIATIVE_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, initiative)
}

func (h *Handler) cancelInitiative(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	initiative, err := h.initiativeService().Cancel(r.Context(), initiativeID)
	if err != nil {
		writeInitiativeAppFailure(w, err, "CANCEL_INITIATIVE_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, initiative)
}

func (h *Handler) getInitiativeProgress(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	progress, err := h.initiativeService().GetProgress(r.Context(), initiativeID)
	if err != nil {
		writeInitiativeAppFailure(w, err, "GET_INITIATIVE_PROGRESS_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, progress)
}

func (h *Handler) linkInitiativeThread(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	var req linkInitiativeThreadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	link, err := h.initiativeService().LinkThread(r.Context(), initiativeapp.LinkThreadInput{
		InitiativeID: initiativeID,
		ThreadID:     req.ThreadID,
		RelationType: req.RelationType,
	})
	if err != nil {
		writeInitiativeAppFailure(w, err, "LINK_INITIATIVE_THREAD_FAILED")
		return
	}
	writeJSON(w, http.StatusCreated, link)
}

func (h *Handler) listInitiativeThreads(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	links, err := h.initiativeService().ListThreads(r.Context(), initiativeID)
	if err != nil {
		writeInitiativeAppFailure(w, err, "LIST_INITIATIVE_THREADS_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, links)
}

func (h *Handler) deleteInitiativeThread(w http.ResponseWriter, r *http.Request) {
	initiativeID, ok := urlParamInt64(r, "initiativeID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid initiative ID", "BAD_ID")
		return
	}
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}
	if err := h.initiativeService().UnlinkThread(r.Context(), initiativeID, threadID); err != nil {
		writeInitiativeAppFailure(w, err, "DELETE_INITIATIVE_THREAD_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
