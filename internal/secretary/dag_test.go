package secretary

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/user/ai-workflow/internal/core"
)

func TestDAGBuildValidateReadyNodes(t *testing.T) {
	dag := Build([]core.TaskItem{
		task("A"),
		task("B", "A"),
		task("C", "A"),
		task("D", "B", "C"),
	})

	if err := dag.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if got := dag.ReadyNodes(); !slices.Equal(got, []string{"A"}) {
		t.Fatalf("ReadyNodes() = %v, want [A]", got)
	}

	wantInDegree := map[string]int{
		"A": 0,
		"B": 1,
		"C": 1,
		"D": 2,
	}
	if !reflect.DeepEqual(dag.InDegree, wantInDegree) {
		t.Fatalf("InDegree = %#v, want %#v", dag.InDegree, wantInDegree)
	}
}

func TestDAGValidateCycle(t *testing.T) {
	dag := Build([]core.TaskItem{
		task("A", "C"),
		task("B", "A"),
		task("C", "B"),
	})

	err := dag.Validate()
	assertDAGErrorCode(t, err, DAGCycleDetectedCode)
}

func TestDAGValidateMissingNode(t *testing.T) {
	dag := Build([]core.TaskItem{
		task("A", "Z"),
		task("B", "A"),
	})

	err := dag.Validate()
	dagErr := assertDAGErrorCode(t, err, DAGMissingNodeCode)
	if dagErr.NodeID != "A" || dagErr.DependencyID != "Z" {
		t.Fatalf("missing node detail mismatch: %+v", dagErr)
	}
}

func TestDAGValidateSelfDependency(t *testing.T) {
	dag := Build([]core.TaskItem{
		task("A", "A"),
		task("B", "A"),
	})

	err := dag.Validate()
	dagErr := assertDAGErrorCode(t, err, DAGSelfDependencyCode)
	if dagErr.NodeID != "A" || dagErr.DependencyID != "A" {
		t.Fatalf("self dependency detail mismatch: %+v", dagErr)
	}
}

func TestDAGEmptyGraph(t *testing.T) {
	dag := Build(nil)

	if err := dag.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if got := dag.ReadyNodes(); len(got) != 0 {
		t.Fatalf("ReadyNodes() = %v, want []", got)
	}
	if len(dag.Nodes) != 0 || len(dag.Downstream) != 0 || len(dag.InDegree) != 0 {
		t.Fatalf("empty DAG maps not initialized: Nodes=%d Downstream=%d InDegree=%d", len(dag.Nodes), len(dag.Downstream), len(dag.InDegree))
	}
}

func TestDAGBuildDeduplicatesEdges(t *testing.T) {
	dag := Build([]core.TaskItem{
		task("A"),
		task("B", "A", "A", "A"),
	})

	if err := dag.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if got := dag.Downstream["A"]; !slices.Equal(got, []string{"B"}) {
		t.Fatalf("Downstream[A] = %v, want [B]", got)
	}
	if got := dag.InDegree["B"]; got != 1 {
		t.Fatalf("InDegree[B] = %d, want 1", got)
	}
}

func TestDAGTransitiveReduceRemovesRedundantEdge(t *testing.T) {
	dag := Build([]core.TaskItem{
		task("A"),
		task("B", "A"),
		task("C", "A", "B"),
	})

	if err := dag.Validate(); err != nil {
		t.Fatalf("Validate() before reduce error = %v", err)
	}
	if got := dag.Downstream["A"]; !slices.Equal(got, []string{"B", "C"}) {
		t.Fatalf("Downstream[A] before reduce = %v, want [B C]", got)
	}

	dag.TransitiveReduce()

	if got := dag.Downstream["A"]; !slices.Equal(got, []string{"B"}) {
		t.Fatalf("Downstream[A] after reduce = %v, want [B]", got)
	}
	if got := dag.Downstream["B"]; !slices.Equal(got, []string{"C"}) {
		t.Fatalf("Downstream[B] after reduce = %v, want [C]", got)
	}
	if got := dag.InDegree["C"]; got != 1 {
		t.Fatalf("InDegree[C] after reduce = %d, want 1", got)
	}
	if err := dag.Validate(); err != nil {
		t.Fatalf("Validate() after reduce error = %v", err)
	}
}

func TestDAGReadyNodesSorted(t *testing.T) {
	dag := Build([]core.TaskItem{
		task("B"),
		task("A"),
		task("C", "A"),
	})

	if err := dag.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if got := dag.ReadyNodes(); !slices.Equal(got, []string{"A", "B"}) {
		t.Fatalf("ReadyNodes() = %v, want [A B]", got)
	}
}

func task(id string, dependsOn ...string) core.TaskItem {
	return core.TaskItem{
		ID:        id,
		DependsOn: dependsOn,
	}
}

func assertDAGErrorCode(t *testing.T, err error, wantCode string) *DAGError {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", wantCode)
	}

	var dagErr *DAGError
	if !errors.As(err, &dagErr) {
		t.Fatalf("expected DAGError, got %T: %v", err, err)
	}
	if dagErr.Code != wantCode {
		t.Fatalf("error code = %s, want %s", dagErr.Code, wantCode)
	}
	return dagErr
}
