// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { isDesktop } from "./desktopBridge";

describe("desktopBridge", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    delete (window as Window & { go?: unknown }).go;
  });

  it("存在 Wails 全局对象时识别为桌面端", () => {
    (window as Window & { go?: unknown }).go = {};

    expect(isDesktop()).toBe(true);
  });

  it("wails.localhost 地址下即使全局对象尚未注入也识别为桌面端", () => {
    const originalLocation = window.location;
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        ...originalLocation,
        hostname: "wails.localhost",
        protocol: "https:",
      },
    });
    try {
      expect(isDesktop()).toBe(true);
    } finally {
      Object.defineProperty(window, "location", {
        configurable: true,
        value: originalLocation,
      });
    }
  });
});
