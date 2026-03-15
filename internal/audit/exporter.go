package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type RunAuditRecord struct {
	EventName      string         `json:"event_name"`
	WorkItemID     int64          `json:"work_item_id,omitempty"`
	ActionID       int64          `json:"action_id,omitempty"`
	RunID          int64          `json:"run_id,omitempty"`
	Kind           string         `json:"kind"`
	Status         string         `json:"status"`
	RedactionLevel string         `json:"redaction_level,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type Exporter interface {
	ExportRunAudit(ctx context.Context, logRef string, records []RunAuditRecord) error
}

type FileExporter struct {
	rootDir string
}

func NewFileExporter(rootDir string) *FileExporter {
	return &FileExporter{rootDir: filepath.Clean(strings.TrimSpace(rootDir))}
}

func (e *FileExporter) ExportRunAudit(_ context.Context, logRef string, records []RunAuditRecord) error {
	return writeJSONLRecords(e.rootDir, logRef, records)
}

func buildRunAuditLogRef(runID int64, now time.Time) string {
	return filepath.ToSlash(filepath.Join(
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
		fmt.Sprintf("run-%d-audit.jsonl", runID),
	))
}

func ReadRunAuditRecords(rootDir, logRef string) ([]RunAuditRecord, error) {
	path, err := resolveLogPath(rootDir, logRef)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	records := make([]RunAuditRecord, 0)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var record RunAuditRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf("decode run audit record: %w", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan run audit file: %w", err)
	}
	return records, nil
}

func writeJSONLRecords[T any](rootDir, logRef string, records []T) error {
	if len(records) == 0 {
		return nil
	}
	path, err := resolveLogPath(rootDir, logRef)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir audit payload dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open audit payload file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, record := range records {
		if err := enc.Encode(record); err != nil {
			return fmt.Errorf("write audit payload record: %w", err)
		}
	}
	return nil
}
