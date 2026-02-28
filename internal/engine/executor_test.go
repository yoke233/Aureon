package engine

import "testing"

func TestNewPipelineID(t *testing.T) {
	id := NewPipelineID()
	if len(id) != 8+1+12 {
		t.Errorf("unexpected ID length: %s (len=%d)", id, len(id))
	}
}

func TestTemplatesDefined(t *testing.T) {
	for _, name := range []string{"full", "standard", "quick", "hotfix"} {
		stages, ok := Templates[name]
		if !ok {
			t.Errorf("template %s not defined", name)
		}
		if len(stages) == 0 {
			t.Errorf("template %s has no stages", name)
		}
	}

	for _, name := range []string{"quick", "hotfix"} {
		stages := Templates[name]
		hasWT := false
		hasCL := false
		for _, s := range stages {
			if s == "worktree_setup" {
				hasWT = true
			}
			if s == "cleanup" {
				hasCL = true
			}
		}
		if !hasWT {
			t.Errorf("%s missing worktree_setup", name)
		}
		if !hasCL {
			t.Errorf("%s missing cleanup", name)
		}
	}
}
