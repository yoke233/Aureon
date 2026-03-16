import { describe, expect, it } from "vitest";
import { applyActivityPayload } from "./chatUtils";

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
