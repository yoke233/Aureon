/** @vitest-environment jsdom */

import type { ReactNode } from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import PlanView, { resolveMiniMapNodeColor } from "./PlanView";
import type { ApiClient } from "../lib/apiClient";
import type { WsClient } from "../lib/wsClient";
import type { TaskPlan } from "../types/workflow";
import type { ListPlansResponse } from "../types/api";

vi.mock("@xyflow/react", () => {
  return {
    BackgroundVariant: { Dots: "dots" },
    MarkerType: { ArrowClosed: "arrowclosed" },
    ReactFlowProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
    ReactFlow: ({ nodes }: { nodes: Array<{ id: string; data: { label: string } }> }) => (
      <div data-testid="mock-react-flow">
        {nodes.map((node) => (
          <div key={node.id}>{node.data.label}</div>
        ))}
      </div>
    ),
    Background: () => <div data-testid="flow-background" />,
    Controls: () => <div data-testid="flow-controls" />,
    MiniMap: () => <div data-testid="flow-minimap" />,
  };
});

const buildPlan = (id: string, name: string): TaskPlan => ({
  id,
  project_id: "proj-1",
  session_id: "chat-1",
  name,
  status: "draft",
  wait_reason: "",
  tasks: [],
  fail_policy: "block",
  review_round: 0,
  created_at: "2026-03-01T10:00:00.000Z",
  updated_at: "2026-03-01T10:00:00.000Z",
});

const createMockApiClient = (): ApiClient => {
  return {
    request: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
    getStats: vi.fn(),
    listProjects: vi.fn(),
    createProject: vi.fn(),
    listPipelines: vi.fn(),
    createPipeline: vi.fn(),
    createChat: vi.fn(),
    getChat: vi.fn(),
    createPlan: vi.fn(),
    listPlans: vi.fn().mockResolvedValue({
      items: [buildPlan("plan-1", "Plan One"), buildPlan("plan-2", "Plan Two")],
      total: 2,
      offset: 0,
    }),
    getPlanDag: vi.fn().mockImplementation(async (_projectID: string, planID: string) => ({
      nodes: [
        { id: `${planID}-a`, title: "Task A", status: "pending", pipeline_id: "" },
        { id: `${planID}-b`, title: "Task B", status: "running", pipeline_id: "" },
      ],
      edges: [{ from: `${planID}-a`, to: `${planID}-b` }],
      stats: {
        total: 2,
        pending: 1,
        ready: 0,
        running: 1,
        done: 0,
        failed: 0,
      },
    })),
  } as unknown as ApiClient;
};

const createDeferred = <T,>() => {
  let resolve: (value: T | PromiseLike<T>) => void = () => {};
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
};

const createMockWsClient = (): WsClient => {
  return {
    connect: vi.fn(),
    disconnect: vi.fn(),
    send: vi.fn(),
    subscribe: vi.fn().mockReturnValue(() => {}),
    onStatusChange: vi.fn().mockReturnValue(() => {}),
    getStatus: vi.fn().mockReturnValue("open"),
  } as unknown as WsClient;
};

describe("PlanView", () => {
  afterEach(() => {
    cleanup();
  });

  it("加载计划列表并获取默认计划 DAG", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(apiClient.listPlans).toHaveBeenCalledWith("proj-1", {
        limit: 50,
        offset: 0,
      });
      expect(apiClient.getPlanDag).toHaveBeenCalledWith("proj-1", "plan-1");
      expect(wsClient.send).toHaveBeenCalledWith({
        type: "subscribe_plan",
        plan_id: "plan-1",
      });
    });

    await waitFor(() => {
      expect(screen.getByTestId("mock-react-flow")).toBeTruthy();
    });
    expect(screen.getByText("Plan One")).toBeTruthy();
    expect(screen.getByText("total: 2")).toBeTruthy();
  });

  it("切换计划后拉取新 DAG", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(apiClient.getPlanDag).toHaveBeenCalledWith("proj-1", "plan-1");
    });

    fireEvent.click(screen.getAllByTestId("plan-item")[1]);

    await waitFor(() => {
      expect(apiClient.getPlanDag).toHaveBeenCalledWith("proj-1", "plan-2");
    });
  });

  it("第一页满 50 条时会继续拉取第二页并渲染补齐数据", async () => {
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();
    vi.mocked(apiClient.listPlans)
      .mockResolvedValueOnce({
        items: Array.from({ length: 50 }, (_, index) => buildPlan(`plan-${index}`, `Plan ${index}`)),
        total: 50,
        offset: 0,
      })
      .mockResolvedValueOnce({
        items: [buildPlan("plan-50", "Plan 50")],
        total: 51,
        offset: 50,
      });

    render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    await waitFor(() => {
      expect(apiClient.listPlans).toHaveBeenNthCalledWith(1, "proj-1", {
        limit: 50,
        offset: 0,
      });
      expect(apiClient.listPlans).toHaveBeenNthCalledWith(2, "proj-1", {
        limit: 50,
        offset: 50,
      });
    });

    expect(screen.getByText("Plan 50")).toBeTruthy();
  });

  it("项目切换后会忽略旧请求返回，避免脏回写", async () => {
    const oldProjectDeferred = createDeferred<ListPlansResponse>();
    const apiClient = createMockApiClient();
    const wsClient = createMockWsClient();
    vi.mocked(apiClient.listPlans).mockImplementation((projectId) => {
      if (projectId === "proj-1") {
        return oldProjectDeferred.promise;
      }
      return Promise.resolve({
        items: [buildPlan("plan-2", "Plan Two")],
        total: 1,
        offset: 0,
      });
    });

    const { rerender } = render(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-1"
        refreshToken={0}
      />,
    );

    rerender(
      <PlanView
        apiClient={apiClient}
        wsClient={wsClient}
        projectId="proj-2"
        refreshToken={0}
      />,
    );

    oldProjectDeferred.resolve({
      items: [buildPlan("plan-1", "Plan One")],
      total: 1,
      offset: 0,
    });

    await waitFor(() => {
      expect(apiClient.getPlanDag).toHaveBeenCalledWith("proj-2", "plan-2");
    });
    expect(screen.getByText("Plan Two")).toBeTruthy();
    expect(screen.queryByText("Plan One")).toBeNull();
  });
});

describe("PlanView mini map color fallback", () => {
  it("未知状态时返回兜底色值", () => {
    expect(resolveMiniMapNodeColor(undefined)).toBe("#64748b");
    expect(resolveMiniMapNodeColor("not_supported_status")).toBe("#64748b");
  });
});
