import type { CronStatus } from "../types/apiV2";
import type { Primitive } from "./httpTransport";
import type { ApiClient } from "./apiClient";

export interface ApiBuilderContext {
  request: ApiClient["request"];
  buildUrl: (
    path: string,
    query?: Record<string, Primitive | null | undefined>,
  ) => string;
  normalizeCronStatus: (value: unknown) => CronStatus | null;
}

export const normalizeCronStatus = (value: unknown): CronStatus | null => {
  if (!value || typeof value !== "object") {
    return null;
  }

  const raw = value as Record<string, unknown>;
  const workItemID = raw.work_item_id;
  if (typeof workItemID !== "number") {
    return null;
  }

  return {
    work_item_id: workItemID,
    enabled: raw.enabled === true,
    is_template: raw.is_template === true,
    schedule: typeof raw.schedule === "string" ? raw.schedule : undefined,
    max_instances: typeof raw.max_instances === "number" ? raw.max_instances : undefined,
    last_triggered: typeof raw.last_triggered === "string" ? raw.last_triggered : undefined,
  };
};
