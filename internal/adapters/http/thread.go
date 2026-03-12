package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/core"
)

type createThreadRequest struct {
	Title    string         `json:"title"`
	OwnerID  string         `json:"owner_id,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type updateThreadRequest struct {
	Title    *string        `json:"title,omitempty"`
	Status   *string        `json:"status,omitempty"`
	OwnerID  *string        `json:"owner_id,omitempty"`
	Summary  *string        `json:"summary,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type createThreadMessageRequest struct {
	SenderID string         `json:"sender_id"`
	Role     string         `json:"role,omitempty"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type addThreadParticipantRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role,omitempty"`
}

// registerThreadRoutes mounts thread endpoints onto the given router.
func registerThreadRoutes(r chi.Router, h *Handler) {
	r.Post("/threads", h.createThread)
	r.Get("/threads", h.listThreads)
	r.Get("/threads/{threadID}", h.getThread)
	r.Put("/threads/{threadID}", h.updateThread)
	r.Delete("/threads/{threadID}", h.deleteThread)

	r.Post("/threads/{threadID}/messages", h.createThreadMessage)
	r.Get("/threads/{threadID}/messages", h.listThreadMessages)

	r.Post("/threads/{threadID}/participants", h.addThreadParticipant)
	r.Get("/threads/{threadID}/participants", h.listThreadParticipants)
	r.Delete("/threads/{threadID}/participants/{userID}", h.removeThreadParticipant)
}

func (h *Handler) createThread(w http.ResponseWriter, r *http.Request) {
	var req createThreadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "title is required", "MISSING_TITLE")
		return
	}

	thread := &core.Thread{
		Title:    title,
		Status:   core.ThreadActive,
		OwnerID:  strings.TrimSpace(req.OwnerID),
		Metadata: req.Metadata,
	}

	id, err := h.store.CreateThread(r.Context(), thread)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "CREATE_THREAD_FAILED")
		return
	}
	thread.ID = id
	writeJSON(w, http.StatusCreated, thread)
}

func (h *Handler) listThreads(w http.ResponseWriter, r *http.Request) {
	filter := core.ThreadFilter{
		Limit:  queryInt(r, "limit", 50),
		Offset: queryInt(r, "offset", 0),
	}
	if s := r.URL.Query().Get("status"); s != "" {
		st := core.ThreadStatus(s)
		filter.Status = &st
	}

	threads, err := h.store.ListThreads(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if threads == nil {
		threads = []*core.Thread{}
	}
	writeJSON(w, http.StatusOK, threads)
}

func (h *Handler) getThread(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	thread, err := h.store.GetThread(r.Context(), threadID)
	if err != nil {
		if err == core.ErrNotFound {
			writeError(w, http.StatusNotFound, "thread not found", "THREAD_NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, thread)
}

func (h *Handler) updateThread(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	thread, err := h.store.GetThread(r.Context(), threadID)
	if err != nil {
		if err == core.ErrNotFound {
			writeError(w, http.StatusNotFound, "thread not found", "THREAD_NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	var req updateThreadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}

	if req.Title != nil {
		thread.Title = strings.TrimSpace(*req.Title)
	}
	if req.Status != nil {
		thread.Status = core.ThreadStatus(strings.TrimSpace(*req.Status))
	}
	if req.OwnerID != nil {
		thread.OwnerID = strings.TrimSpace(*req.OwnerID)
	}
	if req.Summary != nil {
		thread.Summary = strings.TrimSpace(*req.Summary)
	}
	if req.Metadata != nil {
		thread.Metadata = req.Metadata
	}

	if err := h.store.UpdateThread(r.Context(), thread); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "UPDATE_THREAD_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, thread)
}

func (h *Handler) deleteThread(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	if err := h.store.DeleteThread(r.Context(), threadID); err != nil {
		if err == core.ErrNotFound {
			writeError(w, http.StatusNotFound, "thread not found", "THREAD_NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "DELETE_THREAD_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---------------------------------------------------------------------------
// Thread Messages
// ---------------------------------------------------------------------------

func (h *Handler) createThreadMessage(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	// Verify thread exists.
	if _, err := h.store.GetThread(r.Context(), threadID); err != nil {
		if err == core.ErrNotFound {
			writeError(w, http.StatusNotFound, "thread not found", "THREAD_NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	var req createThreadMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required", "MISSING_CONTENT")
		return
	}

	msg := &core.ThreadMessage{
		ThreadID: threadID,
		SenderID: strings.TrimSpace(req.SenderID),
		Role:     req.Role,
		Content:  req.Content,
		Metadata: req.Metadata,
	}

	id, err := h.store.CreateThreadMessage(r.Context(), msg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "CREATE_MESSAGE_FAILED")
		return
	}
	msg.ID = id
	writeJSON(w, http.StatusCreated, msg)
}

func (h *Handler) listThreadMessages(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	msgs, err := h.store.ListThreadMessages(r.Context(), threadID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if msgs == nil {
		msgs = []*core.ThreadMessage{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

// ---------------------------------------------------------------------------
// Thread Participants
// ---------------------------------------------------------------------------

func (h *Handler) addThreadParticipant(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	// Verify thread exists.
	if _, err := h.store.GetThread(r.Context(), threadID); err != nil {
		if err == core.ErrNotFound {
			writeError(w, http.StatusNotFound, "thread not found", "THREAD_NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	var req addThreadParticipantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "BAD_REQUEST")
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		writeError(w, http.StatusBadRequest, "user_id is required", "MISSING_USER_ID")
		return
	}

	p := &core.ThreadParticipant{
		ThreadID: threadID,
		UserID:   strings.TrimSpace(req.UserID),
		Role:     req.Role,
	}

	id, err := h.store.AddThreadParticipant(r.Context(), p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "ADD_PARTICIPANT_FAILED")
		return
	}
	p.ID = id
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) listThreadParticipants(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	participants, err := h.store.ListThreadParticipants(r.Context(), threadID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if participants == nil {
		participants = []*core.ThreadParticipant{}
	}
	writeJSON(w, http.StatusOK, participants)
}

func (h *Handler) removeThreadParticipant(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	userID := strings.TrimSpace(chi.URLParam(r, "userID"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required", "MISSING_USER_ID")
		return
	}

	if err := h.store.RemoveThreadParticipant(r.Context(), threadID, userID); err != nil {
		if err == core.ErrNotFound {
			writeError(w, http.StatusNotFound, "participant not found", "PARTICIPANT_NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "REMOVE_PARTICIPANT_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
