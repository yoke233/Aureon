package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/yoke233/zhanggui/internal/core"
)

const maxAttachmentSize = 50 << 20 // 50 MB

func (h *Handler) uploadThreadAttachment(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	if h.dataDir == "" {
		writeError(w, http.StatusInternalServerError, "file storage not configured", "STORAGE_NOT_CONFIGURED")
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

	if err := r.ParseMultipartForm(maxAttachmentSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form", "BAD_REQUEST")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field", "BAD_REQUEST")
		return
	}
	defer file.Close()

	if header.Size > maxAttachmentSize {
		writeError(w, http.StatusRequestEntityTooLarge, "file exceeds 50MB limit", "FILE_TOO_LARGE")
		return
	}

	note := r.FormValue("note")
	uploadedBy := r.FormValue("uploaded_by")

	// Build storage directory.
	storageDir := filepath.Join(h.dataDir, "threads", strconv.FormatInt(threadID, 10), "attachments")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create storage directory", "STORAGE_ERROR")
		return
	}

	// Save file with timestamp prefix to avoid collisions.
	storedName := fmt.Sprintf("%d-%s", time.Now().UnixMilli(), header.Filename)
	fullPath := filepath.Join(storageDir, storedName)

	dst, err := os.Create(fullPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create file", "STORAGE_ERROR")
		return
	}
	written, err := io.Copy(dst, file)
	dst.Close()
	if err != nil {
		os.Remove(fullPath)
		writeError(w, http.StatusInternalServerError, "failed to write file", "STORAGE_ERROR")
		return
	}

	// Relative path stored in DB (relative to dataDir).
	relPath := filepath.Join("threads", strconv.FormatInt(threadID, 10), "attachments", storedName)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	att := &core.ThreadAttachment{
		ThreadID:    threadID,
		FileName:    header.Filename,
		FilePath:    relPath,
		FileSize:    written,
		ContentType: contentType,
		UploadedBy:  uploadedBy,
		Note:        note,
	}

	id, err := h.store.CreateThreadAttachment(r.Context(), att)
	if err != nil {
		os.Remove(fullPath)
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	att.ID = id

	writeJSON(w, http.StatusCreated, att)
}

func (h *Handler) listThreadAttachments(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}

	attachments, err := h.store.ListThreadAttachments(r.Context(), threadID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if attachments == nil {
		attachments = []*core.ThreadAttachment{}
	}
	writeJSON(w, http.StatusOK, attachments)
}

func (h *Handler) downloadThreadAttachment(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}
	attachmentID, ok := urlParamInt64(r, "attachmentID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid attachment ID", "BAD_ID")
		return
	}

	att, err := h.store.GetThreadAttachment(r.Context(), attachmentID)
	if err != nil {
		if err == core.ErrNotFound {
			writeError(w, http.StatusNotFound, "attachment not found", "ATTACHMENT_NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if att.ThreadID != threadID {
		writeError(w, http.StatusNotFound, "attachment not found", "ATTACHMENT_NOT_FOUND")
		return
	}

	fullPath := filepath.Join(h.dataDir, att.FilePath)

	info, err := os.Stat(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "attachment file missing", "FILE_MISSING")
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "cannot download a directory", "IS_DIRECTORY")
		return
	}

	if att.ContentType != "" {
		w.Header().Set("Content-Type", att.ContentType)
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, att.FileName))
	http.ServeFile(w, r, fullPath)
}

func (h *Handler) deleteThreadAttachment(w http.ResponseWriter, r *http.Request) {
	threadID, ok := urlParamInt64(r, "threadID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid thread ID", "BAD_ID")
		return
	}
	attachmentID, ok := urlParamInt64(r, "attachmentID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid attachment ID", "BAD_ID")
		return
	}

	att, err := h.store.GetThreadAttachment(r.Context(), attachmentID)
	if err != nil {
		if err == core.ErrNotFound {
			writeError(w, http.StatusNotFound, "attachment not found", "ATTACHMENT_NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if att.ThreadID != threadID {
		writeError(w, http.StatusNotFound, "attachment not found", "ATTACHMENT_NOT_FOUND")
		return
	}

	// Delete file from disk.
	if att.FilePath != "" {
		fullPath := filepath.Join(h.dataDir, att.FilePath)
		_ = os.RemoveAll(fullPath)
	}

	// Delete DB record.
	if err := h.store.DeleteThreadAttachment(r.Context(), attachmentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
