import { useEffect, useMemo, useState } from "react";
import type { ApiClient } from "../lib/apiClient";
import type { TaskItemStatus, TaskPlan } from "../types/workflow";

interface BoardViewProps {
  apiClient: ApiClient;
  projectId: string;
  refreshToken: number;
}

export type BoardStatus = "pending" | "ready" | "running" | "done" | "failed";

export interface BoardTask {
  id: string;
  plan_id: string;
  plan_name: string;
  title: string;
  status: BoardStatus;
  pipeline_id: string;
}

export const BOARD_COLUMNS: BoardStatus[] = [
  "pending",
  "ready",
  "running",
  "done",
  "failed",
];

const BOARD_STATUS_LABELS: Record<BoardStatus, string> = {
  pending: "Pending",
  ready: "Ready",
  running: "Running",
  done: "Done",
  failed: "Failed",
};

export const toBoardStatus = (status: TaskItemStatus): BoardStatus => {
  switch (status) {
    case "pending":
      return "pending";
    case "ready":
      return "ready";
    case "running":
      return "running";
    case "done":
      return "done";
    case "failed":
      return "failed";
    case "blocked_by_failure":
      return "failed";
    case "skipped":
      return "done";
    default:
      return "pending";
  }
};

export const groupBoardTasks = (
  tasks: BoardTask[],
): Record<BoardStatus, BoardTask[]> => {
  const grouped: Record<BoardStatus, BoardTask[]> = {
    pending: [],
    ready: [],
    running: [],
    done: [],
    failed: [],
  };
  tasks.forEach((task) => {
    grouped[task.status].push(task);
  });
  return grouped;
};

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "请求失败，请稍后重试";
};

const PAGE_LIMIT = 50;
const REFRESH_INTERVAL_MS = 10_000;

const BoardView = ({ apiClient, projectId, refreshToken }: BoardViewProps) => {
  const [tasks, setTasks] = useState<BoardTask[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    let inFlight = false;
    const loadTasks = async () => {
      if (inFlight) {
        return;
      }
      inFlight = true;
      setLoading(true);
      setError(null);
      try {
        const allPlans: TaskPlan[] = [];
        let offset = 0;
        while (true) {
          const response = await apiClient.listPlans(projectId, {
            limit: PAGE_LIMIT,
            offset,
          });
          if (cancelled) {
            return;
          }
          allPlans.push(...response.items);
          const currentCount = response.items.length;
          if (currentCount === 0) {
            break;
          }
          offset += currentCount;
          if (currentCount < PAGE_LIMIT) {
            break;
          }
        }
        const flattened: BoardTask[] = allPlans.flatMap((plan) =>
          (plan.tasks ?? []).map((task) => ({
            id: task.id,
            plan_id: plan.id,
            plan_name: plan.name || plan.id,
            title: task.title,
            status: toBoardStatus(task.status),
            pipeline_id: task.pipeline_id,
          })),
        );
        if (!cancelled) {
          setTasks(flattened);
          setSelectedTaskId((current) =>
            current && flattened.some((task) => task.id === current) ? current : null,
          );
        }
      } catch (requestError) {
        if (!cancelled) {
          setTasks([]);
          setSelectedTaskId(null);
          setError(getErrorMessage(requestError));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
        inFlight = false;
      }
    };

    void loadTasks();
    // Fallback refresh for non-PlanView scenarios where plan-scoped WS events may be missed.
    const intervalId = setInterval(() => {
      void loadTasks();
    }, REFRESH_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(intervalId);
    };
  }, [apiClient, projectId, refreshToken]);

  const groupedTasks = useMemo(() => groupBoardTasks(tasks), [tasks]);
  const selectedTask = selectedTaskId
    ? tasks.find((task) => task.id === selectedTaskId) ?? null
    : null;

  return (
    <section className="flex flex-col gap-4">
      <header className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <h1 className="text-xl font-bold">Board</h1>
        <p className="mt-1 text-sm text-slate-600">
          基于真实 plans/tasks 的看板分组（pending / ready / running / done / failed）。
        </p>
        <p className="mt-2 text-xs text-slate-500">
          数据来源: GET /api/v1/projects/:projectID/plans
        </p>
      </header>

      {error ? (
        <p className="rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
          {error}
        </p>
      ) : null}

      <section className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        {loading ? (
          <p className="text-sm text-slate-500">加载中...</p>
        ) : selectedTask ? (
          <div className="text-sm text-slate-700">
            <span className="font-semibold text-slate-900">当前选中：</span>
            {selectedTask.title} · plan={selectedTask.plan_name} · status=
            {selectedTask.status}
          </div>
        ) : (
          <p className="text-sm text-slate-500">点击任务卡片查看详情。</p>
        )}
      </section>

      <section className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
        {BOARD_COLUMNS.map((column) => (
          <article
            key={column}
            className="flex min-h-72 flex-col rounded-xl border border-slate-200 bg-white p-3 shadow-sm"
          >
            <header className="mb-3 flex items-center justify-between">
              <h2 className="text-sm font-semibold">{BOARD_STATUS_LABELS[column]}</h2>
              <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-700">
                {groupedTasks[column].length}
              </span>
            </header>
            <div className="flex flex-1 flex-col gap-2">
              {groupedTasks[column].length === 0 ? (
                <p className="rounded-lg border border-dashed border-slate-300 bg-slate-50 px-2 py-3 text-xs text-slate-500">
                  空列
                </p>
              ) : (
                groupedTasks[column].map((task) => (
                  <button
                    key={task.id}
                    type="button"
                    data-testid="board-task"
                    className={`rounded-lg border px-2 py-2 text-left text-xs transition ${
                      selectedTaskId === task.id
                        ? "border-slate-900 bg-slate-900 text-white"
                        : "border-slate-300 bg-white text-slate-800 hover:bg-slate-50"
                    }`}
                    onClick={() => {
                      setSelectedTaskId((current) =>
                        current === task.id ? null : task.id,
                      );
                    }}
                  >
                    <p className="font-semibold">{task.title}</p>
                    <p className="mt-1 opacity-80">plan={task.plan_name}</p>
                    {task.pipeline_id ? (
                      <p className="mt-1 opacity-80">pipeline={task.pipeline_id}</p>
                    ) : null}
                  </button>
                ))
              )}
            </div>
          </article>
        ))}
      </section>
    </section>
  );
};

export default BoardView;
