// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ChatPageShell } from "./ChatPageShell";

describe("ChatPageShell", () => {
  it("为主内容区保留 min-w-0 约束，避免侧栏被挤压重叠", () => {
    const { container } = render(
      <ChatPageShell
        sidebar={<aside data-testid="chat-sidebar">sessions</aside>}
        header={<div>header</div>}
        mainPanel={<div data-testid="main-panel">main</div>}
        hiddenFileInput={<input type="file" />}
      />,
    );

    const root = container.firstElementChild as HTMLElement | null;
    const mainColumn = screen.getByTestId("main-panel").parentElement;

    expect(root?.className).toContain("min-w-0");
    expect(mainColumn?.className).toContain("min-w-0");
    expect(screen.getByTestId("chat-sidebar")).toBeTruthy();
  });
});
