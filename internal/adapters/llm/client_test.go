package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientCompleteSupportsResponsesAndChatCompletions(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		wantPath     string
		responseBody string
	}{
		{
			name:     "responses",
			provider: ProviderOpenAIResponse,
			wantPath: "/responses",
			responseBody: `{
				"id":"resp_123",
				"object":"response",
				"created_at":1742000000,
				"model":"gpt-4.1-mini",
				"output":[{"id":"msg_1","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"{\"ok\":true}"}]}]
			}`,
		},
		{
			name:     "chat completions",
			provider: ProviderOpenAIChatCompletion,
			wantPath: "/chat/completions",
			responseBody: `{
				"id":"chatcmpl_123",
				"object":"chat.completion",
				"created":1742000000,
				"model":"gpt-4.1-mini",
				"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"{\"ok\":true}","refusal":""}}]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			var capturedBody string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				capturedBody = string(body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer srv.Close()

			client, err := New(Config{
				Provider: tt.provider,
				BaseURL:  srv.URL,
				APIKey:   "test-key",
				Model:    "gpt-4.1-mini",
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			raw, err := client.Complete(context.Background(), "say hi", []ToolDef{{
				Name:        "generate_dag",
				Description: "desc",
				InputSchema: map[string]any{"type": "object"},
			}})
			if err != nil {
				t.Fatalf("Complete() error = %v", err)
			}
			if string(raw) != `{"ok":true}` {
				t.Fatalf("Complete() = %s", string(raw))
			}
			if capturedPath != tt.wantPath {
				t.Fatalf("path = %q, want %q", capturedPath, tt.wantPath)
			}
			if !strings.Contains(capturedBody, "say hi") {
				t.Fatalf("body missing prompt: %s", capturedBody)
			}
			if tt.provider == ProviderOpenAIChatCompletion && !strings.Contains(capturedBody, "response_format") {
				t.Fatalf("chat completion body missing response_format: %s", capturedBody)
			}
		})
	}
}

func TestClientCompleteTextSupportsChatCompletions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q, want /chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl_123",
			"object":"chat.completion",
			"created":1742000000,
			"model":"gpt-4.1-mini",
			"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"plain text","refusal":""}}]
		}`))
	}))
	defer srv.Close()

	client, err := New(Config{
		Provider: ProviderOpenAIChatCompletion,
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Model:    "gpt-4.1-mini",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	text, err := client.CompleteText(context.Background(), "say hi")
	if err != nil {
		t.Fatalf("CompleteText() error = %v", err)
	}
	if text != "plain text" {
		t.Fatalf("CompleteText() = %q, want plain text", text)
	}
}

func TestNewRejectsUnknownProvider(t *testing.T) {
	if _, err := New(Config{
		Provider: "anthropic",
		APIKey:   "test-key",
		Model:    "gpt-4.1-mini",
	}); err == nil {
		t.Fatal("New() should reject unknown provider")
	}
}
