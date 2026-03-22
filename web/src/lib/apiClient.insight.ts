import type {
  AnalyticsSummary,
  CreateNotificationRequest,
  InspectionFinding,
  InspectionInsight,
  InspectionReport,
  Notification,
  TriggerInspectionRequest,
  UnreadCountResponse,
  UsageAnalyticsSummary,
} from "../types/apiV2";
import type { ApiClient } from "./apiClient";
import type { ApiBuilderContext } from "./apiClient.shared";

export const buildInsightApi = ({
  request,
}: ApiBuilderContext): Pick<
  ApiClient,
  | "getAnalyticsSummary"
  | "getUsageSummary"
  | "listNotifications"
  | "createNotification"
  | "getNotification"
  | "markNotificationRead"
  | "markAllNotificationsRead"
  | "deleteNotification"
  | "getUnreadNotificationCount"
  | "listInspections"
  | "getInspection"
  | "triggerInspection"
  | "listInspectionFindings"
  | "listInspectionInsights"
> => ({
  getAnalyticsSummary: (params) =>
    request<AnalyticsSummary>({
      path: "/analytics/summary",
      query: {
        project_id: params?.project_id,
        since: params?.since,
        until: params?.until,
        limit: params?.limit,
      },
    }),
  getUsageSummary: (params) =>
    request<UsageAnalyticsSummary>({
      path: "/analytics/usage",
      query: {
        project_id: params?.project_id,
        since: params?.since,
        until: params?.until,
        limit: params?.limit,
      },
    }),
  listNotifications: (params) =>
    request<Notification[]>({
      path: "/notifications",
      query: {
        category: params?.category,
        level: params?.level,
        read: params?.read === undefined ? undefined : String(params.read),
        project_id: params?.project_id,
        work_item_id: params?.work_item_id,
        limit: params?.limit,
        offset: params?.offset,
      },
    }).then((items) => (Array.isArray(items) ? items : [])),
  createNotification: (body) =>
    request<Notification, CreateNotificationRequest>({
      path: "/notifications",
      method: "POST",
      body,
    }),
  getNotification: (notificationId) =>
    request<Notification>({ path: `/notifications/${notificationId}` }),
  markNotificationRead: (notificationId) =>
    request<void>({ path: `/notifications/${notificationId}/read`, method: "POST" }),
  markAllNotificationsRead: () =>
    request<void>({ path: "/notifications/read-all", method: "POST" }),
  deleteNotification: (notificationId) =>
    request<void>({ path: `/notifications/${notificationId}`, method: "DELETE" }),
  getUnreadNotificationCount: () =>
    request<UnreadCountResponse>({ path: "/notifications/unread-count" }),
  listInspections: (params) =>
    request<InspectionReport[]>({
      path: "/inspections",
      query: {
        project_id: params?.project_id,
        status: params?.status,
        since: params?.since,
        until: params?.until,
        limit: params?.limit,
        offset: params?.offset,
      },
    }).then((items) => (Array.isArray(items) ? items : [])),
  getInspection: (inspectionId) =>
    request<InspectionReport>({ path: `/inspections/${inspectionId}` }),
  triggerInspection: (body) =>
    request<InspectionReport, TriggerInspectionRequest>({
      path: "/inspections/trigger",
      method: "POST",
      body: body ?? {},
    }),
  listInspectionFindings: (inspectionId) =>
    request<InspectionFinding[]>({ path: `/inspections/${inspectionId}/findings` }).then(
      (items) => (Array.isArray(items) ? items : []),
    ),
  listInspectionInsights: (inspectionId) =>
    request<InspectionInsight[]>({ path: `/inspections/${inspectionId}/insights` }).then(
      (items) => (Array.isArray(items) ? items : []),
    ),
});
