/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { App, AppErrorBoundary } from "./App";

afterEach(() => {
  clearMocks();
  cleanup();
});

test("App 启动后通过 typed invoke service 展示 core health 状态", async () => {
  mockIPC((command) => {
    expect(command).toBe("core_health");
    return {
      code: 0,
      message: "ok",
      data: {
        status: "ok",
        version: "0.1.0",
      },
    };
  });

  render(<App />);

  expect(screen.getByText("正在连接本地核心服务")).toBeInTheDocument();

  await waitFor(() => {
    expect(screen.getByText("本地核心服务已连接")).toBeInTheDocument();
  });
  expect(screen.getByText("版本 0.1.0")).toBeInTheDocument();
});

test("App 提供基础路由外壳和错误边界", async () => {
  mockIPC(() => ({
    code: 0,
    message: "ok",
    data: {
      status: "ok",
      version: "0.1.0",
    },
  }));

  render(<App />);

  expect(screen.getByRole("navigation", { name: "主导航" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "概览" })).toHaveAttribute("href", "#/");

  await waitFor(() => {
    expect(screen.getByText("本地核心服务已连接")).toBeInTheDocument();
  });
});

test("AppErrorBoundary 捕获渲染异常并显示失败状态", () => {
  const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);

  function BrokenView(): never {
    throw new Error("render failed");
  }

  try {
    render(
      <AppErrorBoundary>
        <BrokenView />
      </AppErrorBoundary>,
    );

    expect(screen.getByText("界面渲染失败")).toBeInTheDocument();
  } finally {
    consoleError.mockRestore();
  }
});
