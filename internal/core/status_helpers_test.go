package core

import "testing"

func TestActionTypeCompositeExists(t *testing.T) {
	if ActionComposite != "composite" {
		t.Fatalf("ActionComposite = %q, want %q", ActionComposite, "composite")
	}
}
