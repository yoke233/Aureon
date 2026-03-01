import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  MarkerType,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  type Edge,
  type Node,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { ApiClient } from "../lib/apiClient";
import type { WsClient } from "../lib/wsClient";
import type { PlanDagNode, PlanDagResponse } from "../types/api";
import type { TaskItemStatus, TaskPlan } from "../types/workflow";

interface PlanViewProps {
  apiClient: ApiClient;
  wsClient: WsClient;
  projectId: string;
  refreshToken: number;
}

type DagNodeData = {
  label: string;
  status: TaskItemStatus;
};

const NODE_STYLE_MAP: Record<TaskItemStatus, CSSProperties> = {
  pending: {
    border: "2px solid #f59e0b",
    backgroundColor: "#fffbeb",
  },
  ready: {
    border: "2px solid #2563eb",
    backgroundColor: "#eff6ff",
  },
  running: {
    border: "2px solid #0ea5e9",
    backgroundColor: "#ecfeff",
  },
  done: {
    border: "2px solid #059669",
    backgroundColor: "#ecfdf5",
  },
  failed: {
    border: "2px solid #e11d48",
    backgroundColor: "#fff1f2",
  },
  skipped: {
    border: "2px solid #64748b",
    backgroundColor: "#f8fafc",
  },
  blocked_by_failure: {
    border: "2px solid #7f1d1d",
    backgroundColor: "#fef2f2",
  },
};

const STATUS_LABELS: Record<TaskItemStatus, string> = {
  pending: "pending",
  ready: "ready",
  running: "running",
  done: "done",
  failed: "failed",
  skipped: "skipped",
  blocked_by_failure: "blocked_by_failure",
};

const PAGE_LIMIT = 50;
const FALLBACK_MINIMAP_COLOR = "#64748b";

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "请求失败，请稍后重试";
};

export const resolveMiniMapNodeColor = (status: unknown): string => {
  if (typeof status !== "string") {
    return FALLBACK_MINIMAP_COLOR;
  }
  const style = NODE_STYLE_MAP[status as TaskItemStatus];
  const border = style?.border;
  if (typeof border !== "string") {
    return FALLBACK_MINIMAP_COLOR;
  }
  const color = border.trim().split(/\s+/).at(-1);
  return color && color.length > 0 ? color : FALLBACK_MINIMAP_COLOR;
};

export const buildDagFlowNodes = (
  dagNodes: PlanDagNode[],
): Node<DagNodeData>[] => {
  return dagNodes.map((node, index) => {
    const row = Math.floor(index / 3);
    const column = index % 3;
    return {
      id: node.id,
      data: {
        label: `${node.title}\n${STATUS_LABELS[node.status]}`,
        status: node.status,
      },
      position: {
        x: column * 260,
        y: row * 140,
      },
      style: {
        ...NODE_STYLE_MAP[node.status],
        borderRadius: 12,
        width: 220,
        padding: 10,
        fontSize: 12,
      },
    };
  });
};

export const buildDagFlowEdges = (dag: PlanDagResponse): Edge[] => {
  return dag.edges.map((edge) => ({
    id: `${edge.from}->${edge.to}`,
    source: edge.from,
    target: edge.to,
    markerEnd: { type: MarkerType.ArrowClosed },
    animated: false,
    style: { strokeWidth: 1.5 },
  }));
};

const PlanView = ({ apiClient, wsClient, projectId, refreshToken }: PlanViewProps) => {
  const [plans, setPlans] = useState<TaskPlan[]>([]);
  const [activePlanId, setActivePlanId] = useState<string | null>(null);
  const [dag, setDag] = useState<PlanDagResponse | null>(null);
  const [plansLoading, setPlansLoading] = useState(false);
  const [dagLoading, setDagLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const plansRequestIdRef = useRef(0);
  const dagRequestIdRef = useRef(0);

  useEffect(() => {
    plansRequestIdRef.current += 1;
    dagRequestIdRef.current += 1;
    setPlans([]);
    setActivePlanId(null);
    setDag(null);
    setPlansLoading(false);
    setDagLoading(false);
    setError(null);
  }, [projectId]);

  const loadPlans = useCallback(async () => {
    const requestId = plansRequestIdRef.current + 1;
    plansRequestIdRef.current = requestId;
    setPlansLoading(true);
    setError(null);
    try {
      const allPlans: TaskPlan[] = [];
      let offset = 0;
      while (true) {
        const response = await apiClient.listPlans(projectId, {
          limit: PAGE_LIMIT,
          offset,
        });
        if (plansRequestIdRef.current !== requestId) {
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
      setPlans(allPlans);
      setActivePlanId((current) => {
        if (current && allPlans.some((plan) => plan.id === current)) {
          return current;
        }
        return allPlans[0]?.id ?? null;
      });
    } catch (requestError) {
      if (plansRequestIdRef.current !== requestId) {
        return;
      }
      setError(getErrorMessage(requestError));
      setPlans([]);
      setActivePlanId(null);
    } finally {
      if (plansRequestIdRef.current === requestId) {
        setPlansLoading(false);
      }
    }
  }, [apiClient, projectId]);

  useEffect(() => {
    void loadPlans();
  }, [loadPlans, refreshToken]);

  const loadDag = useCallback(
    async (planId: string) => {
      const requestId = dagRequestIdRef.current + 1;
      dagRequestIdRef.current = requestId;
      setDagLoading(true);
      setError(null);
      try {
        const response = await apiClient.getPlanDag(projectId, planId);
        if (dagRequestIdRef.current !== requestId) {
          return;
        }
        setDag(response);
      } catch (requestError) {
        if (dagRequestIdRef.current !== requestId) {
          return;
        }
        setDag(null);
        setError(getErrorMessage(requestError));
      } finally {
        if (dagRequestIdRef.current === requestId) {
          setDagLoading(false);
        }
      }
    },
    [apiClient, projectId],
  );

  useEffect(() => {
    if (!activePlanId) {
      dagRequestIdRef.current += 1;
      setDagLoading(false);
      setDag(null);
      return;
    }
    void loadDag(activePlanId);
  }, [activePlanId, loadDag, refreshToken]);

  useEffect(() => {
    if (!activePlanId) {
      return;
    }

    const subscribe = () => {
      if (wsClient.getStatus() !== "open") {
        return;
      }
      try {
        wsClient.send({ type: "subscribe_plan", plan_id: activePlanId });
      } catch {
        // Ignore: reconnect callback will retry subscribe.
      }
    };

    const unsubscribe = () => {
      if (wsClient.getStatus() !== "open") {
        return;
      }
      try {
        wsClient.send({ type: "unsubscribe_plan", plan_id: activePlanId });
      } catch {
        // No-op.
      }
    };

    subscribe();
    const unsubscribeStatus = wsClient.onStatusChange((status) => {
      if (status === "open") {
        subscribe();
      }
    });

    return () => {
      unsubscribeStatus();
      unsubscribe();
    };
  }, [activePlanId, wsClient]);

  const flowNodes = useMemo(() => buildDagFlowNodes(dag?.nodes ?? []), [dag]);
  const flowEdges = useMemo(
    () => (dag ? buildDagFlowEdges(dag) : []),
    [dag],
  );

  return (
    <ReactFlowProvider>
      <section className="grid gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
        <aside className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
          <h2 className="text-lg font-semibold">Plan 列表</h2>
          <p className="mt-1 text-xs text-slate-500">
            数据来源: GET /api/v1/projects/:projectID/plans
          </p>
          {plansLoading ? <p className="mt-3 text-sm text-slate-500">加载中...</p> : null}
          {plans.length === 0 && !plansLoading ? (
            <p className="mt-3 text-sm text-slate-500">当前项目暂无计划。</p>
          ) : null}
          <div className="mt-3 flex flex-col gap-2">
            {plans.map((plan) => (
              <button
                key={plan.id}
                type="button"
                data-testid="plan-item"
                className={`rounded-lg border px-3 py-2 text-left text-sm transition ${
                  activePlanId === plan.id
                    ? "border-slate-900 bg-slate-900 text-white"
                    : "border-slate-300 bg-white text-slate-800 hover:bg-slate-50"
                }`}
                onClick={() => {
                  setActivePlanId(plan.id);
                }}
              >
                <p className="font-semibold">{plan.name || plan.id}</p>
                <p className="mt-1 text-xs opacity-80">
                  status={plan.status} · tasks={plan.tasks.length}
                </p>
              </button>
            ))}
          </div>
        </aside>

        <div className="flex flex-col gap-4">
          <header className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
            <h1 className="text-xl font-bold">Plan DAG</h1>
            <p className="mt-1 text-sm text-slate-600">
              选中计划后调用 GET /plans/:id/dag 展示节点、边与统计。
            </p>
            {dag ? (
              <div className="mt-3 grid gap-2 text-xs text-slate-700 sm:grid-cols-3 lg:grid-cols-6">
                <span className="rounded bg-slate-100 px-2 py-1">total: {dag.stats.total}</span>
                <span className="rounded bg-slate-100 px-2 py-1">pending: {dag.stats.pending}</span>
                <span className="rounded bg-slate-100 px-2 py-1">ready: {dag.stats.ready}</span>
                <span className="rounded bg-slate-100 px-2 py-1">running: {dag.stats.running}</span>
                <span className="rounded bg-slate-100 px-2 py-1">done: {dag.stats.done}</span>
                <span className="rounded bg-slate-100 px-2 py-1">failed: {dag.stats.failed}</span>
              </div>
            ) : null}
          </header>

          {error ? (
            <p className="rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
              {error}
            </p>
          ) : null}

          <div className="h-[520px] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
            {!activePlanId ? (
              <div className="flex h-full items-center justify-center text-sm text-slate-500">
                请选择一个计划查看 DAG。
              </div>
            ) : dagLoading ? (
              <div className="flex h-full items-center justify-center text-sm text-slate-500">
                DAG 加载中...
              </div>
            ) : flowNodes.length === 0 ? (
              <div className="flex h-full items-center justify-center text-sm text-slate-500">
                当前计划暂无节点。
              </div>
            ) : (
              <ReactFlow
                nodes={flowNodes}
                edges={flowEdges}
                fitView
                proOptions={{ hideAttribution: true }}
              >
                <MiniMap
                  pannable
                  zoomable
                  nodeColor={(node) => {
                    return resolveMiniMapNodeColor(node.data?.status);
                  }}
                />
                <Controls showInteractive={false} />
                <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
              </ReactFlow>
            )}
          </div>

          <section className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
            <h3 className="text-sm font-semibold">边列表</h3>
            {dag && dag.edges.length > 0 ? (
              <ul className="mt-2 space-y-1 text-xs text-slate-700">
                {dag.edges.map((edge) => (
                  <li key={`${edge.from}->${edge.to}`}>{edge.from} -&gt; {edge.to}</li>
                ))}
              </ul>
            ) : (
              <p className="mt-2 text-xs text-slate-500">暂无边数据。</p>
            )}
          </section>
        </div>
      </section>
    </ReactFlowProvider>
  );
};

export default PlanView;
