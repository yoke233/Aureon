// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { InspectionPage } from "./InspectionPage";

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
        <InspectionPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("InspectionPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("加载巡检报告、切换详情、展开 finding 并触发新的 inspection", async () => {
    const listResponses = [
      [
        {
          id: 11,
          status: "completed",
          trigger: "manual",
          created_at: "2026-03-15T00:00:00Z",
          period_start: "2026-03-14T00:00:00Z",
          period_end: "2026-03-15T00:00:00Z",
          findings: [{ id: 101 }],
        },
        {
          id: 12,
          status: "completed",
          trigger: "cron",
          created_at: "2026-03-15T01:00:00Z",
          period_start: "2026-03-14T01:00:00Z",
          period_end: "2026-03-15T01:00:00Z",
          findings: [],
        },
      ],
      [
        {
          id: 13,
          status: "completed",
          trigger: "manual",
          created_at: "2026-03-15T02:00:00Z",
          period_start: "2026-03-14T02:00:00Z",
          period_end: "2026-03-15T02:00:00Z",
          findings: [],
        },
      ],
    ];

    const detailById = new Map([
      [11, {
        id: 11,
        status: "completed",
        trigger: "manual",
        created_at: "2026-03-15T00:00:00Z",
        period_start: "2026-03-14T00:00:00Z",
        period_end: "2026-03-15T00:00:00Z",
        summary: "发现关键路径问题",
        snapshot: {
          total_work_items: 8,
          active_work_items: 2,
          failed_work_items: 1,
          blocked_work_items: 1,
          success_rate: 0.75,
          avg_duration_s: 120,
          total_runs: 20,
          failed_runs: 2,
          total_tokens: 1234,
        },
        findings: [
          {
            id: 101,
            inspection_id: 11,
            category: "failure",
            severity: "high",
            title: "支付链路失败率升高",
            description: "最近 24h 支付回调异常增多",
            evidence: "payment.timeout",
            recommendation: "补充重试与超时告警",
            recurring: true,
            occurrence_count: 3,
            created_at: "2026-03-15T00:00:00Z",
          },
        ],
        insights: [
          {
            id: 201,
            inspection_id: 11,
            type: "trend",
            title: "失败率恶化",
            description: "支付场景失败率较上周上升",
            trend: "degrading",
            action_items: ["补充告警", "排查超时链路"],
            created_at: "2026-03-15T00:00:00Z",
          },
        ],
        suggested_skills: [
          {
            name: "payment-recovery",
            description: "支付恢复流程",
            rationale: "重复失败模式可沉淀为 skill",
          },
        ],
      }],
      [12, {
        id: 12,
        status: "completed",
        trigger: "cron",
        created_at: "2026-03-15T01:00:00Z",
        period_start: "2026-03-14T01:00:00Z",
        period_end: "2026-03-15T01:00:00Z",
        summary: "系统整体健康",
        findings: [],
        insights: [],
        suggested_skills: [],
      }],
      [13, {
        id: 13,
        status: "completed",
        trigger: "manual",
        created_at: "2026-03-15T02:00:00Z",
        period_start: "2026-03-14T02:00:00Z",
        period_end: "2026-03-15T02:00:00Z",
        summary: "新的巡检结果",
        findings: [],
        insights: [],
        suggested_skills: [],
      }],
    ]);

    const apiClient = {
      listInspections: vi.fn().mockImplementation(() => Promise.resolve(listResponses.shift() ?? [])),
      getInspection: vi.fn().mockImplementation((id: number) => Promise.resolve(detailById.get(id))),
      triggerInspection: vi.fn().mockResolvedValue(detailById.get(13)),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProjectId: 9,
    });

    renderPage();

    expect(await screen.findByText("发现关键路径问题")).toBeTruthy();
    expect(await screen.findByText("支付链路失败率升高")).toBeTruthy();

    fireEvent.click(screen.getByText("支付链路失败率升高"));
    expect(await screen.findByText("最近 24h 支付回调异常增多")).toBeTruthy();
    expect(screen.getByText("补充重试与超时告警")).toBeTruthy();
    expect(screen.getByText("payment-recovery")).toBeTruthy();

    const cronReportButton = screen.getAllByRole("button").find((button) => button.textContent?.includes("cron"));
    expect(cronReportButton).toBeTruthy();
    fireEvent.click(cronReportButton as HTMLButtonElement);
    await waitFor(() => {
      expect(apiClient.getInspection).toHaveBeenCalledWith(12);
    });
    expect(await screen.findByText("系统整体健康")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Run Inspection" }));

    await waitFor(() => {
      expect(apiClient.triggerInspection).toHaveBeenCalledWith({
        project_id: 9,
        lookback_hours: 24,
      });
    });

    expect(await screen.findByText("新的巡检结果")).toBeTruthy();
    await waitFor(() => {
      expect(apiClient.listInspections).toHaveBeenCalledTimes(2);
    });
  });
});
