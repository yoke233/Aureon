/** @vitest-environment jsdom */

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => {
  const listProjects = vi.fn().mockResolvedValue([
    {
      id: "proj-1",
      name: "Alpha",
      repo_path: "D:/repo/alpha",
      created_at: "2026-03-01T10:00:00.000Z",
      updated_at: "2026-03-01T10:00:00.000Z",
    },
    {
      id: "proj-2",
      name: "Beta",
      repo_path: "D:/repo/beta",
      created_at: "2026-03-01T10:00:00.000Z",
      updated_at: "2026-03-01T10:00:00.000Z",
    },
  ]);

  const apiClient = {
    request: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    del: vi.fn(),
    getStats: vi.fn(),
    listProjects,
    createProject: vi.fn(),
    listPipelines: vi.fn().mockResolvedValue({ items: [], total: 0, offset: 0 }),
    createPipeline: vi.fn(),
    createChat: vi.fn(),
    getChat: vi.fn(),
    createPlan: vi.fn(),
    listPlans: vi.fn().mockResolvedValue({ items: [], total: 0, offset: 0 }),
    getPlanDag: vi.fn().mockResolvedValue({
      nodes: [],
      edges: [],
      stats: { total: 0, pending: 0, ready: 0, running: 0, done: 0, failed: 0 },
    }),
  };

  const wsClient = {
    connect: vi.fn(),
    disconnect: vi.fn(),
    send: vi.fn(),
    subscribe: vi.fn().mockReturnValue(() => {}),
    onStatusChange: vi.fn().mockReturnValue(() => {}),
    getStatus: vi.fn().mockReturnValue("idle"),
  };

  return {
    apiClient,
    wsClient,
    listProjects,
  };
});

vi.mock("./lib/apiClient", () => {
  return {
    createApiClient: vi.fn(() => mocks.apiClient),
  };
});

vi.mock("./lib/wsClient", () => {
  return {
    createWsClient: vi.fn(() => mocks.wsClient),
  };
});

vi.mock("./views/ChatView", () => ({
  default: () => <div>Chat View Mock</div>,
}));

vi.mock("./views/PlanView", () => ({
  default: () => <div>Plan View Mock</div>,
}));

vi.mock("./views/BoardView", () => ({
  default: () => <div>Board View Mock</div>,
}));

vi.mock("./views/PipelineView", () => ({
  default: () => <div>Pipeline View Mock</div>,
}));

import App from "./App";

describe("App", () => {
  it("加载项目、支持项目切换与四视图切换", async () => {
    render(<App />);

    await waitFor(() => {
      expect(mocks.listProjects).toHaveBeenCalledTimes(1);
    });

    expect(screen.getByText("Chat View Mock")).toBeTruthy();

    const projectSelect = screen.getByLabelText("当前项目") as HTMLSelectElement;
    expect(projectSelect.value).toBe("proj-1");

    fireEvent.click(screen.getByRole("button", { name: "Plan" }));
    expect(screen.getByText("Plan View Mock")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Board" }));
    expect(screen.getByText("Board View Mock")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Pipeline" }));
    expect(screen.getByText("Pipeline View Mock")).toBeTruthy();

    fireEvent.change(projectSelect, { target: { value: "proj-2" } });
    expect(projectSelect.value).toBe("proj-2");
  });
});
