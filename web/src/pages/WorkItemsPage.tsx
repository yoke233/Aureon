import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import {
  ChevronDown,
  Columns3,
  Folder,
  List,
  Loader2,
  Plus,
  Search,
  Tag,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectItem } from "@/components/ui/select";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { formatRelativeTime, getErrorMessage } from "@/lib/v2Workbench";
import { cn } from "@/lib/utils";
import type { AgentProfile, PendingWorkItem, WorkItem, WorkItemPriority, WorkItemStatus } from "@/types/apiV2";

interface KanbanColumn {
  key: string;
  statuses: WorkItemStatus[];
  color: string;
  bgHover: string;
}

const KANBAN_COLUMNS: KanbanColumn[] = [
  { key: "open", statuses: ["open", "pending_execution"], color: "bg-blue-500", bgHover: "hover:bg-blue-50" },
  { key: "accepted", statuses: ["accepted", "queued", "pending_review"], color: "bg-amber-500", bgHover: "hover:bg-amber-50" },
  { key: "in_progress", statuses: ["running", "in_execution", "blocked", "escalated", "failed", "needs_rework"], color: "bg-violet-500", bgHover: "hover:bg-violet-50" },
  { key: "done", statuses: ["done", "completed"], color: "bg-emerald-500", bgHover: "hover:bg-emerald-50" },
  { key: "closed", statuses: ["closed", "cancelled"], color: "bg-zinc-400", bgHover: "hover:bg-zinc-50" },
];

const priorityConfig: Record<WorkItemPriority, { label: string; text: string; bg: string }> = {
  urgent: { label: "紧急", text: "text-red-500", bg: "bg-red-50" },
  high: { label: "高", text: "text-amber-500", bg: "bg-amber-50" },
  medium: { label: "中", text: "text-blue-500", bg: "bg-blue-50" },
  low: { label: "低", text: "text-zinc-500", bg: "bg-zinc-50" },
};

function PriorityBadge({ priority }: { priority: WorkItemPriority }) {
  const config = priorityConfig[priority] ?? priorityConfig.medium;
  return (
    <span className={cn("rounded px-1.5 py-0.5 text-[11px] font-medium", config.text, config.bg)}>
      {config.label}
    </span>
  );
}

function LabelBadge({ label }: { label: string }) {
  return (
    <span className="rounded bg-blue-50 px-1.5 py-0.5 text-[11px] font-medium text-blue-500">
      {label}
    </span>
  );
}

function WorkItemCard({ workItem, projectName }: { workItem: WorkItem; projectName?: string }) {
  return (
    <Link
      to={`/work-items/${workItem.id}`}
      className="block rounded-md border bg-white p-3 transition-shadow hover:shadow-sm"
    >
      {projectName ? (
        <div className="mb-1 flex items-center gap-1 text-[11px] text-muted-foreground">
          <Folder className="h-3 w-3" />
          <span>{projectName}</span>
        </div>
      ) : null}
      <p className="text-[13px] font-medium leading-snug text-foreground">{workItem.title}</p>
      {workItem.body ? (
        <p className="mt-1 line-clamp-2 text-xs leading-relaxed text-muted-foreground">{workItem.body}</p>
      ) : null}
      <div className="mt-2 flex items-center justify-between">
        <PriorityBadge priority={workItem.priority} />
        <div className="flex items-center gap-1">
          {workItem.labels?.slice(0, 2).map((label) => (
            <LabelBadge key={label} label={label} />
          ))}
        </div>
      </div>
    </Link>
  );
}

function pendingReasonLabel(reason: string) {
  switch (reason) {
    case "pending_review":
      return "待审核";
    case "needs_rework":
      return "待返工";
    case "escalated":
      return "待上级处理";
    default:
      return reason;
  }
}

function InboxWorkItemCard({
  item,
  decisionLoading,
  onApprove,
  onReject,
  onUnblock,
}: {
  item: PendingWorkItem;
  decisionLoading: boolean;
  onApprove: () => void;
  onReject: () => void;
  onUnblock: () => void;
}) {
  const workItem = item.work_item;
  const pendingAction = item.pending_action;

  return (
    <div className="rounded-lg border bg-white p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <Link to={`/work-items/${workItem.id}`} className="text-sm font-semibold text-foreground hover:underline">
            {workItem.title}
          </Link>
          <div className="mt-1 flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
            <span className="rounded bg-amber-50 px-1.5 py-0.5 font-medium text-amber-700">
              {pendingReasonLabel(item.reason)}
            </span>
            <span>处理人：{item.next_handler}</span>
            <span>WI-{workItem.id}</span>
          </div>
        </div>
        <PriorityBadge priority={workItem.priority} />
      </div>

      {item.latest_summary ? (
        <p className="mt-3 text-xs leading-relaxed text-muted-foreground">{item.latest_summary}</p>
      ) : null}

      {item.latest_context?.content ? (
        <div className="mt-3 rounded-md bg-muted/60 px-3 py-2 text-xs text-muted-foreground">
          {item.latest_context.content}
        </div>
      ) : null}

      <div className="mt-3 flex items-center justify-between gap-3">
        <div className="text-[11px] text-muted-foreground">
          {pendingAction ? `待处理动作：${pendingAction.name} · ${pendingAction.status}` : "暂无待处理动作"}
        </div>
        <div className="flex items-center gap-2">
          <Link to={`/work-items/${workItem.id}`}>
            <Button size="sm" variant="outline" className="h-8 px-3 text-xs">
              查看详情
            </Button>
          </Link>
          {pendingAction ? (
            pendingAction.status === "blocked" ? (
              <Button
                size="sm"
                className="h-8 px-3 text-xs"
                disabled={decisionLoading}
                onClick={onUnblock}
              >
                {decisionLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : "解除阻塞"}
              </Button>
            ) : (
              <>
                <Button
                  size="sm"
                  variant="outline"
                  className="h-8 px-3 text-xs"
                  disabled={decisionLoading}
                  onClick={onReject}
                >
                  {decisionLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : "Reject"}
                </Button>
                <Button
                  size="sm"
                  className="h-8 px-3 text-xs"
                  disabled={decisionLoading}
                  onClick={onApprove}
                >
                  {decisionLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : "Approve"}
                </Button>
              </>
            )
          ) : null}
        </div>
      </div>
    </div>
  );
}

export function WorkItemsPage() {
  const { t } = useTranslation();
  const { apiClient, selectedProject, selectedProjectId, projects } = useWorkbench();
  const [search, setSearch] = useState("");
  const [workItems, setWorkItems] = useState<WorkItem[]>([]);
  const [pendingItems, setPendingItems] = useState<PendingWorkItem[]>([]);
  const [profiles, setProfiles] = useState<AgentProfile[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pageMode, setPageMode] = useState<"all" | "inbox">("all");
  const [viewMode, setViewMode] = useState<"kanban" | "list">("kanban");
  const [priorityFilter, setPriorityFilter] = useState<WorkItemPriority | "all">("all");
  const [labelFilter, setLabelFilter] = useState<string>("all");
  const [profileFilter, setProfileFilter] = useState<string>("human");
  const [decisionLoadingKey, setDecisionLoadingKey] = useState<string | null>(null);
  const [reloadNonce, setReloadNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        if (pageMode === "inbox") {
          const listed = await apiClient.listPendingWorkItems(profileFilter === "all" ? undefined : profileFilter);
          if (!cancelled) {
            setPendingItems(listed);
            setWorkItems([]);
          }
          return;
        }
        const listed = await apiClient.listWorkItems({
          project_id: selectedProjectId ?? undefined,
          archived: false,
          limit: 200,
          offset: 0,
        });
        if (!cancelled) {
          setWorkItems(listed);
          setPendingItems([]);
        }
      } catch (loadError) {
        if (!cancelled) {
          setError(getErrorMessage(loadError));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [apiClient, selectedProjectId, pageMode, profileFilter, reloadNonce]);

  useEffect(() => {
    let cancelled = false;
    const loadProfiles = async () => {
      try {
        const listed = await apiClient.listProfiles();
        if (!cancelled) {
          setProfiles(listed);
        }
      } catch {
        if (!cancelled) {
          setProfiles([]);
        }
      }
    };
    void loadProfiles();
    return () => {
      cancelled = true;
    };
  }, [apiClient]);

  const projectNameMap = useMemo(() => {
    const map = new Map<number, string>();
    for (const project of projects) {
      map.set(project.id, project.name);
    }
    return map;
  }, [projects]);

  const allLabels = useMemo(() => {
    const labelSet = new Set<string>();
    for (const workItem of workItems) {
      for (const label of workItem.labels ?? []) {
        labelSet.add(label);
      }
    }
    return Array.from(labelSet).sort();
  }, [workItems]);

  const filteredWorkItems = useMemo(
    () =>
      workItems.filter((workItem) => {
        if (
          search &&
          !workItem.title.toLowerCase().includes(search.toLowerCase()) &&
          !String(workItem.id).includes(search)
        ) {
          return false;
        }
        if (priorityFilter !== "all" && workItem.priority !== priorityFilter) {
          return false;
        }
        if (labelFilter !== "all" && !(workItem.labels ?? []).includes(labelFilter)) {
          return false;
        }
        return true;
      }),
    [workItems, search, priorityFilter, labelFilter],
  );

  const filteredPendingItems = useMemo(
    () =>
      pendingItems.filter((item) => {
        const title = item.work_item.title.toLowerCase();
        const idText = String(item.work_item.id);
        if (search && !title.includes(search.toLowerCase()) && !idText.includes(search)) {
          return false;
        }
        return true;
      }),
    [pendingItems, search],
  );

  const kanbanColumns = useMemo(
    () =>
      KANBAN_COLUMNS.map((column) => ({
        ...column,
        items: filteredWorkItems.filter((workItem) => column.statuses.includes(workItem.status)),
      })),
    [filteredWorkItems],
  );

  const inboxProfileOptions = useMemo(() => {
    const seen = new Set<string>(["all", "human"]);
    const options = ["all", "human"];
    for (const profile of profiles) {
      if (!profile?.id || seen.has(profile.id)) {
        continue;
      }
      seen.add(profile.id);
      options.push(profile.id);
    }
    return options;
  }, [profiles]);

  const handleInboxDecision = async (actionId: number, decision: "approve" | "reject") => {
    setDecisionLoadingKey(`${decision}:${actionId}`);
    setError(null);
    try {
      await apiClient.decideAction(actionId, {
        decision,
        reason: decision === "approve" ? "approved from inbox" : "rejected from inbox",
      });
      setReloadNonce((value) => value + 1);
    } catch (decisionError) {
      setError(getErrorMessage(decisionError));
    } finally {
      setDecisionLoadingKey(null);
    }
  };

  const handleInboxUnblock = async (actionId: number) => {
    setDecisionLoadingKey(`unblock:${actionId}`);
    setError(null);
    try {
      await apiClient.unblockAction(actionId, { reason: "unblocked from inbox" });
      setReloadNonce((value) => value + 1);
    } catch (unblockError) {
      setError(getErrorMessage(unblockError));
    } finally {
      setDecisionLoadingKey(null);
    }
  };

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <div className="shrink-0 space-y-4 px-8 pt-8">
        <div className="flex items-center justify-between">
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-2xl font-bold tracking-tight">{t("workItems.title")}</h1>
              {loading ? <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" /> : null}
            </div>
            <p className="text-sm text-muted-foreground">{t("workItems.subtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <div className="flex overflow-hidden rounded-md border">
              <button
                type="button"
                className={cn(
                  "flex items-center gap-1.5 px-3 py-1.5 text-[13px] font-medium transition-colors",
                  pageMode === "all" ? "bg-foreground text-background" : "text-muted-foreground hover:text-foreground",
                )}
                onClick={() => setPageMode("all")}
              >
                全部
              </button>
              <button
                type="button"
                className={cn(
                  "flex items-center gap-1.5 px-3 py-1.5 text-[13px] font-medium transition-colors",
                  pageMode === "inbox" ? "bg-foreground text-background" : "text-muted-foreground hover:text-foreground",
                )}
                onClick={() => setPageMode("inbox")}
              >
                Inbox
              </button>
            </div>
            <div className="flex overflow-hidden rounded-md border">
              <button
                type="button"
                className={cn(
                  "flex items-center gap-1.5 px-3 py-1.5 text-[13px] font-medium transition-colors",
                  viewMode === "kanban" ? "bg-foreground text-background" : "text-muted-foreground hover:text-foreground",
                )}
                onClick={() => setViewMode("kanban")}
                disabled={pageMode === "inbox"}
              >
                <Columns3 className="h-4 w-4" />
                {t("workItems.kanban")}
              </button>
              <button
                type="button"
                className={cn(
                  "flex items-center gap-1.5 px-3 py-1.5 text-[13px] font-medium transition-colors",
                  viewMode === "list" ? "bg-foreground text-background" : "text-muted-foreground hover:text-foreground",
                )}
                onClick={() => setViewMode("list")}
                disabled={pageMode === "inbox"}
              >
                <List className="h-4 w-4" />
                {t("workItems.list")}
              </button>
            </div>
            <Link to="/work-items/new">
              <Button size="sm">
                <Plus className="mr-1.5 h-4 w-4" />
                {t("workItems.new")}
              </Button>
            </Link>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <div className="relative w-60">
            <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder={t("workItems.searchPlaceholder")}
              className="h-9 pl-8 text-[13px]"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>
          {!selectedProject ? (
            <div className="flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-[13px]">
              <Folder className="h-3.5 w-3.5 text-muted-foreground" />
              <span>{t("workItems.allProjects")}</span>
              <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
            </div>
          ) : null}
          {pageMode === "all" ? (
            <>
              <Select
                className="h-9 text-[13px]"
                value={priorityFilter}
                onValueChange={(v) => setPriorityFilter(v as WorkItemPriority | "all")}
              >
                <SelectItem value="all">{t("workItems.allPriorities")}</SelectItem>
                <SelectItem value="urgent">{t("workItems.priorityUrgent")}</SelectItem>
                <SelectItem value="high">{t("workItems.priorityHigh")}</SelectItem>
                <SelectItem value="medium">{t("workItems.priorityMedium")}</SelectItem>
                <SelectItem value="low">{t("workItems.priorityLow")}</SelectItem>
              </Select>
              {allLabels.length > 0 ? (
                <div className="flex items-center gap-1.5">
                  <Tag className="h-3.5 w-3.5 text-muted-foreground" />
                  <Select
                    className="h-9 text-[13px]"
                    value={labelFilter}
                    onValueChange={setLabelFilter}
                  >
                    <SelectItem value="all">{t("workItems.allLabels")}</SelectItem>
                    {allLabels.map((label) => (
                      <SelectItem key={label} value={label}>{label}</SelectItem>
                    ))}
                  </Select>
                </div>
              ) : null}
            </>
          ) : (
            <Select
              className="h-9 text-[13px]"
              value={profileFilter}
              onValueChange={setProfileFilter}
            >
              {inboxProfileOptions.map((profileId) => (
                <SelectItem key={profileId} value={profileId}>
                  {profileId === "all" ? "全部处理人" : profileId}
                </SelectItem>
              ))}
            </Select>
          )}
        </div>
      </div>

      {error ? (
        <p className="mx-8 mt-4 rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p>
      ) : null}

      <div className="flex-1 overflow-auto px-8 pb-8 pt-4">
        {pageMode === "inbox" ? (
          <div className="space-y-3">
            {filteredPendingItems.length === 0 ? (
              <div className="rounded-lg border bg-white px-4 py-10 text-center text-sm text-muted-foreground">
                当前没有待处理 inbox 项
              </div>
            ) : (
              filteredPendingItems.map((item) => {
                const actionId = item.pending_action?.id ?? 0;
                const loadingKeyPrefix = item.pending_action?.status === "blocked" ? "unblock" : "";
                const decisionLoading = actionId > 0
                  && (
                    decisionLoadingKey === `approve:${actionId}`
                    || decisionLoadingKey === `reject:${actionId}`
                    || decisionLoadingKey === `unblock:${actionId}`
                    || (loadingKeyPrefix !== "" && decisionLoadingKey === `${loadingKeyPrefix}:${actionId}`)
                  );
                return (
                  <InboxWorkItemCard
                    key={item.work_item.id}
                    item={item}
                    decisionLoading={decisionLoading}
                    onApprove={() => {
                      if (actionId > 0) {
                        void handleInboxDecision(actionId, "approve");
                      }
                    }}
                    onReject={() => {
                      if (actionId > 0) {
                        void handleInboxDecision(actionId, "reject");
                      }
                    }}
                    onUnblock={() => {
                      if (actionId > 0) {
                        void handleInboxUnblock(actionId);
                      }
                    }}
                  />
                );
              })
            )}
          </div>
        ) : viewMode === "kanban" ? (
          <div className="flex h-full gap-4">
            {kanbanColumns.map((column) => (
              <div key={column.key} className="flex min-w-[220px] flex-1 flex-col rounded-lg bg-muted/50 p-3">
                <div className="mb-3 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className={cn("h-2 w-2 rounded-full", column.color)} />
                    <span className="text-[13px] font-semibold">{t(`workItems.col_${column.key}`)}</span>
                  </div>
                  <span className="rounded-full bg-background px-2 py-0.5 text-xs font-medium text-muted-foreground">
                    {column.items.length}
                  </span>
                </div>
                <div className="flex-1 space-y-2 overflow-y-auto">
                  {column.items.map((workItem) => (
                    <WorkItemCard
                      key={workItem.id}
                      workItem={workItem}
                      projectName={workItem.project_id != null ? projectNameMap.get(workItem.project_id) : undefined}
                    />
                  ))}
                  {column.items.length === 0 ? (
                    <p className="py-6 text-center text-xs text-muted-foreground">{t("workItems.empty")}</p>
                  ) : null}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="rounded-lg border">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/30">
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">{t("workItems.titleCol")}</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">{t("common.status")}</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">{t("workItems.priority")}</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">{t("workItems.labels")}</th>
                  <th className="px-4 py-2.5 text-left text-xs font-medium text-muted-foreground">{t("workItems.updated")}</th>
                </tr>
              </thead>
              <tbody>
                {filteredWorkItems.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="px-4 py-8 text-center text-muted-foreground">{t("workItems.empty")}</td>
                  </tr>
                ) : (
                  filteredWorkItems.map((workItem) => (
                    <tr key={workItem.id} className="border-b transition-colors hover:bg-muted/30">
                      <td className="px-4 py-2.5">
                        <Link to={`/work-items/${workItem.id}`} className="font-medium hover:underline">{workItem.title}</Link>
                        {workItem.project_id != null && projectNameMap.get(workItem.project_id) ? (
                          <span className="ml-2 text-xs text-muted-foreground">{projectNameMap.get(workItem.project_id)}</span>
                        ) : null}
                      </td>
                      <td className="px-4 py-2.5">
                        <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium">{workItem.status}</span>
                      </td>
                      <td className="px-4 py-2.5"><PriorityBadge priority={workItem.priority} /></td>
                      <td className="px-4 py-2.5">
                        <div className="flex gap-1">
                          {(workItem.labels ?? []).slice(0, 3).map((label) => <LabelBadge key={label} label={label} />)}
                        </div>
                      </td>
                      <td className="px-4 py-2.5 text-xs text-muted-foreground">{formatRelativeTime(workItem.updated_at)}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
