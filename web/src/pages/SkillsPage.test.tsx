// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { SkillsPage } from "./SkillsPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

vi.mock("@/components/skills/CreateSkillDialog", () => ({
  CreateSkillDialog: ({
    open,
    onClose,
    onCreate,
  }: {
    open: boolean;
    onClose: () => void;
    onCreate: (name: string, skillMd?: string) => Promise<void>;
  }) => (open ? (
    <div data-testid="create-skill-dialog">
      <button
        type="button"
        onClick={() => {
          void onCreate("new-skill", "# skill");
          onClose();
        }}
      >
        提交创建
      </button>
    </div>
  ) : null),
}));

vi.mock("@/components/skills/ImportGitHubDialog", () => ({
  ImportGitHubDialog: ({
    open,
    onClose,
    onImport,
  }: {
    open: boolean;
    onClose: () => void;
    onImport: (repoUrl: string, skillName: string) => Promise<void>;
  }) => (open ? (
    <div data-testid="import-skill-dialog">
      <button
        type="button"
        onClick={() => {
          void onImport("https://github.com/acme/skills", "repo-skill");
          onClose();
        }}
      >
        提交导入
      </button>
    </div>
  ) : null),
}));

vi.mock("@/components/skills/SkillDetailDialog", () => ({
  SkillDetailDialog: ({
    open,
    loading,
    skill,
    onSave,
    onDelete,
  }: {
    open: boolean;
    loading: boolean;
    skill: { name: string; skill_md: string } | null;
    onSave: (name: string, skillMd: string) => Promise<void>;
    onDelete: (name: string) => void;
  }) => (open ? (
    <div data-testid="skill-detail-dialog">
      {loading ? <span>加载中</span> : null}
      {skill ? <span>{skill.name}</span> : null}
      {skill ? (
        <>
          <button type="button" onClick={() => void onSave(skill.name, `${skill.skill_md}\nupdated`)}>
            保存技能
          </button>
          <button type="button" onClick={() => onDelete(skill.name)}>
            删除技能
          </button>
        </>
      ) : null}
    </div>
  ) : null),
}));

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <SkillsPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("SkillsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("加载技能列表并支持搜索、查看详情与保存", async () => {
    const skills = [
      {
        name: "alpha-skill",
        valid: true,
        metadata: { description: "处理 Alpha 流程" },
        validation_errors: [],
        profiles_using: ["lead-alpha"],
      },
      {
        name: "beta-skill",
        valid: false,
        metadata: { description: "存在校验问题" },
        validation_errors: ["缺少 frontmatter"],
        profiles_using: [],
      },
    ];

    const apiClient = {
      listSkills: vi.fn().mockResolvedValue(skills),
      getSkill: vi.fn()
        .mockResolvedValueOnce({
          name: "alpha-skill",
          valid: true,
          metadata: { description: "处理 Alpha 流程" },
          skill_md: "# alpha",
          validation_errors: [],
          profiles_using: ["lead-alpha"],
        })
        .mockResolvedValueOnce({
          name: "alpha-skill",
          valid: true,
          metadata: { description: "处理 Alpha 流程" },
          skill_md: "# alpha\nupdated",
          validation_errors: [],
          profiles_using: ["lead-alpha"],
        }),
      createSkill: vi.fn(),
      importGitHubSkill: vi.fn(),
      updateSkill: vi.fn().mockResolvedValue({}),
      deleteSkill: vi.fn(),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    expect(await screen.findByText("alpha-skill")).toBeTruthy();
    expect(screen.getByText("beta-skill")).toBeTruthy();
    expect(screen.getAllByText("2")[0]).toBeTruthy();

    fireEvent.change(screen.getByPlaceholderText("搜索技能名称或描述..."), {
      target: { value: "beta" },
    });

    await waitFor(() => {
      expect(screen.getByText("beta-skill")).toBeTruthy();
      expect(screen.queryByText("alpha-skill")).toBeNull();
    });

    fireEvent.change(screen.getByPlaceholderText("搜索技能名称或描述..."), {
      target: { value: "" },
    });

    fireEvent.click(await screen.findByText("alpha-skill"));

    await waitFor(() => {
      expect(apiClient.getSkill).toHaveBeenCalledWith("alpha-skill");
    });

    fireEvent.click(await screen.findByRole("button", { name: "保存技能" }));

    await waitFor(() => {
      expect(apiClient.updateSkill).toHaveBeenCalledWith("alpha-skill", {
        skill_md: "# alpha\nupdated",
      });
    });

    await waitFor(() => {
      expect(apiClient.listSkills).toHaveBeenCalledTimes(2);
    });
  });

  it("支持创建、导入和删除技能", async () => {
    const skills = [
      {
        name: "alpha-skill",
        valid: true,
        metadata: { description: "处理 Alpha 流程" },
        validation_errors: [],
        profiles_using: [],
      },
    ];

    const apiClient = {
      listSkills: vi.fn().mockResolvedValue(skills),
      getSkill: vi.fn().mockResolvedValue({
        name: "alpha-skill",
        valid: true,
        metadata: { description: "处理 Alpha 流程" },
        skill_md: "# alpha",
        validation_errors: [],
        profiles_using: [],
      }),
      createSkill: vi.fn().mockResolvedValue({}),
      importGitHubSkill: vi.fn().mockResolvedValue({}),
      updateSkill: vi.fn(),
      deleteSkill: vi.fn().mockResolvedValue({}),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    await screen.findByText("alpha-skill");

    fireEvent.click(screen.getByRole("button", { name: "新建技能" }));
    fireEvent.click(await screen.findByRole("button", { name: "提交创建" }));

    await waitFor(() => {
      expect(apiClient.createSkill).toHaveBeenCalledWith({
        name: "new-skill",
        skill_md: "# skill",
      });
    });

    fireEvent.click(screen.getByRole("button", { name: "从 GitHub 导入" }));
    fireEvent.click(await screen.findByRole("button", { name: "提交导入" }));

    await waitFor(() => {
      expect(apiClient.importGitHubSkill).toHaveBeenCalledWith({
        repo_url: "https://github.com/acme/skills",
        skill_name: "repo-skill",
      });
    });

    fireEvent.click(screen.getByText("alpha-skill"));
    fireEvent.click(await screen.findByRole("button", { name: "删除技能" }));
    fireEvent.click(await screen.findByRole("button", { name: "确认删除" }));

    await waitFor(() => {
      expect(apiClient.deleteSkill).toHaveBeenCalledWith("alpha-skill");
    });
  });
});
