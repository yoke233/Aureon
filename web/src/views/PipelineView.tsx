import { useEffect, useState } from "react";
import type { ApiClient } from "../lib/apiClient";
import type { Pipeline } from "../types/workflow";

interface PipelineViewProps {
  apiClient: ApiClient;
  projectId: string;
  refreshToken: number;
}

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "请求失败，请稍后重试";
};

const PAGE_LIMIT = 50;
const REFRESH_INTERVAL_MS = 10_000;

const PipelineView = ({ apiClient, projectId, refreshToken }: PipelineViewProps) => {
  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    let inFlight = false;
    const loadPipelines = async () => {
      if (inFlight) {
        return;
      }
      inFlight = true;
      setLoading(true);
      setError(null);

      try {
        const allPipelines: Pipeline[] = [];
        let offset = 0;
        while (true) {
          const response = await apiClient.listPipelines(projectId, {
            limit: PAGE_LIMIT,
            offset,
          });
          if (cancelled) {
            return;
          }
          allPipelines.push(...response.items);
          const currentCount = response.items.length;
          if (currentCount === 0) {
            break;
          }
          offset += currentCount;
          if (currentCount < PAGE_LIMIT) {
            break;
          }
        }
        if (!cancelled) {
          setPipelines(allPipelines);
        }
      } catch (requestError) {
        if (!cancelled) {
          setPipelines([]);
          setError(getErrorMessage(requestError));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
        inFlight = false;
      }
    };

    void loadPipelines();
    // Fallback refresh for non-PlanView scenarios where WS events are not enough to keep list current.
    const intervalId = setInterval(() => {
      void loadPipelines();
    }, REFRESH_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(intervalId);
    };
  }, [apiClient, projectId, refreshToken]);

  return (
    <section className="flex flex-col gap-4">
      <header className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <h1 className="text-xl font-bold">Pipeline</h1>
        <p className="mt-1 text-sm text-slate-600">
          最小可用列表视图，数据来源 GET /api/v1/projects/:projectID/pipelines。
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
        ) : pipelines.length === 0 ? (
          <p className="text-sm text-slate-500">当前项目暂无流水线。</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full table-auto border-collapse text-sm">
              <thead>
                <tr className="border-b border-slate-200 text-left text-xs text-slate-500">
                  <th className="px-2 py-2 font-semibold">ID</th>
                  <th className="px-2 py-2 font-semibold">Name</th>
                  <th className="px-2 py-2 font-semibold">Status</th>
                  <th className="px-2 py-2 font-semibold">Current Stage</th>
                  <th className="px-2 py-2 font-semibold">Updated</th>
                </tr>
              </thead>
              <tbody>
                {pipelines.map((pipeline) => (
                  <tr key={pipeline.id} data-testid="pipeline-row" className="border-b border-slate-100">
                    <td className="px-2 py-2 font-mono text-xs">{pipeline.id}</td>
                    <td className="px-2 py-2">{pipeline.name}</td>
                    <td className="px-2 py-2">{pipeline.status}</td>
                    <td className="px-2 py-2">{pipeline.current_stage || "-"}</td>
                    <td className="px-2 py-2">
                      {pipeline.updated_at ? new Date(pipeline.updated_at).toLocaleString("zh-CN") : "-"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </section>
  );
};

export default PipelineView;
