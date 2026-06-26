/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test, vi } from "vitest";
import { AboutAppPage } from "./AboutAppPage";

const dialogOpenMock = vi.hoisted(() => vi.fn());

vi.mock("@tauri-apps/plugin-dialog", () => ({
  open: dialogOpenMock,
}));

afterEach(() => {
  clearMocks();
  dialogOpenMock.mockReset();
  cleanup();
});

test("关于页检查更新调用真实后端并展示版本结果", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "check_update") {
      return {
        code: 0,
        message: "ok",
        data: {
          current_version: "0.1.0",
          latest_version: "0.1.1",
          has_new_version: true,
          action: "PROMPT_ONLY",
          release_notes_url: "https://updates.invest-compass.example/releases/0.1.1",
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <AboutAppPage />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("button", { name: "检查更新" }));

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "check_update", payload: {} });
  });
  expect(screen.getByText("发现新版本")).toBeInTheDocument();
  expect(screen.getByText("0.1.1")).toBeInTheDocument();
  expect(screen.getByText("发布说明可用")).toBeInTheDocument();
  expect(screen.queryByText("检查更新待接入")).not.toBeInTheDocument();
  expect(screen.queryByText("当前已是最新版本")).not.toBeInTheDocument();
});

test("关于页日志导出选择目录后调用真实后端", async () => {
  dialogOpenMock.mockResolvedValue("/tmp/invest-compass-logs");
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "export_logs") {
      return {
        file_path: "/tmp/invest-compass-logs/invest-compass-logs-20260624-120000.txt",
        file_name: "invest-compass-logs-20260624-120000.txt",
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <AboutAppPage />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("button", { name: /导出日志/ }));

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "export_logs", payload: { targetDir: "/tmp/invest-compass-logs" } });
  });
  await waitFor(() => {
    expect(screen.getByText(/脱敏日志已导出/)).toBeInTheDocument();
  });
  expect(screen.queryByText("日志导出待接入")).not.toBeInTheDocument();
});

test("关于页取消日志导出目录选择时不调用后端", async () => {
  dialogOpenMock.mockResolvedValue(null);
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <AboutAppPage />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("button", { name: /导出日志/ }));

  await waitFor(() => {
    expect(screen.getByText("已取消导出日志")).toBeInTheDocument();
  });
  expect(calls).toEqual([]);
});

test("关于页 LICENSE、用户手册和发布说明使用内置静态页面", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <AboutAppPage />
    </AntApp>,
  );

  fireEvent.click(screen.getByRole("button", { name: /查看 LICENSE/ }));
  expect(screen.getByRole("dialog", { name: "内置 LICENSE" })).toBeInTheDocument();
  expect(screen.getByText("GNU General Public License v3.0")).toBeInTheDocument();
  expect(screen.getByText(/本软件按现状提供，不附带任何明示或默示担保/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "关闭" }));

  fireEvent.click(screen.getByRole("button", { name: /打开用户手册/ }));
  expect(screen.getByRole("dialog", { name: "内置用户手册" })).toBeInTheDocument();
  expect(screen.getByText("首版使用流程")).toBeInTheDocument();
  expect(screen.getByText(/所有分析输出仅作研究辅助，不构成投资建议/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "关闭" }));

  fireEvent.click(screen.getByRole("button", { name: /查看发布说明/ }));
  expect(screen.getByRole("dialog", { name: "内置发布说明" })).toBeInTheDocument();
  expect(screen.getByText("v0.1.0 内测版")).toBeInTheDocument();
  expect(screen.getByText(/首版不包含自动下载、自动安装或静默升级能力/)).toBeInTheDocument();

  expect(calls).toEqual([]);
});
