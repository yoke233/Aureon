package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindDevBinaryPrefersRuntimeSandbox(t *testing.T) {
	t.Setenv("AI_WORKFLOW_DEV_BINARY", "")

	repoRoot := t.TempDir()
	runtimePath := filepath.Join(repoRoot, ".runtime", "bin", binaryName())
	distPath := filepath.Join(repoRoot, "dist", binaryName())

	mustWriteTestBinary(t, runtimePath)
	mustWriteTestBinary(t, distPath)

	got, err := findDevBinary(repoRoot)
	if err != nil {
		t.Fatalf("findDevBinary() error = %v", err)
	}
	if got != runtimePath {
		t.Fatalf("findDevBinary() = %q, want %q", got, runtimePath)
	}
}

func TestFindDevBinaryUsesExplicitOverrideFirst(t *testing.T) {
	repoRoot := t.TempDir()
	overridePath := filepath.Join(repoRoot, "custom", binaryName())
	mustWriteTestBinary(t, overridePath)
	t.Setenv("AI_WORKFLOW_DEV_BINARY", overridePath)

	got, err := findDevBinary(repoRoot)
	if err != nil {
		t.Fatalf("findDevBinary() error = %v", err)
	}
	if got != overridePath {
		t.Fatalf("findDevBinary() = %q, want %q", got, overridePath)
	}
}

func TestFindDevBinaryErrorsWhenMissing(t *testing.T) {
	t.Setenv("AI_WORKFLOW_DEV_BINARY", "")

	if _, err := findDevBinary(t.TempDir()); err == nil {
		t.Fatal("expected findDevBinary() to fail when no candidates exist")
	}
}

func mustWriteTestBinary(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "ai-flow.exe"
	}
	return "ai-flow"
}
