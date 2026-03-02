package acpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	h := &recordingHandler{}
	c, err := New(testLaunchConfig(t), h, WithEventHandler(h))
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}
	defer func() {
		if err := c.Close(context.Background()); err != nil {
			t.Fatalf("close client failed: %v", err)
		}
	}()

	if err := c.Initialize(ctx, ClientCapabilities{FSRead: true, FSWrite: true, Terminal: true}); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	sess, err := c.NewSession(ctx, NewSessionRequest{CWD: t.TempDir()})
	if err != nil {
		t.Fatalf("new session failed: %v", err)
	}
	if sess.SessionID == "" {
		t.Fatal("expected non-empty session id")
	}

	got, err := c.Prompt(ctx, PromptRequest{
		SessionID: sess.SessionID,
		Prompt:    "hello",
		Metadata: map[string]string{
			"role_id": "worker",
		},
	})
	if err != nil {
		t.Fatalf("prompt failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil prompt result")
	}
	if !strings.Contains(got.Text, "worker") {
		t.Fatalf("expected role metadata in response text, got %q", got.Text)
	}
	if h.writeCount() == 0 {
		t.Fatal("expected write-file tool call to be routed to handler")
	}
	if h.updateCount() == 0 {
		t.Fatal("expected session/update callback to be invoked")
	}
}

func TestClientCloseIsIdempotent(t *testing.T) {
	c, err := New(testLaunchConfig(t), &NopHandler{})
	if err != nil {
		t.Fatalf("new client failed: %v", err)
	}

	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
}

func testLaunchConfig(t *testing.T) LaunchConfig {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	acpDir := filepath.Dir(thisFile)
	repoRoot := filepath.Clean(filepath.Join(acpDir, "..", ".."))
	fakeAgentPath := filepath.Join(repoRoot, "internal", "acpclient", "testdata", "fake_agent.go")
	return LaunchConfig{
		Command: "go",
		Args:    []string{"run", fakeAgentPath},
		WorkDir: repoRoot,
	}
}

type recordingHandler struct {
	mu            sync.Mutex
	writeFileHits int
	updateHits    int
}

func (h *recordingHandler) HandleReadFile(context.Context, ReadFileRequest) (ReadFileResult, error) {
	return ReadFileResult{Content: ""}, nil
}

func (h *recordingHandler) HandleWriteFile(context.Context, WriteFileRequest) (WriteFileResult, error) {
	h.mu.Lock()
	h.writeFileHits++
	h.mu.Unlock()
	return WriteFileResult{BytesWritten: 1}, nil
}

func (h *recordingHandler) HandleRequestPermission(context.Context, PermissionRequest) (PermissionDecision, error) {
	return PermissionDecision{Outcome: "allow"}, nil
}

func (h *recordingHandler) HandleTerminalCreate(context.Context, TerminalCreateRequest) (TerminalCreateResult, error) {
	return TerminalCreateResult{TerminalID: "t1"}, nil
}

func (h *recordingHandler) HandleTerminalWrite(context.Context, TerminalWriteRequest) (TerminalWriteResult, error) {
	return TerminalWriteResult{Written: 0}, nil
}

func (h *recordingHandler) HandleTerminalRead(context.Context, TerminalReadRequest) (TerminalReadResult, error) {
	return TerminalReadResult{}, nil
}

func (h *recordingHandler) HandleTerminalResize(context.Context, TerminalResizeRequest) (TerminalResizeResult, error) {
	return TerminalResizeResult{}, nil
}

func (h *recordingHandler) HandleTerminalClose(context.Context, TerminalCloseRequest) (TerminalCloseResult, error) {
	return TerminalCloseResult{}, nil
}

func (h *recordingHandler) HandleSessionUpdate(context.Context, SessionUpdate) error {
	h.mu.Lock()
	h.updateHits++
	h.mu.Unlock()
	return nil
}

func (h *recordingHandler) writeCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.writeFileHits
}

func (h *recordingHandler) updateCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.updateHits
}

func TestClientNewSessionRetriesWithoutMetadataOnInvalidParams(t *testing.T) {
	serverRead, clientWrite := io.Pipe()
	clientRead, serverWrite := io.Pipe()
	defer clientRead.Close()
	defer clientWrite.Close()
	defer serverRead.Close()
	defer serverWrite.Close()

	transport := NewTransport(clientWrite, clientRead)
	defer func() { _ = transport.Close() }()

	go func() {
		reader := bufio.NewReader(serverRead)
		first := readRPCLine(t, reader)
		if first.Method != "session/new" {
			t.Errorf("expected first method session/new, got %q", first.Method)
			return
		}
		if first.Params.Metadata["role_id"] == "" {
			t.Errorf("expected first request _meta.role_id")
			return
		}
		_ = writeLineJSON(serverWrite, map[string]any{
			"jsonrpc": "2.0",
			"id":      first.ID,
			"error": map[string]any{
				"code":    -32602,
				"message": "Invalid params",
			},
		})

		second := readRPCLine(t, reader)
		if second.Method != "session/new" {
			t.Errorf("expected second method session/new, got %q", second.Method)
			return
		}
		if len(second.Params.Metadata) != 0 {
			t.Errorf("expected second request _meta omitted, got %#v", second.Params.Metadata)
			return
		}
		_ = writeLineJSON(serverWrite, map[string]any{
			"jsonrpc": "2.0",
			"id":      second.ID,
			"result": map[string]any{
				"sessionId": "sid-new-1",
			},
		})
	}()

	client := &Client{
		transport:  transport,
		activeText: make(map[string]*strings.Builder),
	}
	session, err := client.NewSession(context.Background(), NewSessionRequest{
		CWD: "D:\\project\\ai-workflow",
		Metadata: map[string]string{
			"role_id": "secretary",
		},
	})
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if session.SessionID != "sid-new-1" {
		t.Fatalf("expected sid-new-1, got %q", session.SessionID)
	}
}

func TestClientPromptRetriesWithoutMetadataOnInvalidParams(t *testing.T) {
	serverRead, clientWrite := io.Pipe()
	clientRead, serverWrite := io.Pipe()
	defer clientRead.Close()
	defer clientWrite.Close()
	defer serverRead.Close()
	defer serverWrite.Close()

	transport := NewTransport(clientWrite, clientRead)
	defer func() { _ = transport.Close() }()

	go func() {
		reader := bufio.NewReader(serverRead)
		first := readRPCLine(t, reader)
		if first.Method != "session/prompt" {
			t.Errorf("expected first method session/prompt, got %q", first.Method)
			return
		}
		if first.Params.Metadata["role_id"] == "" {
			t.Errorf("expected first prompt _meta.role_id")
			return
		}
		_ = writeLineJSON(serverWrite, map[string]any{
			"jsonrpc": "2.0",
			"id":      first.ID,
			"error": map[string]any{
				"code":    -32602,
				"message": "Invalid params",
			},
		})

		second := readRPCLine(t, reader)
		if second.Method != "session/prompt" {
			t.Errorf("expected second method session/prompt, got %q", second.Method)
			return
		}
		if len(second.Params.Metadata) != 0 {
			t.Errorf("expected second prompt _meta omitted, got %#v", second.Params.Metadata)
			return
		}
		_ = writeLineJSON(serverWrite, map[string]any{
			"jsonrpc": "2.0",
			"id":      second.ID,
			"result": map[string]any{
				"requestId":  "req-compat-1",
				"stopReason": "end_turn",
				"text":       "compat-ok",
			},
		})
	}()

	client := &Client{
		transport:  transport,
		activeText: make(map[string]*strings.Builder),
	}
	result, err := client.Prompt(context.Background(), PromptRequest{
		SessionID: "sid-1",
		Prompt:    "hello",
		Metadata: map[string]string{
			"role_id": "secretary",
		},
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil prompt result")
	}
	if result.Text != "compat-ok" {
		t.Fatalf("expected compat-ok, got %q", result.Text)
	}
}

type rpcLine struct {
	ID     any    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Metadata map[string]string `json:"_meta"`
	} `json:"params"`
}

func readRPCLine(t *testing.T, reader *bufio.Reader) rpcLine {
	t.Helper()
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read rpc line: %v", err)
	}
	var msg rpcLine
	if err := json.Unmarshal(line, &msg); err != nil {
		t.Fatalf("decode rpc line: %v", err)
	}
	return msg
}
