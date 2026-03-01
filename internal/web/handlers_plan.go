package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/user/ai-workflow/internal/core"
	"github.com/user/ai-workflow/internal/secretary"
)

type planHandlers struct {
	store core.Store
}

type createPlanRequest struct {
	SessionID  string `json:"session_id"`
	Name       string `json:"name"`
	FailPolicy string `json:"fail_policy"`
}

type planListResponse struct {
	Items  []core.TaskPlan `json:"items"`
	Total  int             `json:"total"`
	Offset int             `json:"offset"`
}

type taskPlanStatusResponse struct {
	Status string `json:"status"`
}

type planDAGNode struct {
	ID         string              `json:"id"`
	Title      string              `json:"title"`
	Status     core.TaskItemStatus `json:"status"`
	PipelineID string              `json:"pipeline_id"`
}

type planDAGEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type planDAGStats struct {
	Total   int `json:"total"`
	Pending int `json:"pending"`
	Ready   int `json:"ready"`
	Running int `json:"running"`
	Done    int `json:"done"`
	Failed  int `json:"failed"`
}

type planDAGResponse struct {
	Nodes []planDAGNode `json:"nodes"`
	Edges []planDAGEdge `json:"edges"`
	Stats planDAGStats  `json:"stats"`
}

type planActionRequest struct {
	Action   string              `json:"action"`
	Feedback *planActionFeedback `json:"feedback,omitempty"`
}

type planActionFeedback struct {
	Category          string `json:"category"`
	Detail            string `json:"detail"`
	ExpectedDirection string `json:"expected_direction,omitempty"`
}

func registerPlanRoutes(r chi.Router, store core.Store) {
	h := &planHandlers{store: store}
	r.Post("/projects/{projectID}/plans", h.createPlan)
	r.Get("/projects/{projectID}/plans", h.listPlans)
	r.Get("/projects/{projectID}/plans/{id}", h.getPlan)
	r.Get("/projects/{projectID}/plans/{id}/dag", h.getPlanDAG)
	r.Post("/projects/{projectID}/plans/{id}/review", h.submitReview)
	r.Post("/projects/{projectID}/plans/{id}/action", h.applyAction)
}

func (h *planHandlers) createPlan(w http.ResponseWriter, r *http.Request) {
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

	var req createPlanRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid json body", "INVALID_JSON")
		return
	}

	req.SessionID = strings.TrimSpace(req.SessionID)
	req.Name = strings.TrimSpace(req.Name)
	req.FailPolicy = strings.ToLower(strings.TrimSpace(req.FailPolicy))
	if req.SessionID == "" {
		writeAPIError(w, http.StatusBadRequest, "session_id is required", "SESSION_ID_REQUIRED")
		return
	}

	session, err := h.store.GetChatSession(req.SessionID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("chat session %s not found", req.SessionID), "CHAT_SESSION_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load chat session", "GET_CHAT_SESSION_FAILED")
		return
	}
	if session.ProjectID != projectID {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("chat session %s not found in project %s", req.SessionID, projectID), "CHAT_SESSION_NOT_FOUND")
		return
	}

	failPolicy, err := parseFailPolicy(req.FailPolicy)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), "INVALID_FAIL_POLICY")
		return
	}

	planID := core.NewTaskPlanID()
	planName := req.Name
	if planName == "" {
		planName = planID
	}
	plan := &core.TaskPlan{
		ID:         planID,
		ProjectID:  projectID,
		SessionID:  req.SessionID,
		Name:       planName,
		Status:     core.PlanDraft,
		WaitReason: core.WaitNone,
		FailPolicy: failPolicy,
	}
	if err := h.store.CreateTaskPlan(plan); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to create task plan", "CREATE_TASK_PLAN_FAILED")
		return
	}

	created, err := h.store.GetTaskPlan(plan.ID)
	if err != nil && !isNotFoundError(err) {
		writeAPIError(w, http.StatusInternalServerError, "task plan created but reload failed", "TASK_PLAN_RELOAD_FAILED")
		return
	}
	if created == nil {
		created = plan
	}

	writeJSON(w, http.StatusCreated, created)
}

func (h *planHandlers) listPlans(w http.ResponseWriter, r *http.Request) {
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
	status := strings.TrimSpace(r.URL.Query().Get("status"))

	// Keep `total` as unpaginated count for client-side paginator semantics.
	allItems, err := h.store.ListTaskPlans(projectID, core.TaskPlanFilter{
		Status: status,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to count task plans", "COUNT_TASK_PLANS_FAILED")
		return
	}
	total := len(allItems)

	items, err := h.store.ListTaskPlans(projectID, core.TaskPlanFilter{
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to list task plans", "LIST_TASK_PLANS_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, planListResponse{
		Items:  items,
		Total:  total,
		Offset: offset,
	})
}

func (h *planHandlers) getPlan(w http.ResponseWriter, r *http.Request) {
	plan, ok := h.loadPlanForProject(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *planHandlers) getPlanDAG(w http.ResponseWriter, r *http.Request) {
	plan, ok := h.loadPlanForProject(w, r)
	if !ok {
		return
	}

	dag := secretary.Build(plan.Tasks)
	if err := dag.Validate(); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(), "INVALID_TASK_DAG")
		return
	}

	nodes := make([]planDAGNode, len(plan.Tasks))
	stats := planDAGStats{Total: len(plan.Tasks)}
	for i := range plan.Tasks {
		item := plan.Tasks[i]
		nodes[i] = planDAGNode{
			ID:         item.ID,
			Title:      item.Title,
			Status:     item.Status,
			PipelineID: item.PipelineID,
		}

		switch item.Status {
		case core.ItemPending:
			stats.Pending++
		case core.ItemReady:
			stats.Ready++
		case core.ItemRunning:
			stats.Running++
		case core.ItemDone:
			stats.Done++
		case core.ItemFailed:
			stats.Failed++
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	edges := make([]planDAGEdge, 0, len(plan.Tasks))
	for from, downstream := range dag.Downstream {
		for _, to := range downstream {
			edges = append(edges, planDAGEdge{
				From: from,
				To:   to,
			})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})

	writeJSON(w, http.StatusOK, planDAGResponse{
		Nodes: nodes,
		Edges: edges,
		Stats: stats,
	})
}

func (h *planHandlers) submitReview(w http.ResponseWriter, r *http.Request) {
	plan, ok := h.loadPlanForProject(w, r)
	if !ok {
		return
	}

	if plan.Status != core.PlanDraft && plan.Status != core.PlanReviewing {
		writeAPIError(w, http.StatusConflict, fmt.Sprintf("submit review requires draft/reviewing, got %s", plan.Status), "PLAN_STATUS_INVALID")
		return
	}

	plan.Status = core.PlanReviewing
	plan.WaitReason = core.WaitNone
	if err := h.store.SaveTaskPlan(plan); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to update task plan", "SAVE_TASK_PLAN_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, taskPlanStatusResponse{
		Status: string(plan.Status),
	})
}

func (h *planHandlers) applyAction(w http.ResponseWriter, r *http.Request) {
	plan, ok := h.loadPlanForProject(w, r)
	if !ok {
		return
	}

	var req planActionRequest
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
	case "approve":
		if plan.Status != core.PlanWaitingHuman || plan.WaitReason != core.WaitFinalApproval {
			writeAPIError(
				w,
				http.StatusConflict,
				fmt.Sprintf("approve requires waiting_human/final_approval, got %s/%s", plan.Status, plan.WaitReason),
				"PLAN_STATUS_INVALID",
			)
			return
		}
		plan.Status = core.PlanExecuting
		plan.WaitReason = core.WaitNone
	case "reject":
		if plan.Status != core.PlanWaitingHuman ||
			(plan.WaitReason != core.WaitFinalApproval && plan.WaitReason != core.WaitFeedbackReq) {
			writeAPIError(
				w,
				http.StatusConflict,
				fmt.Sprintf(
					"reject requires waiting_human/final_approval|feedback_required, got %s/%s",
					plan.Status,
					plan.WaitReason,
				),
				"PLAN_STATUS_INVALID",
			)
			return
		}
		if err := validatePlanRejectFeedback(req.Feedback); err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), feedbackErrorCode(err))
			return
		}
		plan.Status = core.PlanReviewing
		plan.WaitReason = core.WaitNone
	case "abort", "abandon":
		if plan.Status != core.PlanWaitingHuman {
			writeAPIError(
				w,
				http.StatusConflict,
				fmt.Sprintf("abandon requires waiting_human, got %s", plan.Status),
				"PLAN_STATUS_INVALID",
			)
			return
		}
		plan.Status = core.PlanAbandoned
		plan.WaitReason = core.WaitNone
	default:
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("unsupported plan action %q", action), "INVALID_ACTION")
		return
	}

	if err := h.store.SaveTaskPlan(plan); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to update task plan", "SAVE_TASK_PLAN_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, taskPlanStatusResponse{
		Status: string(plan.Status),
	})
}

func (h *planHandlers) loadPlanForProject(w http.ResponseWriter, r *http.Request) (*core.TaskPlan, bool) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return nil, false
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	planID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" || planID == "" {
		writeAPIError(w, http.StatusBadRequest, "project id and plan id are required", "INVALID_PATH_PARAM")
		return nil, false
	}

	plan, err := h.store.GetTaskPlan(planID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("task plan %s not found", planID), "TASK_PLAN_NOT_FOUND")
			return nil, false
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load task plan", "GET_TASK_PLAN_FAILED")
		return nil, false
	}
	if plan.ProjectID != projectID {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("task plan %s not found in project %s", planID, projectID), "TASK_PLAN_NOT_FOUND")
		return nil, false
	}

	return plan, true
}

func parseFailPolicy(raw string) (core.FailurePolicy, error) {
	switch raw {
	case "", string(core.FailBlock):
		return core.FailBlock, nil
	case string(core.FailSkip):
		return core.FailSkip, nil
	case string(core.FailHuman):
		return core.FailHuman, nil
	default:
		return "", fmt.Errorf("invalid fail_policy %q", raw)
	}
}

func validatePlanRejectFeedback(feedback *planActionFeedback) error {
	if feedback == nil {
		return fmt.Errorf("reject action requires feedback")
	}

	// 第一段：字段必填校验（category + detail）
	category := strings.TrimSpace(feedback.Category)
	if category == "" {
		return fmt.Errorf("reject action requires feedback.category")
	}
	detail := strings.TrimSpace(feedback.Detail)
	if detail == "" {
		return fmt.Errorf("reject action requires feedback.detail")
	}

	// 第二段：复用领域校验（合法类别 + detail 最少长度）
	err := secretary.HumanFeedback{
		Category:          secretary.FeedbackCategory(category),
		Detail:            detail,
		ExpectedDirection: strings.TrimSpace(feedback.ExpectedDirection),
	}.Validate()
	if err != nil {
		return err
	}
	return nil
}

func feedbackErrorCode(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "feedback.category"):
		return "FEEDBACK_CATEGORY_REQUIRED"
	case strings.Contains(msg, "feedback.detail"):
		return "FEEDBACK_DETAIL_REQUIRED"
	case strings.Contains(msg, "requires feedback"):
		return "FEEDBACK_REQUIRED"
	default:
		return "INVALID_FEEDBACK"
	}
}
