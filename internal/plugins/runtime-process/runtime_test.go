package runtimeprocess

import (
	"context"
	"io"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestCreateAndWait(t *testing.T) {
	rt := New()
	if err := rt.Init(context.Background()); err != nil {
		t.Fatal(err)
	}

	sess, err := rt.Create(context.Background(), core.RuntimeOpts{
		Command: []string{"go", "version"},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, _ := io.ReadAll(sess.Stdout)
	if err := sess.Wait(); err != nil {
		t.Fatal(err)
	}
	if string(out) == "" {
		t.Error("expected output from command")
	}
}
