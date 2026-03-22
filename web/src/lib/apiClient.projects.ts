import type {
  AnalyzeRequirementRequest,
  AnalyzeRequirementResponse,
  CreateProjectRequest,
  CreateResourceSpaceRequest,
  CreateThreadFromRequirementRequest,
  CreateThreadFromRequirementResponse,
  CreateGitTagRequest,
  CreateGitTagResponse,
  FeatureEntry,
  FeatureManifestSnapshot,
  FeatureManifestSummary,
  FeatureStatus,
  GitCommitEntry,
  GitTagEntry,
  Project,
  PushGitTagRequest,
  PushGitTagResponse,
  Resource,
  ResourceSpace,
  UpdateProjectRequest,
} from "../types/apiV2";
import type { ApiClient, DetectGitInfoResponse } from "./apiClient";
import type { ApiBuilderContext } from "./apiClient.shared";

export const buildProjectApi = ({
  request,
}: ApiBuilderContext): Pick<
  ApiClient,
  | "listProjects"
  | "createProject"
  | "getProject"
  | "updateProject"
  | "deleteProject"
  | "analyzeRequirement"
  | "createThreadFromRequirement"
  | "listProjectResources"
  | "createProjectResource"
  | "getProjectResource"
  | "deleteProjectResource"
  | "getFileResource"
  | "deleteFileResource"
  | "listRunResources"
  | "detectGitInfo"
  | "listGitCommits"
  | "listGitTags"
  | "createGitTag"
  | "pushGitTag"
  | "getOrCreateManifest"
  | "getManifest"
  | "getManifestSummary"
  | "getManifestSnapshot"
  | "listManifestEntries"
  | "createManifestEntry"
  | "updateManifestEntryStatus"
  | "updateManifestEntry"
  | "deleteManifestEntry"
> => ({
  listProjects: (params) =>
    request<Project[]>({
      path: "/projects",
      query: {
        limit: params?.limit,
        offset: params?.offset,
      },
    }).then((items) => (Array.isArray(items) ? items : [])),
  createProject: (body) =>
    request<Project, CreateProjectRequest>({
      path: "/projects",
      method: "POST",
      body,
    }),
  getProject: (projectId) =>
    request<Project>({
      path: `/projects/${projectId}`,
    }),
  updateProject: (projectId, body) =>
    request<Project, UpdateProjectRequest>({
      path: `/projects/${projectId}`,
      method: "PUT",
      body,
    }),
  deleteProject: (projectId) =>
    request<void>({
      path: `/projects/${projectId}`,
      method: "DELETE",
    }),
  analyzeRequirement: (body) =>
    request<AnalyzeRequirementResponse, AnalyzeRequirementRequest>({
      path: "/requirements/analyze",
      method: "POST",
      body,
    }),
  createThreadFromRequirement: (body) =>
    request<CreateThreadFromRequirementResponse, CreateThreadFromRequirementRequest>({
      path: "/requirements/create-thread",
      method: "POST",
      body,
    }),
  listProjectResources: (projectId) =>
    request<ResourceSpace[]>({
      path: `/projects/${projectId}/spaces`,
    }).then((items) => (Array.isArray(items) ? items : [])),
  createProjectResource: (projectId, body) =>
    request<ResourceSpace, CreateResourceSpaceRequest>({
      path: `/projects/${projectId}/spaces`,
      method: "POST",
      body,
    }),
  getProjectResource: (resourceId) =>
    request<ResourceSpace>({
      path: `/spaces/${resourceId}`,
    }),
  deleteProjectResource: (resourceId) =>
    request<void>({
      path: `/spaces/${resourceId}`,
      method: "DELETE",
    }),
  getFileResource: (resourceId) =>
    request<Resource>({
      path: `/resources/${resourceId}`,
    }),
  deleteFileResource: (resourceId) =>
    request<void>({
      path: `/resources/${resourceId}`,
      method: "DELETE",
    }),
  listRunResources: (runId) =>
    request<Resource[]>({
      path: `/runs/${runId}/resources`,
    }).then((items) => (Array.isArray(items) ? items : [])),
  detectGitInfo: (path: string) =>
    request<DetectGitInfoResponse, { path: string }>({
      path: "/utils/detect-git",
      method: "POST",
      body: { path },
    }),
  listGitCommits: (projectId, params) =>
    request<GitCommitEntry[]>({
      path: `/projects/${projectId}/git/commits`,
      query: { limit: params?.limit },
    }).then((items) => (Array.isArray(items) ? items : [])),
  listGitTags: (projectId) =>
    request<GitTagEntry[]>({
      path: `/projects/${projectId}/git/tags`,
    }).then((items) => (Array.isArray(items) ? items : [])),
  createGitTag: (projectId, body) =>
    request<CreateGitTagResponse, CreateGitTagRequest>({
      path: `/projects/${projectId}/git/tags`,
      method: "POST",
      body,
    }),
  pushGitTag: (projectId, body) =>
    request<PushGitTagResponse, PushGitTagRequest>({
      path: `/projects/${projectId}/git/tags/push`,
      method: "POST",
      body,
    }),
  getOrCreateManifest: (projectId) =>
    request<FeatureManifestSummary>({ path: `/projects/${projectId}/manifest` }),
  getManifest: (projectId) =>
    request<FeatureManifestSummary>({ path: `/projects/${projectId}/manifest` }),
  getManifestSummary: (projectId) =>
    request<FeatureManifestSummary>({ path: `/projects/${projectId}/manifest/summary` }),
  getManifestSnapshot: (projectId) =>
    request<FeatureManifestSnapshot>({ path: `/projects/${projectId}/manifest/snapshot` }),
  listManifestEntries: (projectId, params) =>
    request<FeatureEntry[]>({ path: `/projects/${projectId}/manifest/entries`, query: params }).then(
      (items) => (Array.isArray(items) ? items : []),
    ),
  createManifestEntry: (projectId, body) =>
    request<FeatureEntry, { key: string; description: string; status?: FeatureStatus; tags?: string[] }>({
      path: `/projects/${projectId}/manifest/entries`,
      method: "POST",
      body,
    }),
  updateManifestEntryStatus: (entryId, status) =>
    request<FeatureEntry, { status: FeatureStatus }>({
      path: `/manifest/entries/${entryId}/status`,
      method: "PATCH",
      body: { status },
    }),
  updateManifestEntry: (entryId, body) =>
    request<FeatureEntry, Partial<{ key: string; description: string; status: FeatureStatus; tags: string[] }>>({
      path: `/manifest/entries/${entryId}`,
      method: "PUT",
      body,
    }),
  deleteManifestEntry: (entryId) =>
    request<void>({ path: `/manifest/entries/${entryId}`, method: "DELETE" }),
});
