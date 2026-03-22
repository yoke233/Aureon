package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func findDevBinary(repoRoot string) (string, error) {
	candidates := devBinaryCandidates(repoRoot)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("ai-flow dev binary not found in %s", strings.Join(candidates, ", "))
}

func devBinaryCandidates(repoRoot string) []string {
	candidates := []string{}
	if custom := strings.TrimSpace(os.Getenv("AI_WORKFLOW_DEV_BINARY")); custom != "" {
		candidates = append(candidates, custom)
	}

	exeName := "ai-flow"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}

	return append(candidates,
		filepath.Join(repoRoot, ".runtime", "bin", exeName),
		filepath.Join(repoRoot, "dist", exeName),
		filepath.Join(repoRoot, "ai-flow.exe"), // legacy fallback for older local setups
		filepath.Join(repoRoot, "ai-flow"),
	)
}
