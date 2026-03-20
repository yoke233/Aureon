package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/yoke233/zhanggui/internal/application/initiativeapp"
	"github.com/yoke233/zhanggui/internal/core"
)

func (h *Handler) initiativeService() *initiativeapp.Service {
	if h == nil {
		return nil
	}
	var tx initiativeapp.Tx
	if txStore, ok := h.store.(core.TransactionalStore); ok {
		tx = initiativeAppTx{store: txStore}
	}
	return initiativeapp.New(initiativeapp.Config{
		Store:     h.store,
		Tx:        tx,
		Scheduler: h.scheduler,
	})
}

type initiativeAppTx struct {
	store core.TransactionalStore
}

func (t initiativeAppTx) InTx(ctx context.Context, fn func(ctx context.Context, store initiativeapp.Store) error) error {
	if t.store == nil {
		return fmt.Errorf("initiative transaction adapter is not configured")
	}
	return t.store.InTx(ctx, func(store core.Store) error {
		txStore, ok := store.(initiativeapp.Store)
		if !ok {
			return fmt.Errorf("transaction store %T does not implement initiativeapp store", store)
		}
		return fn(ctx, txStore)
	})
}

func writeInitiativeAppFailure(w http.ResponseWriter, err error, fallbackCode string) {
	switch {
	case errors.Is(err, core.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
	case errors.Is(err, core.ErrInvalidTransition):
		writeError(w, http.StatusConflict, err.Error(), "INVALID_STATE")
	default:
		msg := err.Error()
		status := http.StatusBadRequest
		if fallbackCode == "" {
			fallbackCode = "INITIATIVE_FAILED"
		}
		writeError(w, status, msg, fallbackCode)
	}
}
