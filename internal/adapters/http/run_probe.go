package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	probeapp "github.com/yoke233/zhanggui/internal/application/probe"
	"github.com/yoke233/zhanggui/internal/core"
)

type createRunProbeRequest struct {
	Question string `json:"question"`
}

func (h *Handler) createRunProbe(w http.ResponseWriter, r *http.Request) {
	if h.probeSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "run probe service is not configured", "PROBE_UNAVAILABLE")
		return
	}

	runID, ok := urlParamInt64(r, "runID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid run ID", "BAD_ID")
		return
	}

	var req createRunProbeRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid probe request body", "BAD_REQUEST")
			return
		}
	}

	probe, err := h.probeSvc.RequestRunProbe(r.Context(), runID, core.RunProbeTriggerManual, strings.TrimSpace(req.Question), 0)
	if errors.Is(err, core.ErrNotFound) {
		writeError(w, http.StatusNotFound, "run not found", "NOT_FOUND")
		return
	}
	if errors.Is(err, probeapp.ErrRunProbeConflict) {
		writeError(w, http.StatusConflict, "run already has an active probe", "PROBE_CONFLICT")
		return
	}
	if errors.Is(err, probeapp.ErrRunNotRunning) {
		writeError(w, http.StatusConflict, "run is not running", "RUN_NOT_RUNNING")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "PROBE_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, probe)
}

func (h *Handler) listRunProbes(w http.ResponseWriter, r *http.Request) {
	if h.probeSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "run probe service is not configured", "PROBE_UNAVAILABLE")
		return
	}
	runID, ok := urlParamInt64(r, "runID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid run ID", "BAD_ID")
		return
	}
	probes, err := h.probeSvc.ListRunProbes(r.Context(), runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "PROBE_LIST_ERROR")
		return
	}
	if probes == nil {
		probes = []*core.RunProbe{}
	}
	writeJSON(w, http.StatusOK, probes)
}

func (h *Handler) getLatestRunProbe(w http.ResponseWriter, r *http.Request) {
	if h.probeSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "run probe service is not configured", "PROBE_UNAVAILABLE")
		return
	}
	runID, ok := urlParamInt64(r, "runID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid run ID", "BAD_ID")
		return
	}
	probe, err := h.probeSvc.GetLatestRunProbe(r.Context(), runID)
	if errors.Is(err, core.ErrNotFound) {
		writeError(w, http.StatusNotFound, "probe not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "PROBE_GET_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, probe)
}
