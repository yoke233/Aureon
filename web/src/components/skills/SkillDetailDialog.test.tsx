// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import type React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { SkillDetailDialog } from "./SkillDetailDialog";

function renderDialog(props?: Partial<React.ComponentProps<typeof SkillDetailDialog>>) {
  const onClose = vi.fn();
  const onSave = vi.fn().mockResolvedValue(undefined);
  const onDelete = vi.fn();
  const result = render(
    <I18nextProvider i18n={i18n}>
      <SkillDetailDialog
        open
        loading={false}
        skill={{
          name: "alpha-skill",
          has_skill_md: true,
          valid: true,
          metadata: { name: "alpha-skill", description: "处理主流程" },
          skill_md: "# alpha",
          validation_errors: [],
          profiles_using: [],
        }}
        onClose={onClose}
        onSave={onSave}
        onDelete={onDelete}
        {...props}
      />
    </I18nextProvider>,
  );

  return { ...result, onClose, onSave, onDelete };
}

describe("SkillDetailDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("支持编辑 skill_md 并保存", async () => {
    const { onSave } = renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "编辑" }));

    const textarea = screen.getByRole("textbox");
    fireEvent.change(textarea, { target: { value: "# alpha\nupdated" } });
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith("alpha-skill", "# alpha\nupdated");
    });

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "编辑" })).toBeTruthy();
    });
  });

  it("技能被配置使用时禁用删除，并展示校验错误和使用信息", () => {
    renderDialog({
      skill: {
        name: "beta-skill",
        has_skill_md: false,
        valid: false,
        metadata: { name: "beta-skill", description: "需要修复" },
        skill_md: "",
        validation_errors: ["缺少 frontmatter"],
        profiles_using: ["lead-beta"],
      },
    });

    expect(screen.getByText("校验错误")).toBeTruthy();
    expect(screen.getByText("缺少 frontmatter")).toBeTruthy();
    expect(screen.getByText("使用此技能的配置")).toBeTruthy();
    expect(screen.getByText("lead-beta")).toBeTruthy();
    expect((screen.getByRole("button", { name: "删除" }) as HTMLButtonElement).disabled).toBe(true);
  });
});
