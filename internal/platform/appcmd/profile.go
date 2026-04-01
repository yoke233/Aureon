package appcmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yoke233/zhanggui/internal/adapters/store/sqlite"
	"github.com/yoke233/zhanggui/internal/core"
	"github.com/yoke233/zhanggui/internal/platform/bootstrap"
	"github.com/yoke233/zhanggui/internal/platform/config"
	"github.com/yoke233/zhanggui/internal/platform/configruntime"
	"github.com/yoke233/zhanggui/internal/platform/profilellm"
	"github.com/yoke233/zhanggui/internal/skills"
)

type profileCLIOptions struct {
	Action string
	ID     string
	From   string
	Name   string
	Role   string
	Driver string
	LLM    string
	Prompt string
	Skill  string
}

type profileResult struct {
	OK      bool                 `json:"ok"`
	Action  string               `json:"action"`
	Summary string               `json:"summary"`
	Profile *core.AgentProfile   `json:"profile,omitempty"`
	List    []*core.AgentProfile `json:"profiles,omitempty"`
}

type profileRuntime struct {
	manager *configruntime.Manager
	store   *sqlite.Store
	close   func() error
}

var newProfileRuntime = defaultNewProfileRuntime

func RunProfile(args []string) error {
	runtime, err := newProfileRuntime()
	if err != nil {
		return err
	}
	if runtime != nil && runtime.close != nil {
		defer runtime.close()
	}
	return runProfileToWriter(os.Stdout, runtime, args)
}

func runProfileToWriter(out io.Writer, runtime *profileRuntime, args []string) error {
	opts, err := parseProfileArgs(args)
	if err != nil {
		return err
	}
	if runtime == nil || runtime.manager == nil || runtime.store == nil {
		return fmt.Errorf("profile runtime is not configured")
	}
	if out == nil {
		out = io.Discard
	}

	result, err := executeProfileAction(context.Background(), runtime, opts)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return enc.Encode(result)
}

func parseProfileArgs(args []string) (profileCLIOptions, error) {
	if len(args) == 0 {
		return profileCLIOptions{}, fmt.Errorf("usage: ai-flow profile <list|get|create|set-base|add-skill|remove-skill|delete> [flags]")
	}

	switch strings.TrimSpace(args[0]) {
	case "list":
		fs := flag.NewFlagSet("profile list", flag.ContinueOnError)
		if err := fs.Parse(args[1:]); err != nil {
			return profileCLIOptions{}, err
		}
		if fs.NArg() != 0 {
			return profileCLIOptions{}, fmt.Errorf("profile list does not accept positional arguments")
		}
		return profileCLIOptions{Action: "list"}, nil
	case "get":
		fs := flag.NewFlagSet("profile get", flag.ContinueOnError)
		var id string
		fs.StringVar(&id, "id", "", "Profile ID")
		if err := fs.Parse(args[1:]); err != nil {
			return profileCLIOptions{}, err
		}
		if fs.NArg() != 0 {
			return profileCLIOptions{}, fmt.Errorf("profile get does not accept positional arguments")
		}
		if strings.TrimSpace(id) == "" {
			return profileCLIOptions{}, fmt.Errorf("--id is required")
		}
		return profileCLIOptions{Action: "get", ID: strings.TrimSpace(id)}, nil
	case "create":
		fs := flag.NewFlagSet("profile create", flag.ContinueOnError)
		var opts profileCLIOptions
		fs.StringVar(&opts.From, "from", "", "Base profile template source")
		fs.StringVar(&opts.ID, "id", "", "Profile ID")
		fs.StringVar(&opts.Name, "name", "", "Profile display name")
		fs.StringVar(&opts.Role, "role", "", "Profile role: lead|worker|gate|support")
		fs.StringVar(&opts.Driver, "driver", "", "Driver ID")
		fs.StringVar(&opts.LLM, "llm", "", "LLM config ID")
		fs.StringVar(&opts.Prompt, "prompt", "", "Prompt template")
		if err := fs.Parse(args[1:]); err != nil {
			return profileCLIOptions{}, err
		}
		if fs.NArg() != 0 {
			return profileCLIOptions{}, fmt.Errorf("profile create does not accept positional arguments")
		}
		if strings.TrimSpace(opts.ID) == "" {
			return profileCLIOptions{}, fmt.Errorf("--id is required")
		}
		if strings.TrimSpace(opts.Role) == "" {
			return profileCLIOptions{}, fmt.Errorf("--role is required")
		}
		opts.Action = "create"
		return opts, nil
	case "set-base":
		fs := flag.NewFlagSet("profile set-base", flag.ContinueOnError)
		var opts profileCLIOptions
		fs.StringVar(&opts.ID, "id", "", "Profile ID")
		fs.StringVar(&opts.Name, "name", "", "Profile display name")
		fs.StringVar(&opts.Role, "role", "", "Profile role: lead|worker|gate|support")
		fs.StringVar(&opts.Driver, "driver", "", "Driver ID")
		fs.StringVar(&opts.LLM, "llm", "", "LLM config ID")
		fs.StringVar(&opts.Prompt, "prompt", "", "Prompt template")
		if err := fs.Parse(args[1:]); err != nil {
			return profileCLIOptions{}, err
		}
		if fs.NArg() != 0 {
			return profileCLIOptions{}, fmt.Errorf("profile set-base does not accept positional arguments")
		}
		if strings.TrimSpace(opts.ID) == "" {
			return profileCLIOptions{}, fmt.Errorf("--id is required")
		}
		opts.Action = "set-base"
		return opts, nil
	case "add-skill":
		fs := flag.NewFlagSet("profile add-skill", flag.ContinueOnError)
		var opts profileCLIOptions
		fs.StringVar(&opts.ID, "id", "", "Profile ID")
		fs.StringVar(&opts.Skill, "skill", "", "Skill name")
		if err := fs.Parse(args[1:]); err != nil {
			return profileCLIOptions{}, err
		}
		if fs.NArg() != 0 {
			return profileCLIOptions{}, fmt.Errorf("profile add-skill does not accept positional arguments")
		}
		if strings.TrimSpace(opts.ID) == "" {
			return profileCLIOptions{}, fmt.Errorf("--id is required")
		}
		if strings.TrimSpace(opts.Skill) == "" {
			return profileCLIOptions{}, fmt.Errorf("--skill is required")
		}
		opts.Action = "add-skill"
		return opts, nil
	case "remove-skill":
		fs := flag.NewFlagSet("profile remove-skill", flag.ContinueOnError)
		var opts profileCLIOptions
		fs.StringVar(&opts.ID, "id", "", "Profile ID")
		fs.StringVar(&opts.Skill, "skill", "", "Skill name")
		if err := fs.Parse(args[1:]); err != nil {
			return profileCLIOptions{}, err
		}
		if fs.NArg() != 0 {
			return profileCLIOptions{}, fmt.Errorf("profile remove-skill does not accept positional arguments")
		}
		if strings.TrimSpace(opts.ID) == "" {
			return profileCLIOptions{}, fmt.Errorf("--id is required")
		}
		if strings.TrimSpace(opts.Skill) == "" {
			return profileCLIOptions{}, fmt.Errorf("--skill is required")
		}
		opts.Action = "remove-skill"
		return opts, nil
	case "delete":
		fs := flag.NewFlagSet("profile delete", flag.ContinueOnError)
		var id string
		fs.StringVar(&id, "id", "", "Profile ID")
		if err := fs.Parse(args[1:]); err != nil {
			return profileCLIOptions{}, err
		}
		if fs.NArg() != 0 {
			return profileCLIOptions{}, fmt.Errorf("profile delete does not accept positional arguments")
		}
		if strings.TrimSpace(id) == "" {
			return profileCLIOptions{}, fmt.Errorf("--id is required")
		}
		return profileCLIOptions{Action: "delete", ID: strings.TrimSpace(id)}, nil
	default:
		return profileCLIOptions{}, fmt.Errorf("usage: ai-flow profile <list|get|create|set-base|add-skill|remove-skill|delete> [flags]")
	}
}

func executeProfileAction(ctx context.Context, runtime *profileRuntime, opts profileCLIOptions) (profileResult, error) {
	switch opts.Action {
	case "list":
		items, err := runtime.store.ListProfiles(ctx)
		if err != nil {
			return profileResult{}, err
		}
		return profileResult{
			OK:      true,
			Action:  opts.Action,
			Summary: "listed runtime profiles",
			List:    items,
		}, nil
	case "get":
		profile, err := runtime.store.GetProfile(ctx, strings.TrimSpace(opts.ID))
		if err != nil {
			return profileResult{}, err
		}
		return profileResult{
			OK:      true,
			Action:  opts.Action,
			Summary: "loaded runtime profile",
			Profile: profile,
		}, nil
	case "create":
		return executeCreateProfile(ctx, runtime, opts)
	case "set-base":
		return executeSetBaseProfile(ctx, runtime, opts)
	case "add-skill":
		return executeAddSkill(ctx, runtime, opts)
	case "remove-skill":
		return executeRemoveSkill(ctx, runtime, opts)
	case "delete":
		return executeDeleteProfile(ctx, runtime, opts)
	default:
		return profileResult{}, fmt.Errorf("unsupported profile action: %s", opts.Action)
	}
}

func executeCreateProfile(ctx context.Context, runtime *profileRuntime, opts profileCLIOptions) (profileResult, error) {
	if strings.TrimSpace(opts.From) != "ceo" {
		return profileResult{}, fmt.Errorf("profile create currently only supports --from ceo")
	}
	ceo, err := runtime.store.GetProfile(ctx, "ceo")
	if err != nil {
		return profileResult{}, fmt.Errorf("load ceo profile: %w", err)
	}
	role, err := parseAgentRole(opts.Role)
	if err != nil {
		return profileResult{}, err
	}
	profileCfg := buildManagedProfileConfigWithName(
		strings.TrimSpace(opts.ID),
		firstNonEmpty(opts.Name, defaultProfileName(role)),
		role,
		"ceo",
		firstNonEmpty(opts.Driver, ceo.DriverID),
		firstNonEmpty(opts.LLM, profilellm.LLMConfigIDSystem),
	)
	if prompt := strings.TrimSpace(opts.Prompt); prompt != "" {
		profileCfg.PromptTemplate = prompt
	}
	profile, err := upsertProfileConfig(ctx, runtime, profileCfg, false)
	if err != nil {
		return profileResult{}, err
	}
	return profileResult{
		OK:      true,
		Action:  opts.Action,
		Summary: "created runtime profile",
		Profile: profile,
	}, nil
}

func executeSetBaseProfile(ctx context.Context, runtime *profileRuntime, opts profileCLIOptions) (profileResult, error) {
	current, err := runtime.store.GetProfile(ctx, strings.TrimSpace(opts.ID))
	if err != nil {
		return profileResult{}, err
	}

	currentCfg := configruntime.CoreProfileToRuntimeConfig(current)
	nextCfg := currentCfg
	nextCfg.Name = firstNonEmpty(opts.Name, currentCfg.Name)
	nextCfg.Driver = firstNonEmpty(opts.Driver, currentCfg.Driver)
	nextCfg.LLMConfigID = firstNonEmpty(opts.LLM, currentCfg.LLMConfigID, profilellm.LLMConfigIDSystem)

	roleChanged := false
	if roleValue := strings.TrimSpace(opts.Role); roleValue != "" {
		role, parseErr := parseAgentRole(roleValue)
		if parseErr != nil {
			return profileResult{}, parseErr
		}
		if current.Role != role {
			roleChanged = true
			roleCfg := buildManagedProfileConfigWithName(
				current.ID,
				nextCfg.Name,
				role,
				current.ManagerProfileID,
				nextCfg.Driver,
				nextCfg.LLMConfigID,
			)
			roleCfg.Skills = append([]string(nil), currentCfg.Skills...)
			roleCfg.MCP = currentCfg.MCP
			nextCfg = roleCfg
		} else {
			nextCfg.Role = string(role)
		}
	}
	if prompt := strings.TrimSpace(opts.Prompt); prompt != "" {
		nextCfg.PromptTemplate = prompt
	} else if roleChanged {
		nextCfg.PromptTemplate = nextCfg.PromptTemplate
	}

	profile, err := upsertProfileConfig(ctx, runtime, nextCfg, true)
	if err != nil {
		return profileResult{}, err
	}
	return profileResult{
		OK:      true,
		Action:  opts.Action,
		Summary: "updated profile base config",
		Profile: profile,
	}, nil
}

func executeAddSkill(ctx context.Context, runtime *profileRuntime, opts profileCLIOptions) (profileResult, error) {
	current, err := runtime.store.GetProfile(ctx, strings.TrimSpace(opts.ID))
	if err != nil {
		return profileResult{}, err
	}
	nextCfg := configruntime.CoreProfileToRuntimeConfig(current)
	nextCfg.Skills = appendUniqueString(nextCfg.Skills, strings.TrimSpace(opts.Skill))
	profile, err := upsertProfileConfig(ctx, runtime, nextCfg, true)
	if err != nil {
		return profileResult{}, err
	}
	return profileResult{
		OK:      true,
		Action:  opts.Action,
		Summary: "added profile skill",
		Profile: profile,
	}, nil
}

func executeRemoveSkill(ctx context.Context, runtime *profileRuntime, opts profileCLIOptions) (profileResult, error) {
	current, err := runtime.store.GetProfile(ctx, strings.TrimSpace(opts.ID))
	if err != nil {
		return profileResult{}, err
	}
	nextCfg := configruntime.CoreProfileToRuntimeConfig(current)
	nextCfg.Skills = removeString(nextCfg.Skills, strings.TrimSpace(opts.Skill))
	profile, err := upsertProfileConfig(ctx, runtime, nextCfg, true)
	if err != nil {
		return profileResult{}, err
	}
	return profileResult{
		OK:      true,
		Action:  opts.Action,
		Summary: "removed profile skill",
		Profile: profile,
	}, nil
}

func executeDeleteProfile(ctx context.Context, runtime *profileRuntime, opts profileCLIOptions) (profileResult, error) {
	profileID := strings.TrimSpace(opts.ID)
	if profileID == "ceo" {
		return profileResult{}, fmt.Errorf("profile %q is protected", profileID)
	}
	if _, err := runtime.manager.DeleteProfileConfig(ctx, profileID); err != nil {
		return profileResult{}, err
	}
	if err := runtime.store.DeleteProfile(ctx, profileID); err != nil && !errors.Is(err, core.ErrProfileNotFound) {
		return profileResult{}, err
	}
	return profileResult{
		OK:      true,
		Action:  opts.Action,
		Summary: "deleted runtime profile",
	}, nil
}

func defaultNewProfileRuntime() (*profileRuntime, error) {
	cfg, dataDir, _, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	skillsRoot := filepath.Join(dataDir, "skills")
	if err := skills.EnsureBuiltinSkills(skillsRoot); err != nil {
		return nil, fmt.Errorf("ensure builtin skills: %w", err)
	}

	configPath := resolveGlobalConfigFilePath(dataDir)
	secretsPath := resolveSecretsFilePath(dataDir)
	manager, err := configruntime.NewManager(configPath, secretsPath, configruntime.DisabledMCPEnv(), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("open runtime manager: %w", err)
	}

	storePath := ExpandStorePath(cfg.Store.Path, dataDir)
	runtimeDBPath := strings.TrimSuffix(storePath, filepath.Ext(storePath)) + "_runtime.db"
	store, err := sqlite.New(runtimeDBPath)
	if err != nil {
		return nil, fmt.Errorf("open runtime store: %w", err)
	}

	bootstrap.SeedRegistry(context.Background(), store, cfg)
	return &profileRuntime{
		manager: manager,
		store:   store,
		close:   store.Close,
	}, nil
}

func upsertProfileConfig(ctx context.Context, runtime *profileRuntime, profileCfg config.RuntimeProfileConfig, update bool) (*core.AgentProfile, error) {
	if runtime == nil || runtime.manager == nil || runtime.store == nil {
		return nil, fmt.Errorf("profile runtime is not configured")
	}
	var err error
	if update {
		_, err = runtime.manager.UpdateProfileConfig(ctx, profileCfg.ID, profileCfg)
	} else {
		_, err = runtime.manager.CreateProfileConfig(ctx, profileCfg)
	}
	if err != nil {
		return nil, err
	}

	snap := runtime.manager.Current()
	if snap == nil || snap.Config == nil {
		return nil, fmt.Errorf("runtime config unavailable after profile update")
	}
	for _, profile := range configruntime.BuildAgents(snap.Config) {
		if profile == nil || strings.TrimSpace(profile.ID) != profileCfg.ID {
			continue
		}
		if err := runtime.store.UpsertProfile(ctx, profile); err != nil {
			return nil, err
		}
		return runtime.store.GetProfile(ctx, profileCfg.ID)
	}
	return nil, fmt.Errorf("profile %q missing from runtime config after update", profileCfg.ID)
}

func parseAgentRole(raw string) (core.AgentRole, error) {
	switch core.AgentRole(strings.TrimSpace(raw)) {
	case core.RoleLead, core.RoleWorker, core.RoleGate, core.RoleSupport:
		return core.AgentRole(strings.TrimSpace(raw)), nil
	default:
		return "", fmt.Errorf("invalid role %q (expected lead|worker|gate|support)", raw)
	}
}

func buildManagedProfileConfig(profileID string, role core.AgentRole, managerProfileID string, driverID string, llmConfigID string) config.RuntimeProfileConfig {
	return buildManagedProfileConfigWithName(profileID, defaultProfileName(role), role, managerProfileID, driverID, llmConfigID)
}

func buildManagedProfileConfigWithName(profileID string, name string, role core.AgentRole, managerProfileID string, driverID string, llmConfigID string) config.RuntimeProfileConfig {
	profile := config.RuntimeProfileConfig{
		ID:               strings.TrimSpace(profileID),
		Name:             firstNonEmpty(name, defaultProfileName(role)),
		ManagerProfileID: strings.TrimSpace(managerProfileID),
		Driver:           strings.TrimSpace(driverID),
		LLMConfigID:      firstNonEmpty(llmConfigID, profilellm.LLMConfigIDSystem),
		Role:             string(role),
		Capabilities:     defaultProfileCapabilities(role),
		ActionsAllowed:   agentActionsToStrings(core.DefaultAgentActions(role)),
		PromptTemplate:   defaultProfilePrompt(role),
		Session:          defaultProfileSession(role),
		MCP:              defaultProfileMCP(role),
	}
	return profile
}

func defaultProfileName(role core.AgentRole) string {
	switch role {
	case core.RoleLead:
		return "Lead Agent"
	case core.RoleWorker:
		return "Worker Agent"
	case core.RoleGate:
		return "Reviewer Agent"
	case core.RoleSupport:
		return "Support Agent"
	default:
		return "Agent"
	}
}

func defaultProfileCapabilities(role core.AgentRole) []string {
	switch role {
	case core.RoleLead:
		return []string{"planning", "review", "fullstack"}
	case core.RoleWorker:
		return []string{"backend", "frontend", "test"}
	case core.RoleGate:
		return []string{"review"}
	case core.RoleSupport:
		return []string{"planning", "analysis"}
	default:
		return nil
	}
}

func defaultProfilePrompt(role core.AgentRole) string {
	switch role {
	case core.RoleLead:
		return "team_leader"
	case core.RoleWorker:
		return "implement"
	case core.RoleGate:
		return "review"
	case core.RoleSupport:
		return "plan_parser"
	default:
		return ""
	}
}

func defaultProfileSession(role core.AgentRole) config.RuntimeSessionConfig {
	switch role {
	case core.RoleLead:
		return config.RuntimeSessionConfig{
			Reuse:    true,
			MaxTurns: 50,
			IdleTTL:  config.Duration{Duration: 30 * time.Minute},
		}
	case core.RoleWorker:
		return config.RuntimeSessionConfig{
			Reuse:    true,
			MaxTurns: 24,
			IdleTTL:  config.Duration{Duration: 15 * time.Minute},
		}
	case core.RoleGate:
		return config.RuntimeSessionConfig{
			Reuse:    true,
			MaxTurns: 16,
			IdleTTL:  config.Duration{Duration: 15 * time.Minute},
		}
	case core.RoleSupport:
		return config.RuntimeSessionConfig{
			Reuse:    true,
			MaxTurns: 12,
			IdleTTL:  config.Duration{Duration: 10 * time.Minute},
		}
	default:
		return config.RuntimeSessionConfig{}
	}
}

func defaultProfileMCP(role core.AgentRole) config.MCPConfig {
	switch role {
	case core.RoleLead:
		return config.MCPConfig{Enabled: true}
	default:
		return config.MCPConfig{}
	}
}

func agentActionsToStrings(actions []core.AgentAction) []string {
	if len(actions) == 0 {
		return nil
	}
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		out = append(out, string(action))
	}
	return out
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return append([]string(nil), items...)
	}
	out := append([]string(nil), items...)
	for _, item := range out {
		if strings.TrimSpace(item) == value {
			return out
		}
	}
	return append(out, value)
}

func removeString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || len(items) == 0 {
		return append([]string(nil), items...)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			continue
		}
		out = append(out, item)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
