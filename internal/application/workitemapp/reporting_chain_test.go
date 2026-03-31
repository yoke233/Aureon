package workitemapp

import (
	"context"
	"reflect"
	"testing"

	agentapp "github.com/yoke233/zhanggui/internal/application/agent"
	"github.com/yoke233/zhanggui/internal/core"
)

func TestBuildEscalationPathUsesCurrentManagerChain(t *testing.T) {
	registry := agentapp.NewConfigRegistry()
	registry.LoadProfiles([]*core.AgentProfile{
		{ID: "ceo", Role: core.RoleLead},
		{ID: "lead", Role: core.RoleLead, ManagerProfileID: "ceo"},
		{ID: "worker", Role: core.RoleWorker, ManagerProfileID: "lead"},
	})

	path, err := BuildEscalationPath(context.Background(), "worker", registry)
	if err != nil {
		t.Fatalf("BuildEscalationPath: %v", err)
	}
	want := []string{"lead", "ceo", "human"}
	if !reflect.DeepEqual(path, want) {
		t.Fatalf("path = %#v, want %#v", path, want)
	}
}
