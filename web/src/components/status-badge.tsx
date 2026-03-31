import { useTranslation } from "react-i18next";
import { Badge, type BadgeProps } from "@/components/ui/badge";

type Status = "done" | "completed" | "succeeded" | "running" | "in_execution" | "in_progress"
  | "pending" | "pending_execution" | "pending_review" | "queued" | "ready"
  | "failed" | "needs_rework" | "cancelled" | "blocked" | "escalated" | "waiting_gate" | "created" | string;

const statusVariant: Record<string, BadgeProps["variant"]> = {
  done: "success",
  completed: "success",
  succeeded: "success",
  running: "info",
  in_execution: "info",
  in_progress: "info",
  pending: "secondary",
  pending_execution: "secondary",
  pending_review: "warning",
  queued: "secondary",
  ready: "info",
  failed: "destructive",
  needs_rework: "destructive",
  cancelled: "secondary",
  blocked: "warning",
  escalated: "warning",
  waiting_gate: "warning",
  created: "secondary",
};

export function StatusBadge({ status }: { status: Status }) {
  const { t } = useTranslation();
  const variant = statusVariant[status] ?? ("outline" as const);
  const label = t(`status.${status}`, status);
  return <Badge variant={variant}>{label}</Badge>;
}
