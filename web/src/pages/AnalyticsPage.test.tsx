// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { AnalyticsPage } from "./AnalyticsPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <AnalyticsPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("AnalyticsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("展示分析指标并支持切换时间范围与手动刷新", async () => {
    const apiClient = {
      getAnalyticsSummary: vi.fn().mockResolvedValue({
        status_distribution: [
          { status: "done", count: 5 },
          { status: "failed", count: 1 },
        ],
        error_breakdown: [
          { error_kind: "permanent", count: 1 },
        ],
        duration_stats: [
          {
            work_item_id: 21,
            work_item_title: "支付链路",
            run_count: 6,
            avg_duration_s: 125,
            min_duration_s: 60,
            max_duration_s: 300,
          },
        ],
        bottlenecks: [
          {
            action_id: 33,
            action_name: "回调确认",
            work_item_id: 21,
            work_item_title: "支付链路",
            avg_duration_s: 98,
            max_duration_s: 180,
            fail_rate: 0.25,
            retry_count: 2,
          },
        ],
        project_errors: [
          {
            project_id: 9,
            project_name: "Alpha",
            total_work_items: 10,
            failed_work_items: 2,
            failure_rate: 0.2,
            failed_runs: 3,
          },
        ],
        recent_failures: [
          {
            run_id: 77,
            failed_at: "2026-03-15T00:00:00Z",
            project_name: "Alpha",
            work_item_id: 21,
            work_item_title: "支付链路",
            action_name: "回调确认",
            error_kind: "permanent",
            attempt: 2,
            duration_s: 98,
            error_message: "timeout",
          },
        ],
      }),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProjectId: 9,
    });

    renderPage();

    expect(await screen.findByText("运行分析")).toBeTruthy();
    expect(screen.getAllByText("支付链路").length).toBeGreaterThan(0);
    expect(screen.getAllByText("回调确认").length).toBeGreaterThan(0);
    expect(screen.getAllByText("永久错误").length).toBeGreaterThan(0);
    expect(screen.getByText("timeout")).toBeTruthy();
    expect(screen.getByRole("link", { name: "打开控制台" }).getAttribute("href")).toBe("/monitoring/scheduled-tasks");

    fireEvent.click(screen.getByRole("button", { name: "全部" }));

    await waitFor(() => {
      expect(apiClient.getAnalyticsSummary).toHaveBeenLastCalledWith({
        project_id: 9,
        since: undefined,
      });
    });

    fireEvent.click(screen.getByRole("button", { name: "刷新" }));

    await waitFor(() => {
      expect(apiClient.getAnalyticsSummary).toHaveBeenCalledTimes(3);
    });
  });

  it("加载失败时展示错误信息", async () => {
    const apiClient = {
      getAnalyticsSummary: vi.fn().mockRejectedValue(new Error("analytics unavailable")),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProjectId: 9,
    });

    renderPage();

    expect(await screen.findByText("analytics unavailable")).toBeTruthy();
  });
});
