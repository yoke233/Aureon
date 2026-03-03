package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/yoke233/ai-workflow/internal/core"
)

const (
	defaultIssueParserRoleID      = "plan_parser"
	maxIssueSourceFileBytes       = 1 << 20 // 1MB
	maxIssueSourceFilesTotalBytes = 5 << 20 // 5MB
	minIssueFeedbackDetailRunes   = 20
)

type issueHandlers struct {
	store        core.Store
	issueManager IssueManager
	issueRoleID  string
}

type createIssuesRequest struct {
	SessionID  string `json:"session_id"`
	Name       string `json:"name"`
	FailPolicy string `json:"fail_policy"`
}

type createIssuesFromFilesRequest struct {
	SessionID  string   `json:"session_id"`
	Name       string   `json:"name"`
	FailPolicy string   `json:"fail_policy"`
	FilePaths  []string `json:"file_paths"`
}

type issueListResponse struct {
	Items  []core.Issue `json:"items"`
	Total  int          `json:"total"`
	Offset int          `json:"offset"`
}

type issueStatusResponse struct {
	Status string `json:"status"`
}

type issueDAGNode struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Status     core.IssueStatus `json:"status"`
	PipelineID string           `json:"pipeline_id"`
}

type issueDAGEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type issueDAGStats struct {
	Total   int `json:"total"`
	Pending int `json:"pending"`
	Ready   int `json:"ready"`
	Running int `json:"running"`
	Done    int `json:"done"`
	Failed  int `json:"failed"`
}

type issueDAGResponse struct {
	Nodes []issueDAGNode `json:"nodes"`
	Edges []issueDAGEdge `json:"edges"`
	Stats issueDAGStats  `json:"stats"`
}

type issueActionRequest struct {
	Action   string               `json:"action"`
	Feedback *issueActionFeedback `json:"feedback,omitempty"`
}

type issueActionFeedback struct {
	Category          string `json:"category"`
	Detail            string `json:"detail"`
	ExpectedDirection string `json:"expected_direction,omitempty"`
}

var allowedIssueFeedbackCategories = map[string]struct{}{
	"missing_node":    {},
	"cycle":           {},
	"self_dependency": {},
	"bad_granularity": {},
	"coverage_gap":    {},
	"other":           {},
}

func registerIssueRoutes(r chi.Router, store core.Store, issueManager IssueManager, issueParserRoleID string) {
	h := &issueHandlers{
		store:        store,
		issueManager: issueManager,
		issueRoleID:  resolveIssueParserRoleID(issueParserRoleID),
	}

	registerResourceRoutes := func(base string) {
		r.Post(base, h.createIssues)
		r.Post(base+"/from-files", h.createIssuesFromFiles)
		r.Get(base, h.listIssues)
		r.Get(base+"/{id}", h.getIssue)
		r.Get(base+"/{id}/dag", h.getIssueDAG)
		r.Post(base+"/{id}/review", h.submitForReview)
		r.Post(base+"/{id}/action", h.applyIssueAction)
	}

	registerResourceRoutes("/projects/{projectID}/issues")
	// Backward-compatible aliases during cutover.
	registerResourceRoutes("/projects/{projectID}/plans")
}

// registerPlanRoutes is kept as a compatibility alias for existing call sites.
func registerPlanRoutes(r chi.Router, store core.Store, issueManager IssueManager, issueParserRoleID string) {
	registerIssueRoutes(r, store, issueManager, issueParserRoleID)
}

func (h *issueHandlers) createIssues(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}
	if h.issueManager == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "issue manager is not configured", "ISSUE_MANAGER_UNAVAILABLE")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	if projectID == "" {
		writeAPIError(w, http.StatusBadRequest, "project id is required", "PROJECT_ID_REQUIRED")
		return
	}
	project, err := h.store.GetProject(projectID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("project %s not found", projectID), "PROJECT_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load project", "GET_PROJECT_FAILED")
		return
	}

	var req createIssuesRequest
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

	createReq := IssueCreateRequest{
		Conversation: summarizeChatMessages(session.Messages),
		ProjectName:  strings.TrimSpace(project.Name),
		RepoPath:     strings.TrimSpace(project.RepoPath),
		Role:         h.issueRoleID,
		WorkDir:      strings.TrimSpace(project.RepoPath),
	}
	if createReq.WorkDir == "" {
		createReq.WorkDir = "."
	}

	issues, err := h.issueManager.CreateIssues(r.Context(), IssueCreateInput{
		ProjectID:  projectID,
		SessionID:  req.SessionID,
		Name:       req.Name,
		FailPolicy: failPolicy,
		Request:    createReq,
	})
	if err != nil {
		log.Printf("[web][issue] create issues failed project=%s session=%s err=%v", projectID, req.SessionID, err)
		writeAPIError(w, http.StatusInternalServerError, "failed to create issues", "CREATE_ISSUES_FAILED")
		return
	}
	if len(issues) == 0 {
		writeAPIError(w, http.StatusInternalServerError, "failed to create issues", "CREATE_ISSUES_FAILED")
		return
	}

	writeJSON(w, http.StatusCreated, buildCreateIssuesResponse(issues))
}

func (h *issueHandlers) createIssuesFromFiles(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return
	}
	if h.issueManager == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "issue manager is not configured", "ISSUE_MANAGER_UNAVAILABLE")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	if projectID == "" {
		writeAPIError(w, http.StatusBadRequest, "project id is required", "PROJECT_ID_REQUIRED")
		return
	}

	var req createIssuesFromFilesRequest
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
	if len(req.FilePaths) == 0 {
		writeAPIError(w, http.StatusBadRequest, "file_paths is required", "FILE_PATHS_REQUIRED")
		return
	}

	project, err := h.store.GetProject(projectID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("project %s not found", projectID), "PROJECT_NOT_FOUND")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load project", "GET_PROJECT_FAILED")
		return
	}
	repoPath := strings.TrimSpace(project.RepoPath)
	if repoPath == "" {
		writeAPIError(w, http.StatusBadRequest, "project repo_path is required", "REPO_PATH_REQUIRED")
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

	sourceFiles, fileContents, err := loadIssueSourceFiles(repoPath, req.FilePaths)
	if err != nil {
		var validationErr *planFilesValidationError
		if errors.As(err, &validationErr) {
			writeAPIError(w, http.StatusBadRequest, validationErr.Error(), validationErr.Code)
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to read source files", "READ_SOURCE_FILES_FAILED")
		return
	}

	createReq := IssueCreateRequest{
		Conversation: summarizeChatMessages(session.Messages),
		ProjectName:  strings.TrimSpace(project.Name),
		RepoPath:     repoPath,
		Role:         h.issueRoleID,
		WorkDir:      repoPath,
	}
	if createReq.WorkDir == "" {
		createReq.WorkDir = "."
	}

	createdIssues, err := h.issueManager.CreateIssues(r.Context(), IssueCreateInput{
		ProjectID:    projectID,
		SessionID:    req.SessionID,
		Name:         req.Name,
		FailPolicy:   failPolicy,
		Request:      createReq,
		SourceFiles:  sourceFiles,
		FileContents: cloneIssueStringMap(fileContents),
	})
	if err != nil {
		log.Printf("[web][issue] create issues from files failed project=%s session=%s err=%v", projectID, req.SessionID, err)
		writeAPIError(w, http.StatusInternalServerError, "failed to create issues", "CREATE_ISSUES_FAILED")
		return
	}
	if len(createdIssues) == 0 {
		writeAPIError(w, http.StatusInternalServerError, "failed to create issues", "CREATE_ISSUES_FAILED")
		return
	}

	submittedIssues := make([]core.Issue, 0, len(createdIssues))
	for i := range createdIssues {
		issueID := strings.TrimSpace(createdIssues[i].ID)
		if issueID == "" {
			writeAPIError(w, http.StatusInternalServerError, "failed to create issues", "CREATE_ISSUES_FAILED")
			return
		}
		reviewInput := h.buildReviewInput(&createdIssues[i])
		reviewInput.FileContents = cloneIssueStringMap(fileContents)
		updated, err := h.issueManager.SubmitForReview(r.Context(), issueID, reviewInput)
		if err != nil {
			if isIssueStatusConflictError(err) {
				writeAPIError(w, http.StatusConflict, err.Error(), "ISSUE_STATUS_INVALID")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "failed to update issue", "SAVE_ISSUE_FAILED")
			return
		}
		if normalized := normalizeIssueForAPI(updated); normalized != nil {
			submittedIssues = append(submittedIssues, *normalized)
			continue
		}
		if normalized := normalizeIssueForAPI(&createdIssues[i]); normalized != nil {
			submittedIssues = append(submittedIssues, *normalized)
		}
	}

	if len(submittedIssues) == 0 {
		writeAPIError(w, http.StatusInternalServerError, "failed to update issue", "SAVE_ISSUE_FAILED")
		return
	}

	writeJSON(w, http.StatusCreated, buildIssueFromFilesResponse(submittedIssues, sourceFiles, fileContents))
}

func resolveIssueParserRoleID(roleID string) string {
	trimmed := strings.TrimSpace(roleID)
	if trimmed == "" {
		return defaultIssueParserRoleID
	}
	return trimmed
}

func (h *issueHandlers) listIssues(w http.ResponseWriter, r *http.Request) {
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

	items, total, err := h.store.ListIssues(projectID, core.IssueFilter{
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to list issues", "LIST_ISSUES_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, issueListResponse{
		Items:  normalizeIssuesForAPI(items),
		Total:  total,
		Offset: offset,
	})
}

func (h *issueHandlers) getIssue(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForProject(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, normalizeIssueForAPI(issue))
}

func (h *issueHandlers) getIssueDAG(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForProject(w, r)
	if !ok {
		return
	}

	allIssues, _, err := h.store.ListIssues(issue.ProjectID, core.IssueFilter{})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "failed to list issues", "LIST_ISSUES_FAILED")
		return
	}

	allByID := make(map[string]core.Issue, len(allIssues)+1)
	for i := range allIssues {
		normalized := normalizeIssueForAPI(&allIssues[i])
		if normalized == nil {
			continue
		}
		allByID[strings.TrimSpace(normalized.ID)] = *normalized
	}
	if normalized := normalizeIssueForAPI(issue); normalized != nil {
		allByID[strings.TrimSpace(normalized.ID)] = *normalized
	}

	rootID := strings.TrimSpace(issue.ID)
	inScope := map[string]struct{}{}
	addInScope := func(id string) {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			return
		}
		if _, exists := allByID[trimmed]; !exists {
			return
		}
		inScope[trimmed] = struct{}{}
	}

	addInScope(rootID)
	for _, dep := range issue.DependsOn {
		addInScope(dep)
	}
	for _, blocked := range issue.Blocks {
		addInScope(blocked)
	}
	for _, candidate := range allByID {
		if hasIssueReference(candidate.DependsOn, rootID) || hasIssueReference(candidate.Blocks, rootID) {
			addInScope(candidate.ID)
		}
	}

	nodeIDs := make([]string, 0, len(inScope))
	for id := range inScope {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	nodes := make([]issueDAGNode, 0, len(nodeIDs))
	stats := issueDAGStats{}
	for _, id := range nodeIDs {
		item, ok := allByID[id]
		if !ok {
			continue
		}
		nodes = append(nodes, issueDAGNode{
			ID:         item.ID,
			Title:      item.Title,
			Status:     item.Status,
			PipelineID: item.PipelineID,
		})
		stats.Total++
		accumulateIssueStats(&stats, item.Status)
	}

	edges := make([]issueDAGEdge, 0, len(nodeIDs)*2)
	edgeSeen := make(map[string]struct{}, len(nodeIDs)*2)
	addEdge := func(from, to string) {
		from = strings.TrimSpace(from)
		to = strings.TrimSpace(to)
		if from == "" || to == "" {
			return
		}
		if _, ok := inScope[from]; !ok {
			return
		}
		if _, ok := inScope[to]; !ok {
			return
		}
		key := from + "->" + to
		if _, exists := edgeSeen[key]; exists {
			return
		}
		edgeSeen[key] = struct{}{}
		edges = append(edges, issueDAGEdge{From: from, To: to})
	}

	for _, id := range nodeIDs {
		item, ok := allByID[id]
		if !ok {
			continue
		}
		for _, dep := range item.DependsOn {
			addEdge(dep, item.ID)
		}
		for _, blocked := range item.Blocks {
			addEdge(item.ID, blocked)
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})

	writeJSON(w, http.StatusOK, issueDAGResponse{
		Nodes: nodes,
		Edges: edges,
		Stats: stats,
	})
}

func accumulateIssueStats(stats *issueDAGStats, status core.IssueStatus) {
	if stats == nil {
		return
	}
	switch status {
	case core.IssueStatusReady:
		stats.Ready++
	case core.IssueStatusExecuting:
		stats.Running++
	case core.IssueStatusDone:
		stats.Done++
	case core.IssueStatusFailed, core.IssueStatusSuperseded, core.IssueStatusAbandoned:
		stats.Failed++
	default:
		stats.Pending++
	}
}

func hasIssueReference(values []string, target string) bool {
	trimmedTarget := strings.TrimSpace(target)
	if trimmedTarget == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), trimmedTarget) {
			return true
		}
	}
	return false
}

func (h *issueHandlers) submitForReview(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForProject(w, r)
	if !ok {
		return
	}

	if h.issueManager == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "issue manager is not configured", "ISSUE_MANAGER_UNAVAILABLE")
		return
	}

	updated, err := h.issueManager.SubmitForReview(r.Context(), issue.ID, h.buildReviewInput(issue))
	if err != nil {
		if isIssueStatusConflictError(err) {
			writeAPIError(w, http.StatusConflict, err.Error(), "ISSUE_STATUS_INVALID")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to update issue", "SAVE_ISSUE_FAILED")
		return
	}

	status := issue.Status
	if updated != nil {
		status = updated.Status
	}
	writeJSON(w, http.StatusOK, issueStatusResponse{
		Status: string(status),
	})
}

func (h *issueHandlers) applyIssueAction(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForProject(w, r)
	if !ok {
		return
	}

	var req issueActionRequest
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

	if h.issueManager == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "issue manager is not configured", "ISSUE_MANAGER_UNAVAILABLE")
		return
	}

	managerAction := IssueAction{Action: action}
	switch action {
	case "approve":
		// no-op
	case "reject":
		if err := validateIssueRejectFeedback(req.Feedback); err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), feedbackErrorCode(err))
			return
		}
		managerAction.Feedback = &IssueFeedback{
			Category:          strings.TrimSpace(req.Feedback.Category),
			Detail:            strings.TrimSpace(req.Feedback.Detail),
			ExpectedDirection: strings.TrimSpace(req.Feedback.ExpectedDirection),
		}
	case "abort", "abandon":
		managerAction.Action = "abandon"
	default:
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("unsupported issue action %q", action), "INVALID_ACTION")
		return
	}

	updated, err := h.issueManager.ApplyIssueAction(r.Context(), issue.ID, managerAction)
	if err != nil {
		switch {
		case isIssueStatusConflictError(err):
			writeAPIError(w, http.StatusConflict, err.Error(), "ISSUE_STATUS_INVALID")
		case isFeedbackValidationError(err):
			writeAPIError(w, http.StatusBadRequest, err.Error(), feedbackErrorCode(err))
		case strings.Contains(strings.ToLower(err.Error()), "unsupported issue action"),
			strings.Contains(strings.ToLower(err.Error()), "unsupported plan action"):
			writeAPIError(w, http.StatusBadRequest, err.Error(), "INVALID_ACTION")
		default:
			writeAPIError(w, http.StatusInternalServerError, "failed to update issue", "SAVE_ISSUE_FAILED")
		}
		return
	}

	status := issue.Status
	if updated != nil {
		status = updated.Status
	}
	writeJSON(w, http.StatusOK, issueStatusResponse{
		Status: string(status),
	})
}

func (h *issueHandlers) loadIssueForProject(w http.ResponseWriter, r *http.Request) (*core.Issue, bool) {
	if h.store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "store is not configured", "STORE_UNAVAILABLE")
		return nil, false
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "projectID"))
	issueID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" || issueID == "" {
		writeAPIError(w, http.StatusBadRequest, "project id and issue id are required", "INVALID_PATH_PARAM")
		return nil, false
	}

	issue, err := h.store.GetIssue(issueID)
	if err != nil {
		if isNotFoundError(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Sprintf("issue %s not found", issueID), "ISSUE_NOT_FOUND")
			return nil, false
		}
		writeAPIError(w, http.StatusInternalServerError, "failed to load issue", "GET_ISSUE_FAILED")
		return nil, false
	}
	if issue.ProjectID != projectID {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("issue %s not found in project %s", issueID, projectID), "ISSUE_NOT_FOUND")
		return nil, false
	}

	return issue, true
}

func (h *issueHandlers) buildReviewInput(issue *core.Issue) IssueReviewInput {
	if h == nil || h.store == nil || issue == nil {
		return IssueReviewInput{}
	}

	input := IssueReviewInput{}
	sessionID := strings.TrimSpace(issue.SessionID)
	if sessionID != "" {
		if session, err := h.store.GetChatSession(sessionID); err == nil && session != nil {
			input.Conversation = summarizeChatMessages(session.Messages)
		}
	}

	if project, err := h.store.GetProject(issue.ProjectID); err == nil && project != nil {
		projectName := strings.TrimSpace(project.Name)
		repoPath := strings.TrimSpace(project.RepoPath)
		parts := make([]string, 0, 2)
		if projectName != "" {
			parts = append(parts, "project="+projectName)
		}
		if repoPath != "" {
			parts = append(parts, "repo="+repoPath)
		}
		input.ProjectContext = strings.Join(parts, " ")
	}
	return input
}

func summarizeChatMessages(messages []core.ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}
	lines := make([]string, 0, len(messages))
	for i := range messages {
		content := strings.TrimSpace(messages[i].Content)
		if content == "" {
			continue
		}
		role := strings.TrimSpace(messages[i].Role)
		if role == "" {
			role = "user"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", role, content))
	}
	return strings.Join(lines, "\n")
}

func isIssueStatusConflictError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "submit for review requires") ||
		strings.Contains(msg, "submit review requires") ||
		strings.Contains(msg, "approve requires") ||
		strings.Contains(msg, "reject requires") ||
		strings.Contains(msg, "abandon requires")
}

func isFeedbackValidationError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "feedback")
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

func validateIssueRejectFeedback(feedback *issueActionFeedback) error {
	if feedback == nil {
		return fmt.Errorf("reject action requires feedback")
	}

	category := strings.TrimSpace(feedback.Category)
	if category == "" {
		return fmt.Errorf("reject action requires feedback.category")
	}
	detail := strings.TrimSpace(feedback.Detail)
	if detail == "" {
		return fmt.Errorf("reject action requires feedback.detail")
	}
	if _, ok := allowedIssueFeedbackCategories[category]; !ok {
		return fmt.Errorf("invalid feedback category %q", category)
	}
	if utf8.RuneCountInString(detail) < minIssueFeedbackDetailRunes {
		return fmt.Errorf("feedback detail must be at least %d characters", minIssueFeedbackDetailRunes)
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

type planFilesValidationError struct {
	Message string
	Code    string
}

func (e *planFilesValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func loadIssueSourceFiles(repoPath string, filePaths []string) ([]string, map[string]string, error) {
	repoRoot := strings.TrimSpace(repoPath)
	if repoRoot == "" {
		return nil, nil, &planFilesValidationError{
			Message: "project repo_path is required",
			Code:    "REPO_PATH_REQUIRED",
		}
	}
	absRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, nil, &planFilesValidationError{
			Message: "invalid project repo_path",
			Code:    "INVALID_REPO_PATH",
		}
	}

	sourceFiles := make([]string, 0, len(filePaths))
	fileContents := make(map[string]string, len(filePaths))
	seen := make(map[string]struct{}, len(filePaths))
	var totalBytes int64

	for i := range filePaths {
		absPath, normalizedPath, err := resolveIssueSourceFilePath(absRepoRoot, filePaths[i])
		if err != nil {
			return nil, nil, err
		}
		if _, duplicated := seen[normalizedPath]; duplicated {
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil, &planFilesValidationError{
					Message: fmt.Sprintf("source file %s not found", normalizedPath),
					Code:    "FILE_NOT_FOUND",
				}
			}
			return nil, nil, &planFilesValidationError{
				Message: fmt.Sprintf("failed to read source file %s", normalizedPath),
				Code:    "FILE_READ_FAILED",
			}
		}
		if info.IsDir() {
			return nil, nil, &planFilesValidationError{
				Message: fmt.Sprintf("source file %s not found", normalizedPath),
				Code:    "FILE_NOT_FOUND",
			}
		}
		if info.Size() > maxIssueSourceFileBytes {
			return nil, nil, &planFilesValidationError{
				Message: fmt.Sprintf("source file %s exceeds 1MB", normalizedPath),
				Code:    "FILE_TOO_LARGE",
			}
		}
		if totalBytes+info.Size() > maxIssueSourceFilesTotalBytes {
			return nil, nil, &planFilesValidationError{
				Message: "total source file size exceeds 5MB",
				Code:    "FILE_TOTAL_TOO_LARGE",
			}
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil, nil, &planFilesValidationError{
				Message: fmt.Sprintf("failed to read source file %s", normalizedPath),
				Code:    "FILE_READ_FAILED",
			}
		}
		contentBytes := int64(len(content))
		if contentBytes > maxIssueSourceFileBytes {
			return nil, nil, &planFilesValidationError{
				Message: fmt.Sprintf("source file %s exceeds 1MB", normalizedPath),
				Code:    "FILE_TOO_LARGE",
			}
		}
		if totalBytes+contentBytes > maxIssueSourceFilesTotalBytes {
			return nil, nil, &planFilesValidationError{
				Message: "total source file size exceeds 5MB",
				Code:    "FILE_TOTAL_TOO_LARGE",
			}
		}

		sourceFiles = append(sourceFiles, normalizedPath)
		fileContents[normalizedPath] = string(content)
		seen[normalizedPath] = struct{}{}
		totalBytes += contentBytes
	}

	if len(sourceFiles) == 0 {
		return nil, nil, &planFilesValidationError{
			Message: "file_paths is required",
			Code:    "FILE_PATHS_REQUIRED",
		}
	}
	return sourceFiles, fileContents, nil
}

func resolveIssueSourceFilePath(repoRoot string, rawPath string) (string, string, error) {
	trimmed := strings.TrimSpace(rawPath)
	absPath, normalizedPath, err := validateRelativePath(repoRoot, trimmed)
	if err != nil {
		if errors.Is(err, errRelativePathRequired) {
			return "", "", &planFilesValidationError{
				Message: "file_paths contains empty path",
				Code:    "FILE_PATH_REQUIRED",
			}
		}
		return "", "", &planFilesValidationError{
			Message: fmt.Sprintf("invalid file path %q", trimmed),
			Code:    "INVALID_FILE_PATH",
		}
	}
	if normalizedPath == "." {
		return "", "", &planFilesValidationError{
			Message: "file_paths contains empty path",
			Code:    "FILE_PATH_REQUIRED",
		}
	}
	return absPath, normalizedPath, nil
}

func buildCreateIssuesResponse(issues []core.Issue) map[string]any {
	normalized := normalizeIssuesForAPI(issues)
	payload := map[string]any{
		"items": normalized,
	}
	if len(normalized) > 0 {
		issue := normalized[0]
		payload["issue"] = issue
	}
	return payload
}

func buildIssueFromFilesResponse(issues []core.Issue, sourceFiles []string, fileContents map[string]string) map[string]any {
	payload := buildCreateIssuesResponse(issues)
	payload["source_files"] = normalizeStringSlice(sourceFiles)
	payload["file_contents"] = cloneIssueStringMap(fileContents)
	return payload
}

func cloneIssueStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func normalizeIssuesForAPI(items []core.Issue) []core.Issue {
	if len(items) == 0 {
		return []core.Issue{}
	}
	out := make([]core.Issue, len(items))
	for i := range items {
		normalized := normalizeIssueForAPI(&items[i])
		if normalized == nil {
			out[i] = core.Issue{}
			continue
		}
		out[i] = *normalized
	}
	return out
}

func normalizeIssueForAPI(issue *core.Issue) *core.Issue {
	if issue == nil {
		return nil
	}
	clone := *issue
	clone.Labels = normalizeStringSlice(issue.Labels)
	clone.Attachments = normalizeStringSlice(issue.Attachments)
	clone.DependsOn = normalizeStringSlice(issue.DependsOn)
	clone.Blocks = normalizeStringSlice(issue.Blocks)
	return &clone
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}
