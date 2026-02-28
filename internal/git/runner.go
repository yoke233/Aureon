package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Runner struct {
	repoDir string
}

func NewRunner(repoDir string) *Runner {
	return &Runner{repoDir: repoDir}
}

func (r *Runner) run(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", r.repoDir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}
