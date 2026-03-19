package httpx

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yoke233/zhanggui/internal/platform/config"
)

func TestTokenAuthMiddleware_DoesNotWarnForWebSocketQueryToken(t *testing.T) {
	registry := NewTokenRegistry(map[string]config.TokenEntry{
		"admin": {Token: "secret-token", Scopes: []string{"*"}},
	})
	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)

	handler := TokenAuthMiddleware(registry, WithAuthLogger(logger))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ws?token=secret-token", nil)
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if strings.Contains(logs.String(), "SECURITY WARNING") {
		t.Fatalf("unexpected security warning for websocket query token: %s", logs.String())
	}
}

func TestTokenAuthMiddleware_WarnsForHTTPQueryToken(t *testing.T) {
	registry := NewTokenRegistry(map[string]config.TokenEntry{
		"admin": {Token: "secret-token", Scopes: []string{"*"}},
	})
	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)

	handler := TokenAuthMiddleware(registry, WithAuthLogger(logger))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/projects?token=secret-token", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if !strings.Contains(logs.String(), "SECURITY WARNING") {
		t.Fatalf("expected security warning for http query token, got logs: %s", logs.String())
	}
}
