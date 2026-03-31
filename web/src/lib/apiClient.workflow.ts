import type {
  Action,
  CancelWorkItemResponse,
  BootstrapPRWorkItemRequest,
  BootstrapPRWorkItemResponse,
  CreateActionRequest,
  CreateDAGTemplateRequest,
  CreateWorkItemFromTemplateRequest,
  CreateWorkItemFromTemplateResponse,
  CreateWorkItemRequest,
  DAGTemplate,
  Deliverable,
  Event,
  GenerateActionsRequest,
  Resource,
  Run,
  RunWorkItemResponse,
  SaveWorkItemAsTemplateRequest,
  SetupCronRequest,
  UpdateActionRequest,
  UpdateDAGTemplateRequest,
  UpdateWorkItemRequest,
  UsageRecord,
  WorkItem,
} from "../types/apiV2";
import type { ApiClient } from "./apiClient";
import type { ApiBuilderContext } from "./apiClient.shared";
import { ApiError } from "./httpTransport";

export const buildWorkflowApi = ({
  request,
  buildUrl,
  normalizeCronStatus,
}: ApiBuilderContext): Pick<
  ApiClient,
  | "listWorkItems"
  | "createWorkItem"
  | "getWorkItem"
  | "runWorkItem"
  | "cancelWorkItem"
  | "updateWorkItem"
  | "archiveWorkItem"
  | "bootstrapPRWorkItem"
  | "listWorkItemDeliverables"
  | "adoptWorkItemFinalDeliverable"
  | "listActions"
  | "createAction"
  | "generateActions"
  | "generateTitle"
  | "getAction"
  | "updateAction"
  | "deleteAction"
  | "listRuns"
  | "getRun"
  | "listEvents"
  | "listWorkItemEvents"
  | "listCronWorkItems"
  | "getWorkItemCronStatus"
  | "setupWorkItemCron"
  | "disableWorkItemCron"
  | "listDAGTemplates"
  | "createDAGTemplate"
  | "getDAGTemplate"
  | "updateDAGTemplate"
  | "deleteDAGTemplate"
  | "saveWorkItemAsTemplate"
  | "createWorkItemFromTemplate"
  | "uploadWorkItemAttachment"
  | "listWorkItemAttachments"
  | "getWorkItemAttachment"
  | "deleteWorkItemAttachment"
  | "getAttachmentDownloadUrl"
  | "getUsageByRun"
> => ({
  listWorkItems: (params) =>
    request<WorkItem[]>({
      path: "/work-items",
      query: {
        project_id: params?.project_id,
        status: params?.status,
        archived: params?.archived === undefined ? undefined : String(params.archived),
        limit: params?.limit,
        offset: params?.offset,
      },
    }).then((items) => (Array.isArray(items) ? items : [])),
  createWorkItem: (body) =>
    request<WorkItem, CreateWorkItemRequest>({
      path: "/work-items",
      method: "POST",
      body,
    }),
  getWorkItem: (workItemId) =>
    request<WorkItem>({
      path: `/work-items/${workItemId}`,
    }),
  runWorkItem: (workItemId) =>
    request<RunWorkItemResponse>({
      path: `/work-items/${workItemId}/run`,
      method: "POST",
    }),
  cancelWorkItem: (workItemId) =>
    request<CancelWorkItemResponse>({
      path: `/work-items/${workItemId}/cancel`,
      method: "POST",
    }),
  updateWorkItem: (workItemId, body) =>
    request<WorkItem, UpdateWorkItemRequest>({
      path: `/work-items/${workItemId}`,
      method: "PUT",
      body,
    }),
  archiveWorkItem: (workItemId) =>
    request<void>({
      path: `/work-items/${workItemId}/archive`,
      method: "POST",
    }),
  bootstrapPRWorkItem: (workItemId, body) =>
    request<BootstrapPRWorkItemResponse, BootstrapPRWorkItemRequest>({
      path: `/work-items/${workItemId}/bootstrap-pr`,
      method: "POST",
      body,
    }),
  listWorkItemDeliverables: (workItemId) =>
    request<Deliverable[]>({
      path: `/work-items/${workItemId}/deliverables`,
    }).then((items) => (Array.isArray(items) ? items : [])),
  adoptWorkItemFinalDeliverable: (workItemId, deliverableId) =>
    request<WorkItem, { deliverable_id: number }>({
      path: `/work-items/${workItemId}/final-deliverable`,
      method: "POST",
      body: { deliverable_id: deliverableId },
    }),
  listActions: (workItemId) =>
    request<Action[]>({
      path: `/work-items/${workItemId}/actions`,
    }).then((items) => (Array.isArray(items) ? items : [])),
  createAction: (workItemId, body) =>
    request<Action, CreateActionRequest>({
      path: `/work-items/${workItemId}/actions`,
      method: "POST",
      body,
    }),
  generateActions: (workItemId, body) =>
    request<Action[], GenerateActionsRequest>({
      path: `/work-items/${workItemId}/generate-actions`,
      method: "POST",
      body,
    }).then((items) => (Array.isArray(items) ? items : [])),
  generateTitle: (body) =>
    request<{ title: string }, { description: string }>({
      path: "/work-items/generate-title",
      method: "POST",
      body,
    }),
  getAction: (actionId) =>
    request<Action>({
      path: `/actions/${actionId}`,
    }),
  updateAction: (actionId, body) =>
    request<Action, UpdateActionRequest>({
      path: `/actions/${actionId}`,
      method: "PUT",
      body,
    }),
  deleteAction: (actionId) =>
    request<void>({
      path: `/actions/${actionId}`,
      method: "DELETE",
    }),
  listRuns: (actionId) =>
    request<Run[]>({
      path: `/actions/${actionId}/runs`,
    }).then((items) => (Array.isArray(items) ? items : [])),
  getRun: (runId) =>
    request<Run>({
      path: `/runs/${runId}`,
    }),
  listEvents: (params) =>
    request<Event[]>({
      path: "/events",
      query: {
        work_item_id: params?.work_item_id,
        action_id: params?.action_id,
        session_id: params?.session_id,
        types: params?.types?.join(","),
        limit: params?.limit,
        offset: params?.offset,
      },
    }).then((items) => (Array.isArray(items) ? items : [])),
  listWorkItemEvents: (workItemId, params) =>
    request<Event[]>({
      path: `/work-items/${workItemId}/events`,
      query: {
        types: params?.types?.join(","),
        limit: params?.limit,
        offset: params?.offset,
      },
    }).then((items) => (Array.isArray(items) ? items : [])),
  listCronWorkItems: () =>
    request<unknown[]>({
      path: "/work-items/cron",
    }).then((items) => {
      if (!Array.isArray(items)) {
        return [];
      }
      return items.map((item) => {
        const normalized = normalizeCronStatus(item);
        if (!normalized) {
          throw new ApiError(500, "Invalid cron status response", item);
        }
        return normalized;
      });
    }),
  getWorkItemCronStatus: (workItemId) =>
    request<unknown>({
      path: `/work-items/${workItemId}/cron`,
    }).then((item) => {
      const normalized = normalizeCronStatus(item);
      if (!normalized) {
        throw new ApiError(500, "Invalid cron status response", item);
      }
      return normalized;
    }),
  setupWorkItemCron: (workItemId, body) =>
    request<unknown, SetupCronRequest>({
      path: `/work-items/${workItemId}/cron`,
      method: "POST",
      body,
    }).then((item) => {
      const normalized = normalizeCronStatus(item);
      if (!normalized) {
        throw new ApiError(500, "Invalid cron status response", item);
      }
      return normalized;
    }),
  disableWorkItemCron: (workItemId) =>
    request<unknown>({
      path: `/work-items/${workItemId}/cron`,
      method: "DELETE",
    }).then((item) => {
      const normalized = normalizeCronStatus(item);
      if (!normalized) {
        throw new ApiError(500, "Invalid cron status response", item);
      }
      return normalized;
    }),
  listDAGTemplates: (params) =>
    request<DAGTemplate[]>({
      path: "/templates",
      query: {
        project_id: params?.project_id,
        tag: params?.tag,
        search: params?.search,
        limit: params?.limit,
        offset: params?.offset,
      },
    }).then((items) => (Array.isArray(items) ? items : [])),
  createDAGTemplate: (body) =>
    request<DAGTemplate, CreateDAGTemplateRequest>({
      path: "/templates",
      method: "POST",
      body,
    }),
  getDAGTemplate: (templateId) =>
    request<DAGTemplate>({
      path: `/templates/${templateId}`,
    }),
  updateDAGTemplate: (templateId, body) =>
    request<DAGTemplate, UpdateDAGTemplateRequest>({
      path: `/templates/${templateId}`,
      method: "PUT",
      body,
    }),
  deleteDAGTemplate: (templateId) =>
    request<void>({
      path: `/templates/${templateId}`,
      method: "DELETE",
    }),
  saveWorkItemAsTemplate: (workItemId, body) =>
    request<DAGTemplate, SaveWorkItemAsTemplateRequest>({
      path: `/work-items/${workItemId}/save-as-template`,
      method: "POST",
      body,
    }),
  createWorkItemFromTemplate: (templateId, body) =>
    request<CreateWorkItemFromTemplateResponse, CreateWorkItemFromTemplateRequest>({
      path: `/templates/${templateId}/create-work-item`,
      method: "POST",
      body,
    }),
  uploadWorkItemAttachment: async (workItemId, file) => {
    const formData = new FormData();
    formData.append("file", file);
    return request<Resource, FormData>({
      path: `/work-items/${workItemId}/resources`,
      method: "POST",
      body: formData,
      bodyMode: "raw",
      responseType: "json",
    });
  },
  listWorkItemAttachments: (workItemId) =>
    request<Resource[]>({
      path: `/work-items/${workItemId}/resources`,
    }).then((items) => (Array.isArray(items) ? items : [])),
  getWorkItemAttachment: (attachmentId) =>
    request<Resource>({
      path: `/resources/${attachmentId}`,
    }),
  deleteWorkItemAttachment: (attachmentId) =>
    request<void>({
      path: `/resources/${attachmentId}`,
      method: "DELETE",
    }),
  getAttachmentDownloadUrl: (attachmentId) =>
    buildUrl(`/resources/${attachmentId}/download`),
  getUsageByRun: (runId) =>
    request<UsageRecord>({
      path: `/runs/${runId}/usage`,
    }),
});
