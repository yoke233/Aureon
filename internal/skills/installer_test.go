package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSkillsLinked_FallsBackToCopyWhenLinkFails(t *testing.T) {
	skillsRoot := t.TempDir()
	targetRoot := t.TempDir()
	skillDir := filepath.Join(skillsRoot, "ceo-manage")
	if err := os.MkdirAll(filepath.Join(skillDir, "agents"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(DefaultSkillMD("ceo-manage")), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "agents", "openai.yaml"), []byte("name: ceo-manage\n"), 0o644); err != nil {
		t.Fatalf("write openai.yaml: %v", err)
	}

	orig := linkSkillDir
	linkSkillDir = func(_, _ string) error {
		return fmt.Errorf("mklink /J failed: exit status 0xc0000142")
	}
	t.Cleanup(func() {
		linkSkillDir = orig
	})

	if err := EnsureSkillsLinked(skillsRoot, targetRoot, []string{"ceo-manage"}); err != nil {
		t.Fatalf("EnsureSkillsLinked() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(targetRoot, "ceo-manage", "SKILL.md")); err != nil {
		t.Fatalf("expected copied SKILL.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetRoot, "ceo-manage", "agents", "openai.yaml")); err != nil {
		t.Fatalf("expected copied nested file: %v", err)
	}
}
