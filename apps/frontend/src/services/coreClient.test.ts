/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import { afterEach, describe, expect, test } from "vitest";
import { coreHealth } from "./coreClient";

describe("coreClient", () => {
  afterEach(() => {
    clearMocks();
  });

  test("coreHealth 通过固定 Tauri command 读取 Go core 健康状态", async () => {
    const calls: Array<{ command: string; payload?: unknown }> = [];

    mockIPC((command, payload) => {
      calls.push({ command, payload });
      return {
        code: 0,
        message: "ok",
        data: {
          status: "ok",
          version: "0.1.0",
        },
      };
    });

    await expect(coreHealth()).resolves.toEqual({
      status: "ok",
      version: "0.1.0",
    });
    expect(calls).toEqual([{ command: "core_health", payload: {} }]);
  });

  test("coreHealth 遇到统一错误响应时抛出业务错误", async () => {
    mockIPC(() => ({
      code: 50001,
      message: "sidecar not ready",
      traceId: "trace-1",
      requestId: "request-1",
      data: null,
    }));

    await expect(coreHealth()).rejects.toThrow("sidecar not ready");
  });
});
