// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { LoginPage } from "./LoginPage";

function renderPage(props?: Partial<React.ComponentProps<typeof LoginPage>>) {
  const onLogin = vi.fn();
  render(
    <I18nextProvider i18n={i18n}>
      <LoginPage onLogin={onLogin} {...props} />
    </I18nextProvider>,
  );
  return { onLogin };
}

describe("LoginPage", () => {
  beforeEach(() => {
    localStorage.clear();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("提交时会 trim token 并调用登录", () => {
    const { onLogin } = renderPage();

    fireEvent.change(screen.getByPlaceholderText("访问令牌"), {
      target: { value: "  secret-token  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "登录" }));

    expect(onLogin).toHaveBeenCalledWith("secret-token");
  });

  it("展示错误并支持切换语言", async () => {
    renderPage({ error: "bad token" });

    expect(screen.getByText("bad token")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "English" }));

    await waitFor(() => {
      expect(screen.getByPlaceholderText("Access Token")).toBeTruthy();
    });
    expect(localStorage.getItem("ai-workflow-lang")).toBe("en");
  });

  it("loading 时禁用输入和提交", () => {
    renderPage({ loading: true });

    expect(screen.getByPlaceholderText("访问令牌").hasAttribute("disabled")).toBe(true);
    expect(screen.getByRole("button", { name: "验证中..." }).hasAttribute("disabled")).toBe(true);
  });
});
