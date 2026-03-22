import type {
  AdminSystemEventRequest,
  AdminSystemEventResponse,
  AgentProfile,
  CreateSkillRequest,
  DriverConfig,
  ImportGitHubSkillRequest,
  SchedulerStats,
  SkillDetail,
  SkillInfo,
  StatsResponse,
} from "../types/apiV2";
import type {
  LLMConfigResponse,
  SandboxSupportResponse,
  UpdateLLMConfigRequest,
  UpdateSandboxSupportRequest,
} from "../types/system";
import type { ApiClient } from "./apiClient";
import type { ApiBuilderContext } from "./apiClient.shared";

export const buildAgentAdminApi = ({
  request,
}: ApiBuilderContext): Pick<
  ApiClient,
  | "getStats"
  | "getSchedulerStats"
  | "getSandboxSupport"
  | "updateSandboxSupport"
  | "getLLMConfig"
  | "updateLLMConfig"
  | "sendSystemEvent"
  | "listProfiles"
  | "createProfile"
  | "updateProfile"
  | "deleteProfile"
  | "listDrivers"
  | "createDriver"
  | "updateDriver"
  | "deleteDriver"
  | "listSkills"
  | "getSkill"
  | "createSkill"
  | "updateSkill"
  | "deleteSkill"
  | "importGitHubSkill"
> => ({
  getStats: () =>
    request<StatsResponse>({
      path: "/stats",
    }),
  getSchedulerStats: () =>
    request<SchedulerStats>({
      path: "/scheduler/stats",
    }),
  getSandboxSupport: () =>
    request<SandboxSupportResponse>({
      path: "/system/sandbox-support",
    }),
  updateSandboxSupport: (body) =>
    request<SandboxSupportResponse, UpdateSandboxSupportRequest>({
      path: "/admin/system/sandbox-support",
      method: "PUT",
      body,
    }),
  getLLMConfig: () =>
    request<LLMConfigResponse>({
      path: "/admin/system/llm-config",
    }),
  updateLLMConfig: (body) =>
    request<LLMConfigResponse, UpdateLLMConfigRequest>({
      path: "/admin/system/llm-config",
      method: "PUT",
      body,
    }),
  sendSystemEvent: (body) =>
    request<AdminSystemEventResponse, AdminSystemEventRequest>({
      path: "/admin/system-event",
      method: "POST",
      body,
    }),
  listProfiles: () =>
    request<AgentProfile[]>({
      path: "/agents/profiles",
    }).then((items) => (Array.isArray(items) ? items : [])),
  createProfile: (body) =>
    request<AgentProfile, AgentProfile>({
      path: "/agents/profiles",
      method: "POST",
      body,
    }),
  updateProfile: (profileId, body) =>
    request<AgentProfile, AgentProfile>({
      path: `/agents/profiles/${encodeURIComponent(profileId)}`,
      method: "PUT",
      body,
    }),
  deleteProfile: (profileId) =>
    request<void>({
      path: `/agents/profiles/${encodeURIComponent(profileId)}`,
      method: "DELETE",
    }),
  listDrivers: () =>
    request<DriverConfig[]>({
      path: "/agents/drivers",
    }).then((items) => (Array.isArray(items) ? items : [])),
  createDriver: (body) =>
    request<DriverConfig, DriverConfig>({
      path: "/agents/drivers",
      method: "POST",
      body,
    }),
  updateDriver: (driverId, body) =>
    request<DriverConfig, DriverConfig>({
      path: `/agents/drivers/${encodeURIComponent(driverId)}`,
      method: "PUT",
      body,
    }),
  deleteDriver: (driverId) =>
    request<void>({
      path: `/agents/drivers/${encodeURIComponent(driverId)}`,
      method: "DELETE",
    }),
  listSkills: () =>
    request<SkillInfo[]>({
      path: "/skills",
    }).then((items) => (Array.isArray(items) ? items : [])),
  getSkill: (name) =>
    request<SkillDetail>({
      path: `/skills/${encodeURIComponent(name)}`,
    }),
  createSkill: (body) =>
    request<SkillInfo, CreateSkillRequest>({
      path: "/skills",
      method: "POST",
      body,
    }),
  updateSkill: (name, body) =>
    request<SkillInfo, { skill_md: string }>({
      path: `/skills/${encodeURIComponent(name)}`,
      method: "PUT",
      body,
    }),
  deleteSkill: (name) =>
    request<void>({
      path: `/skills/${encodeURIComponent(name)}`,
      method: "DELETE",
    }),
  importGitHubSkill: (body) =>
    request<SkillInfo, ImportGitHubSkillRequest>({
      path: "/skills/import/github",
      method: "POST",
      body,
    }),
});
