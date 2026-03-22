export interface DriverCapabilities {
  fs_read: boolean;
  fs_write: boolean;
  terminal: boolean;
}

export interface DriverConfig {
  id: string;
  launch_command: string;
  launch_args?: string[];
  env?: Record<string, string>;
  capabilities_max: DriverCapabilities;
}

export interface AgentProfileSession {
  reuse?: boolean;
  max_turns?: number;
  idle_ttl?: string;
}

export interface AgentProfileMCP {
  enabled?: boolean;
  tools?: string[];
}

export interface AgentProfile {
  id: string;
  name?: string;
  driver_id?: string;
  llm_config_id?: string;
  driver?: DriverConfig;
  role: "lead" | "worker" | "gate" | "support" | string;
  capabilities?: string[];
  actions_allowed?: string[];
  prompt_template?: string;
  skills?: string[];
  session?: AgentProfileSession;
  mcp?: AgentProfileMCP;
}

export interface ConfigOptionValue {
  value: string;
  name: string;
  description?: string;
  group_id?: string;
  group_name?: string;
}

export interface ConfigOption {
  id: string;
  name: string;
  description?: string;
  category?: string;
  type: "select" | string;
  current_value: string;
  options: ConfigOptionValue[];
}

export interface SessionMode {
  id: string;
  name: string;
  description?: string;
}

export interface SessionModeState {
  available_modes: SessionMode[];
  current_mode_id: string;
}

export interface SlashCommandInput {
  hint?: string;
}

export interface SlashCommand {
  name: string;
  description?: string;
  input?: SlashCommandInput;
}

export interface SkillMetadata {
  name: string;
  description: string;
}

export interface SkillInfo {
  name: string;
  has_skill_md: boolean;
  valid: boolean;
  metadata?: SkillMetadata;
  validation_errors?: string[];
  profiles_using?: string[];
}

export interface SkillDetail extends SkillInfo {
  skill_md: string;
}

export interface CreateSkillRequest {
  name: string;
  skill_md?: string;
}

export interface ImportGitHubSkillRequest {
  repo_url: string;
  skill_name: string;
}

export interface SchedulerStats {
  enabled: boolean;
  message?: string;
  stats?: Record<string, unknown>;
}

