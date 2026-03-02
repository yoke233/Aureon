package secretary

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yoke233/ai-workflow/internal/core"
)

const (
	DAGCycleDetectedCode  = "DAG_CYCLE_DETECTED"
	DAGMissingNodeCode    = "DAG_MISSING_NODE"
	DAGSelfDependencyCode = "DAG_SELF_DEPENDENCY"
)

type DAGError struct {
	Code         string
	NodeID       string
	DependencyID string
	Message      string
}

func (e *DAGError) Error() string {
	if e == nil {
		return ""
	}

	parts := []string{e.Code}
	if e.NodeID != "" {
		parts = append(parts, fmt.Sprintf("node=%s", e.NodeID))
	}
	if e.DependencyID != "" {
		parts = append(parts, fmt.Sprintf("dependency=%s", e.DependencyID))
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, " ")
}

type DAG struct {
	Nodes      map[string]*core.TaskItem
	Downstream map[string][]string
	InDegree   map[string]int
}

func Build(tasks []core.TaskItem) *DAG {
	d := &DAG{
		Nodes:      make(map[string]*core.TaskItem, len(tasks)),
		Downstream: make(map[string][]string, len(tasks)),
		InDegree:   make(map[string]int, len(tasks)),
	}

	for _, task := range tasks {
		item := task
		d.Nodes[item.ID] = &item
		d.Downstream[item.ID] = []string{}
		d.InDegree[item.ID] = 0
	}

	for _, taskID := range sortedNodeIDs(d.Nodes) {
		task := d.Nodes[taskID]
		seenDeps := make(map[string]struct{}, len(task.DependsOn))
		for _, depID := range task.DependsOn {
			if _, duplicated := seenDeps[depID]; duplicated {
				continue
			}
			seenDeps[depID] = struct{}{}

			if depID == taskID {
				continue
			}
			if _, exists := d.Nodes[depID]; !exists {
				continue
			}

			d.Downstream[depID] = append(d.Downstream[depID], taskID)
			d.InDegree[taskID]++
		}
	}

	for _, nodeID := range sortedNodeIDs(d.Nodes) {
		sort.Strings(d.Downstream[nodeID])
	}

	return d
}

func (d *DAG) Validate() error {
	if d == nil {
		return nil
	}

	if d.Nodes == nil {
		d.Nodes = map[string]*core.TaskItem{}
	}
	if d.Downstream == nil {
		d.Downstream = map[string][]string{}
	}
	if d.InDegree == nil {
		d.InDegree = map[string]int{}
	}

	for _, taskID := range sortedNodeIDs(d.Nodes) {
		task := d.Nodes[taskID]
		if task == nil {
			continue
		}

		seenDeps := make(map[string]struct{}, len(task.DependsOn))
		for _, depID := range task.DependsOn {
			if _, duplicated := seenDeps[depID]; duplicated {
				continue
			}
			seenDeps[depID] = struct{}{}

			if depID == taskID {
				return &DAGError{
					Code:         DAGSelfDependencyCode,
					NodeID:       taskID,
					DependencyID: depID,
					Message:      "task cannot depend on itself",
				}
			}
			if _, exists := d.Nodes[depID]; !exists {
				return &DAGError{
					Code:         DAGMissingNodeCode,
					NodeID:       taskID,
					DependencyID: depID,
					Message:      "dependency node does not exist",
				}
			}
		}
	}

	normalizedDownstream, err := d.normalizeDownstream()
	if err != nil {
		return err
	}

	initialInDegree := buildInDegree(d.Nodes, normalizedDownstream)
	workingInDegree := copyInDegree(initialInDegree)

	ready := make([]string, 0, len(workingInDegree))
	for taskID, deg := range workingInDegree {
		if deg == 0 {
			ready = append(ready, taskID)
		}
	}
	sort.Strings(ready)

	visited := 0
	for len(ready) > 0 {
		current := ready[0]
		ready = ready[1:]
		visited++

		for _, next := range normalizedDownstream[current] {
			workingInDegree[next]--
			if workingInDegree[next] == 0 {
				ready = append(ready, next)
			}
		}
		sort.Strings(ready)
	}

	if visited != len(d.Nodes) {
		return &DAGError{
			Code:    DAGCycleDetectedCode,
			Message: "cycle detected in task dependency graph",
		}
	}

	d.Downstream = normalizedDownstream
	d.InDegree = initialInDegree
	return nil
}

func (d *DAG) ReadyNodes() []string {
	if d == nil || len(d.Nodes) == 0 {
		return nil
	}

	ready := make([]string, 0, len(d.Nodes))
	for _, taskID := range sortedNodeIDs(d.Nodes) {
		if d.InDegree[taskID] == 0 {
			ready = append(ready, taskID)
		}
	}
	return ready
}

func (d *DAG) TransitiveReduce() {
	if d == nil || len(d.Nodes) == 0 {
		return
	}

	normalizedDownstream, err := d.normalizeDownstream()
	if err != nil {
		return
	}

	adjacencySet := make(map[string]map[string]struct{}, len(d.Nodes))
	for _, nodeID := range sortedNodeIDs(d.Nodes) {
		adjacencySet[nodeID] = map[string]struct{}{}
	}
	for from, downstream := range normalizedDownstream {
		for _, to := range downstream {
			adjacencySet[from][to] = struct{}{}
		}
	}

	for _, from := range sortedNodeIDs(d.Nodes) {
		targets := setKeys(adjacencySet[from])
		for _, to := range targets {
			delete(adjacencySet[from], to)
			if !reachable(adjacencySet, from, to) {
				adjacencySet[from][to] = struct{}{}
			}
		}
	}

	reducedDownstream := make(map[string][]string, len(d.Nodes))
	for _, nodeID := range sortedNodeIDs(d.Nodes) {
		reducedDownstream[nodeID] = setKeys(adjacencySet[nodeID])
	}

	d.Downstream = reducedDownstream
	d.InDegree = buildInDegree(d.Nodes, reducedDownstream)
}

func (d *DAG) normalizeDownstream() (map[string][]string, error) {
	normalized := make(map[string][]string, len(d.Nodes))
	for _, nodeID := range sortedNodeIDs(d.Nodes) {
		normalized[nodeID] = []string{}
	}

	for from, downstream := range d.Downstream {
		if _, exists := d.Nodes[from]; !exists {
			return nil, &DAGError{
				Code:         DAGMissingNodeCode,
				NodeID:       from,
				DependencyID: from,
				Message:      "source node in downstream map does not exist",
			}
		}

		seen := make(map[string]struct{}, len(downstream))
		for _, to := range downstream {
			if _, duplicated := seen[to]; duplicated {
				continue
			}
			seen[to] = struct{}{}

			if to == from {
				return nil, &DAGError{
					Code:         DAGSelfDependencyCode,
					NodeID:       from,
					DependencyID: to,
					Message:      "task cannot depend on itself",
				}
			}
			if _, exists := d.Nodes[to]; !exists {
				return nil, &DAGError{
					Code:         DAGMissingNodeCode,
					NodeID:       from,
					DependencyID: to,
					Message:      "dependency node in downstream map does not exist",
				}
			}

			normalized[from] = append(normalized[from], to)
		}
	}

	for _, nodeID := range sortedNodeIDs(d.Nodes) {
		sort.Strings(normalized[nodeID])
	}
	return normalized, nil
}

func buildInDegree(nodes map[string]*core.TaskItem, downstream map[string][]string) map[string]int {
	inDegree := make(map[string]int, len(nodes))
	for nodeID := range nodes {
		inDegree[nodeID] = 0
	}

	for _, edges := range downstream {
		for _, to := range edges {
			inDegree[to]++
		}
	}
	return inDegree
}

func copyInDegree(inDegree map[string]int) map[string]int {
	dup := make(map[string]int, len(inDegree))
	for taskID, deg := range inDegree {
		dup[taskID] = deg
	}
	return dup
}

func reachable(adjacency map[string]map[string]struct{}, from, target string) bool {
	if from == target {
		return true
	}

	visited := map[string]struct{}{from: {}}
	stack := []string{from}

	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		for next := range adjacency[current] {
			if next == target {
				return true
			}
			if _, seen := visited[next]; seen {
				continue
			}
			visited[next] = struct{}{}
			stack = append(stack, next)
		}
	}
	return false
}

func sortedNodeIDs(nodes map[string]*core.TaskItem) []string {
	ids := make([]string, 0, len(nodes))
	for taskID := range nodes {
		ids = append(ids, taskID)
	}
	sort.Strings(ids)
	return ids
}

func setKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
