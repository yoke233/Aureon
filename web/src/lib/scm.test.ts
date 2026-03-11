import { describe, expect, it } from "vitest";
import {
  detectScmProviderFromBinding,
  detectScmProviderFromBindings,
  getScmFlowProviderFromBindings,
  isScmFlowEnabledBinding,
} from "./scm";

describe("scm helpers", () => {
  it("优先读取 binding.config.provider", () => {
    expect(
      detectScmProviderFromBinding({
        kind: "git",
        uri: "D:/repo/demo",
        config: { provider: "codeup" },
      }),
    ).toBe("codeup");
  });

  it("可以从 GitHub https remote 识别 provider", () => {
    expect(
      detectScmProviderFromBinding({
        kind: "git",
        uri: "https://github.com/acme/demo.git",
        config: {},
      }),
    ).toBe("github");
  });

  it("可以从 Codeup ssh remote 识别 provider", () => {
    expect(
      detectScmProviderFromBinding({
        kind: "git",
        uri: "git@codeup.aliyun.com:5f6ea0829cffa29cfdd39a7f/test-workflow.git",
        config: {},
      }),
    ).toBe("codeup");
  });

  it("会跳过非 git 资源并返回第一个受支持 provider", () => {
    expect(
      detectScmProviderFromBindings([
        { kind: "local_fs", uri: "D:/repo/demo", config: {} },
        { kind: "git", uri: "https://codeup.aliyun.com/5f6ea0829cffa29cfdd39a7f/test-workflow.git", config: {} },
      ]),
    ).toBe("codeup");
  });

  it("只在支持的 provider 且显式开启时识别 scm flow", () => {
    expect(
      isScmFlowEnabledBinding({
        kind: "git",
        uri: "https://github.com/acme/demo.git",
        config: { enable_scm_flow: true },
      }),
    ).toBe(true);

    expect(
      isScmFlowEnabledBinding({
        kind: "git",
        uri: "https://gitlab.com/acme/demo.git",
        config: { enable_scm_flow: true },
      }),
    ).toBe(false);
  });

  it("返回第一个启用 scm flow 的 provider", () => {
    expect(
      getScmFlowProviderFromBindings([
        { kind: "git", uri: "https://github.com/acme/demo.git", config: {} },
        { kind: "git", uri: "https://codeup.aliyun.com/group/demo.git", config: { enable_scm_flow: true } },
      ]),
    ).toBe("codeup");
  });
});
