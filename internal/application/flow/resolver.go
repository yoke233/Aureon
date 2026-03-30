package flow

import (
	"context"
	"strings"

	"github.com/yoke233/zhanggui/internal/core"
)

// ProfileRegistry is a Resolver backed by a static list of AgentProfiles.
type ProfileRegistry struct {
	profiles []*core.AgentProfile
}

// NewProfileRegistry creates a Resolver from a set of agent profiles.
func NewProfileRegistry(profiles []*core.AgentProfile) *ProfileRegistry {
	return &ProfileRegistry{profiles: profiles}
}

func preferredProfileID(action *core.Action) string {
	if action == nil || action.Config == nil {
		return ""
	}
	raw, _ := action.Config["preferred_profile_id"].(string)
	return strings.TrimSpace(raw)
}

// Resolve picks the first profile that matches the action's AgentRole and RequiredCapabilities.
func (r *ProfileRegistry) Resolve(_ context.Context, action *core.Action) (string, error) {
	if action == nil {
		return "", core.ErrNoMatchingAgent
	}
	if preferred := preferredProfileID(action); preferred != "" {
		for _, p := range r.profiles {
			if p != nil && p.ID == preferred {
				return p.ID, nil
			}
		}
	}
	role := core.AgentRole(action.AgentRole)
	for _, p := range r.profiles {
		if p == nil {
			continue
		}
		if role != "" && p.Role != role {
			continue
		}
		if !p.MatchesRequirements(action.RequiredCapabilities) {
			continue
		}
		return p.ID, nil
	}
	return "", core.ErrNoMatchingAgent
}
