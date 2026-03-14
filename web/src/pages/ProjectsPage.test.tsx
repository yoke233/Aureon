// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { ProjectsPage } from "./ProjectsPage";

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
        <ProjectsPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("ProjectsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("聚合项目指标，支持搜索、切换当前项目和刷新", async () => {
    const projects = [
      {
        id: 1,
        name: "Alpha",
        description: "核心业务项目",
        kind: "service",
      },
      {
        id: 2,
        name: "Beta",
        description: "边车工具项目",
        kind: "tooling",
      },
    ];

    const apiClient = {
      listWorkItems: vi.fn().mockImplementation(({ project_id }: { project_id: number }) => {
        if (project_id === 1) {
          return Promise.resolve([
            { status: "done" },
            { status: "failed" },
            { status: "running" },
          ]);
        }
        return Promise.resolve([
          { status: "queued" },
        ]);
      }),
      listProjectResources: vi.fn().mockImplementation((projectId: number) => {
        if (projectId === 1) {
          return Promise.resolve([
            { id: 1, kind: "git" },
            { id: 2, kind: "docs" },
          ]);
        }
        return Promise.resolve([
          { id: 3, kind: "local" },
        ]);
      }),
    };

    const setSelectedProjectId = vi.fn();
    const reloadProjects = vi.fn().mockResolvedValue(projects);

    mockUseWorkbench.mockReturnValue({
      apiClient,
      projects,
      selectedProjectId: 1,
      setSelectedProjectId,
      reloadProjects,
    });

    renderPage();

    expect(await screen.findByText("Alpha")).toBeTruthy();
    expect(await screen.findByText("50%")).toBeTruthy();
    expect(screen.getByText("版本标签")).toBeTruthy();

    fireEvent.change(screen.getByPlaceholderText("搜索项目..."), {
      target: { value: "Beta" },
    });

    await waitFor(() => {
      expect(screen.getByText("Beta")).toBeTruthy();
      expect(screen.queryByText("Alpha")).toBeNull();
    });

    fireEvent.change(screen.getByPlaceholderText("搜索项目..."), {
      target: { value: "" },
    });

    fireEvent.click(await screen.findByText("Beta"));
    expect(setSelectedProjectId).toHaveBeenCalledWith(2);

    fireEvent.click(screen.getByRole("button", { name: "刷新" }));
    await waitFor(() => {
      expect(reloadProjects).toHaveBeenCalledWith(1);
    });
  });
});
