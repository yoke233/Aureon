package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/user/ai-workflow/internal/core"
)

type taskHandlers struct {
	store core.Store
}

type taskActionRequest struct {
	Action string `json:"action"`
}

func registerTaskRoutes(r chi.Router, store core.Store) {
	h := &taskHandlers{store: store}
	r.Post("/projects/{projectID}/plans/{id}/tasks/{taskID}/action", h.applyAction)
}

func (h *taskHandlers) applyAction(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	planID := strings.TrimSpace(chi.URLParam(r, "id"))
	taskID := strings.TrimSpace(chi.URLParam(r, "taskID"))
	if projectID == "" || planID == "" || taskID == "" {
		writeAPIError(w, http.StatusBadRequest, "project id, plan id and task id are required", "INVALID_PATH_PARAM")
		return
	}

	plan, err := h.store.GetTaskPlan(planID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("task plan %s not found", planID), "TASK_PLAN_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load task plan", "GET_TASK_PLAN_FAILED")
		return
	}
	if plan.ProjectID != projectID {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("task plan %s not found in project %s", planID, projectID), "TASK_PLAN_NOT_FOUND")
		return
	}

	task, err := h.store.GetTaskItem(taskID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("task item %s not found", taskID), "TASK_ITEM_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load task item", "GET_TASK_ITEM_FAILED")
		return
	}
	if task.PlanID != planID {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("task item %s not found in plan %s", taskID, planID), "TASK_ITEM_NOT_FOUND")
		return
	}

	var req taskActionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid json body", "INVALID_JSON")
		return
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		writeAPIError(w, http.StatusBadRequest, "action is required", "ACTION_REQUIRED")
		return
	}

	switch action {
	case "retry":
		if task.Status != core.ItemFailed && task.Status != core.ItemBlockedByFailure && task.Status != core.ItemSkipped {
			writeAPIError(
				w,
				http.StatusConflict,
				fmt.Sprintf("retry requires failed/blocked_by_failure/skipped, got %s", task.Status),
				"TASK_STATUS_INVALID",
			)
			return
		}
		task.Status = core.ItemReady
	case "skip":
		if task.Status != core.ItemPending && task.Status != core.ItemReady && task.Status != core.ItemBlockedByFailure {
			writeAPIError(
				w,
				http.StatusConflict,
				fmt.Sprintf("skip requires pending/ready/blocked_by_failure, got %s", task.Status),
				"TASK_STATUS_INVALID",
			)
			return
		}
		task.Status = core.ItemSkipped
	case "abort":
		if task.Status != core.ItemPending && task.Status != core.ItemReady && task.Status != core.ItemRunning {
			writeAPIError(
				w,
				http.StatusConflict,
				fmt.Sprintf("abort requires pending/ready/running, got %s", task.Status),
				"TASK_STATUS_INVALID",
			)
			return
		}
		task.Status = core.ItemFailed
	default:
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("unsupported task action %q", action), "INVALID_ACTION")
		return
	}

	if err := h.store.SaveTaskItem(task); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to update task item", "SAVE_TASK_ITEM_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, taskPlanStatusResponse{
		Status: string(task.Status),
	})
}
