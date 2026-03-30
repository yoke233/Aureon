import { describe, expect, it } from "vitest";
import { applyActivityPayload, defaultDraftProfileID, resolveProfileLabel } from "./chatUtils";

describe("applyActivityPayload", () => {
  it("为空 tool_call_id 生成唯一的 tool activity id", () => {
    const t = ((key: string) => key) as never;
    const at = "2026-03-16T13:30:00.000Z";

    const first = applyActivityPayload([], "session-1", {
      type: "tool_call",
      tool_call_id: "   ",
      content: "first",
    }, at, t);

    const second = applyActivityPayload(first, "session-1", {
      type: "tool_call",
      tool_call_id: "",
      content: "second",
    }, at, t);

    expect(second).toHaveLength(2);
    expect(second[0].id).toBe("session-1-tool-1773667800000-0");
    expect(second[1].id).toBe("session-1-tool-1773667800000-1");
    expect(new Set(second.map((item) => item.id)).size).toBe(2);
  });
});

describe("defaultDraftProfileID", () => {
  it("优先选择 ceo profile", () => {
    expect(defaultDraftProfileID([
      { id: "lead", name: "Lead" },
      { id: "ceo", name: "CEO Orchestrator" },
    ] as never)).toBe("ceo");
  });

  it("保留当前仍存在的选择", () => {
    expect(defaultDraftProfileID([
      { id: "ceo", name: "CEO Orchestrator" },
      { id: "lead", name: "Lead" },
    ] as never, "lead")).toBe("lead");
  });
});

describe("resolveProfileLabel", () => {
  it("优先使用 profile_name，其次回退到 profile_id", () => {
    expect(resolveProfileLabel("CEO Orchestrator", "ceo", "Agent")).toBe("CEO Orchestrator");
    expect(resolveProfileLabel("", "ceo", "Agent")).toBe("ceo");
    expect(resolveProfileLabel("", "", "Agent")).toBe("Agent");
  });
});
