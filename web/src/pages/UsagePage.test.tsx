// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { UsagePage } from "./UsagePage";

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
        <UsagePage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("UsagePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("展示用量统计并支持切换时间范围与刷新", async () => {
    const apiClient = {
      getUsageSummary: vi.fn().mockResolvedValue({
        totals: {
          total_tokens: 12345,
          input_tokens: 8000,
          output_tokens: 4345,
          run_count: 5,
          cache_read_tokens: 3000,
          cache_write_tokens: 900,
          reasoning_tokens: 1200,
        },
        by_project: [
          {
            project_id: 9,
            project_name: "Alpha",
            run_count: 3,
            input_tokens: 5000,
            output_tokens: 2000,
            total_tokens: 7000,
          },
        ],
        by_agent: [
          {
            agent_id: "agent-1",
            project_id: 9,
            project_name: "Alpha",
            run_count: 2,
            input_tokens: 2000,
            output_tokens: 1500,
            total_tokens: 3500,
          },
        ],
        by_profile: [
          {
            profile_id: "lead-1",
            agent_id: "agent-1",
            project_id: 9,
            project_name: "Alpha",
            run_count: 2,
            input_tokens: 2000,
            output_tokens: 1500,
            cache_read_tokens: 300,
            cache_write_tokens: 100,
            reasoning_tokens: 400,
            total_tokens: 3500,
          },
        ],
      }),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProjectId: 9,
    });

    renderPage();

    expect(await screen.findByText("用量统计")).toBeTruthy();
    expect(screen.getAllByText("12.3K").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Alpha").length).toBeGreaterThan(0);
    expect(screen.getAllByText("agent-1").length).toBeGreaterThan(0);
    expect(screen.getByText("lead-1")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "全部" }));

    await waitFor(() => {
      expect(apiClient.getUsageSummary).toHaveBeenLastCalledWith({
        project_id: 9,
        since: undefined,
      });
    });

    fireEvent.click(screen.getByRole("button", { name: "刷新" }));

    await waitFor(() => {
      expect(apiClient.getUsageSummary).toHaveBeenCalledTimes(3);
    });
  });

  it("加载失败时展示错误信息", async () => {
    const apiClient = {
      getUsageSummary: vi.fn().mockRejectedValue(new Error("usage unavailable")),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      selectedProjectId: 9,
    });

    renderPage();

    expect(await screen.findByText("usage unavailable")).toBeTruthy();
  });
});
