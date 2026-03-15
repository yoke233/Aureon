package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/yoke233/ai-workflow/internal/application/workitemtrackapp"
	"github.com/yoke233/ai-workflow/internal/core"
)

func (h *Handler) workItemTrackService() *workitemtrackapp.Service {
	if h == nil {
		return nil
	}
	var tx workitemtrackapp.Tx
	if txStore, ok := h.store.(core.TransactionalStore); ok {
		tx = workItemTrackAppTx{store: txStore}
	}
	var leadDispatch workitemtrackapp.LeadDispatcher
	if h.threadPool != nil {
		leadDispatch = workItemTrackLeadDispatcher{pool: h.threadPool}
	}
	return workitemtrackapp.New(workitemtrackapp.Config{
		Store:        h.store,
		Tx:           tx,
		Bus:          h.bus,
		Executor:     workItemTrackExecutor{handler: h},
		LeadDispatch: leadDispatch,
	})
}

type workItemTrackExecutor struct {
	handler *Handler
}

func (e workItemTrackExecutor) RunWorkItem(ctx context.Context, workItemID int64) error {
	if e.handler == nil {
		return fmt.Errorf("work item track executor is not configured")
	}
	_, err := e.handler.workItemService().RunWorkItem(ctx, workItemID)
	return err
}

type workItemTrackLeadDispatcher struct {
	pool ThreadAgentRuntime
}

func (d workItemTrackLeadDispatcher) DispatchLeadToThread(ctx context.Context, threadID int64, kickoffMessage string) error {
	if d.pool == nil {
		return fmt.Errorf("thread agent runtime is not available")
	}

	// Invite lead agent. InviteAgent is idempotent — returns existing member if already active.
	if _, err := d.pool.InviteAgent(ctx, threadID, "lead"); err != nil {
		return fmt.Errorf("invite lead to thread %d: %w", threadID, err)
	}

	// Send the kickoff message so the lead starts planning.
	if err := d.pool.SendMessage(ctx, threadID, "lead", kickoffMessage); err != nil {
		return fmt.Errorf("send kickoff to lead in thread %d: %w", threadID, err)
	}
	return nil
}

type workItemTrackAppTx struct {
	store core.TransactionalStore
}

func (t workItemTrackAppTx) InTx(ctx context.Context, fn func(ctx context.Context, store workitemtrackapp.TxStore) error) error {
	if t.store == nil {
		return fmt.Errorf("work item track transaction adapter is not configured")
	}
	return t.store.InTx(ctx, func(store core.Store) error {
		txStore, ok := store.(workitemtrackapp.TxStore)
		if !ok {
			return fmt.Errorf("transaction store %T does not implement workitemtrackapp tx store", store)
		}
		return fn(ctx, txStore)
	})
}

func writeWorkItemTrackAppError(w http.ResponseWriter, err error) bool {
	switch workitemtrackapp.CodeOf(err) {
	case workitemtrackapp.CodeTrackNotFound:
		writeError(w, http.StatusNotFound, "track not found", workitemtrackapp.CodeTrackNotFound)
	case workitemtrackapp.CodeThreadNotFound:
		writeError(w, http.StatusNotFound, "thread not found", workitemtrackapp.CodeThreadNotFound)
	case workitemtrackapp.CodeWorkItemNotFound:
		writeError(w, http.StatusNotFound, "work item not found", workitemtrackapp.CodeWorkItemNotFound)
	case workitemtrackapp.CodeMissingThreadID:
		writeError(w, http.StatusBadRequest, "thread_id is required", workitemtrackapp.CodeMissingThreadID)
	case workitemtrackapp.CodeMissingTitle:
		writeError(w, http.StatusBadRequest, "title is required", workitemtrackapp.CodeMissingTitle)
	case workitemtrackapp.CodeInvalidRelationType:
		writeError(w, http.StatusBadRequest, err.Error(), workitemtrackapp.CodeInvalidRelationType)
	case workitemtrackapp.CodeInvalidState:
		writeError(w, http.StatusConflict, err.Error(), workitemtrackapp.CodeInvalidState)
	case workitemtrackapp.CodeRunUnavailable:
		writeError(w, http.StatusServiceUnavailable, err.Error(), workitemtrackapp.CodeRunUnavailable)
	default:
		return false
	}
	return true
}

func writeWorkItemTrackAppFailure(w http.ResponseWriter, err error, fallbackCode string) {
	if writeWorkItemTrackAppError(w, err) {
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error(), fallbackCode)
}
