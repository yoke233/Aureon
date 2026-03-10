package core

// AgentRole is the role classification for an agent.
type AgentRole string

const (
	RoleLead    AgentRole = "lead"
	RoleWorker  AgentRole = "worker"
	RoleGate    AgentRole = "gate"
	RoleSupport AgentRole = "support"
)

// Action represents an operation an agent can perform.
type Action string

const (
	ActionReadContext  Action = "read_context"
	ActionSearchFiles  Action = "search_files"
	ActionFSWrite      Action = "fs_write"
	ActionTerminal     Action = "terminal"
	ActionSubmit       Action = "submit"
	ActionMarkBlocked  Action = "mark_blocked"
	ActionRequestHelp  Action = "request_help"
	ActionApprove      Action = "approve"
	ActionReject       Action = "reject"
	ActionCreateStep   Action = "create_step"
	ActionExpandFlow   Action = "expand_flow"
)

// AgentProfile defines an agent's identity, capabilities, and constraints.
type AgentProfile struct {
	ID              string    `json:"id"`
	Role            AgentRole `json:"role"`
	Driver          string    `json:"driver"`                    // claude | codex | human
	LaunchCommand   string    `json:"launch_command,omitempty"`
	LaunchArgs      []string  `json:"launch_args,omitempty"`
	Capabilities    []string  `json:"capabilities,omitempty"`    // capability tags (dev.backend, test.qa, ...)
	ActionsAllowed  []Action  `json:"actions_allowed,omitempty"` // permitted actions
}

// DefaultActions returns the default action whitelist for a role.
func DefaultActions(role AgentRole) []Action {
	common := []Action{ActionReadContext, ActionSearchFiles, ActionSubmit, ActionMarkBlocked, ActionRequestHelp}
	switch role {
	case RoleLead:
		return append(common, ActionFSWrite, ActionTerminal, ActionCreateStep, ActionExpandFlow)
	case RoleWorker:
		return append(common, ActionFSWrite, ActionTerminal)
	case RoleGate:
		return append(common, ActionApprove, ActionReject)
	case RoleSupport:
		return common
	default:
		return common
	}
}

// HasAction checks if the profile permits the given action.
func (p *AgentProfile) HasAction(action Action) bool {
	actions := p.ActionsAllowed
	if len(actions) == 0 {
		actions = DefaultActions(p.Role)
	}
	for _, a := range actions {
		if a == action {
			return true
		}
	}
	return false
}

// HasCapability checks if the profile has the given capability tag.
func (p *AgentProfile) HasCapability(cap string) bool {
	for _, c := range p.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// MatchesRequirements checks if the profile satisfies all required capability tags.
func (p *AgentProfile) MatchesRequirements(required []string) bool {
	for _, req := range required {
		if !p.HasCapability(req) {
			return false
		}
	}
	return true
}
