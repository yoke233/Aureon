package workitemapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/yoke233/zhanggui/internal/core"
)

const HumanEscalationTarget = "human"

func BuildEscalationPath(ctx context.Context, activeProfileID string, registry core.AgentRegistry) ([]string, error) {
	if registry == nil {
		return nil, fmt.Errorf("agent registry is required")
	}
	current := strings.TrimSpace(activeProfileID)
	if current == "" {
		return nil, fmt.Errorf("active_profile_id is required")
	}

	seen := map[string]struct{}{current: {}}
	path := make([]string, 0, 4)
	for {
		profile, err := registry.ResolveByID(ctx, current)
		if err != nil {
			return nil, err
		}
		managerID := strings.TrimSpace(profile.ManagerProfileID)
		if managerID == "" {
			break
		}
		if _, exists := seen[managerID]; exists {
			return nil, fmt.Errorf("manager cycle detected at profile %q", managerID)
		}
		path = append(path, managerID)
		seen[managerID] = struct{}{}
		current = managerID
	}
	path = append(path, HumanEscalationTarget)
	return path, nil
}

func DefaultReviewerProfileID(ctx context.Context, executorProfileID string, registry core.AgentRegistry) (string, error) {
	if registry == nil {
		return "", fmt.Errorf("agent registry is required")
	}
	profile, err := registry.ResolveByID(ctx, strings.TrimSpace(executorProfileID))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(profile.ManagerProfileID), nil
}
