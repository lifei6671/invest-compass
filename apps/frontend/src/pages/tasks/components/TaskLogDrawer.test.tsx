/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { App as AntApp } from "antd";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { TaskLogDrawer } from "./TaskLogDrawer";
import type { TaskItem } from "../types";
import { taskLogsList } from "../../../services/coreClient";

const taskLogsListMock = vi.hoisted(() => vi.fn());

vi.mock("../../../services/coreClient", () => ({
  selectDirectory: vi.fn(),
  taskEvents: vi.fn().mockResolvedValue({ items: [] }),
  taskLogContext: vi.fn().mockResolvedValue({
    task_id: "task_running",
    stock: "生益科技 CN:SH:600183",
    analysis_type: "个股综合分析",
    model: "DeepSeek-V3",
  }),
  taskLogDiagnosis: vi.fn().mockResolvedValue({
    task_id: "task_running",
    summary: "当前任务暂无错误诊断。",
    causes: [],
    suggestions: [],
    retryable: false,
  }),
  taskLogGet: vi.fn().mockResolvedValue({
    id: 1,
    task_id: "task_running",
    time: "15:28:40.123",
    timestamp: "2025-05-20T15:28:40.123+08:00",
    level: "INFO",
    module: "market",
    stage: "quote_fetch",
    message: "开始拉取行情",
    retryable: false,
    raw_json: "{\"level\":\"INFO\",\"message\":\"开始拉取行情\"}",
  }),
  taskLogSummary: vi.fn().mockResolvedValue({
    title: "生益科技 个股综合分析",
    task_id: "task_running",
    task_type: "AI 分析",
    stock: "600183.SH    生益科技",
    model: "DeepSeek-V3",
    started_at: "2025-05-20 15:28:34",
    duration: "00:03:12",
    request_id: "req_test",
    trace_id: "trace_test",
    status: "RUNNING",
  }),
  taskLogsExport: vi.fn(),
  taskLogsList: taskLogsListMock,
}));

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: query.includes("min-width: 768px"),
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

const originalGetComputedStyle = window.getComputedStyle.bind(window);
Object.defineProperty(window, "getComputedStyle", {
  writable: true,
  value: (element: Element) => originalGetComputedStyle(element),
});

const runningTask: TaskItem = {
  id: "task_running",
  taskType: "AI 分析",
  title: "生益科技 个股综合分析",
  stockCode: "600183.SH",
  stockName: "生益科技",
  status: "RUNNING",
  progress: 62,
  startedAt: "2025-05-20 15:28:34",
  duration: "00:03:12",
  model: "DeepSeek-V3",
};

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  taskLogsListMock.mockReset();
});

test("暂停自动滚动后仍继续增量拉取运行中任务日志", async () => {
  vi.useFakeTimers();
  taskLogsListMock
    .mockResolvedValueOnce({
      rows: [
        {
          id: 1,
          task_id: "task_running",
          time: "15:28:40.123",
          timestamp: "2025-05-20T15:28:40.123+08:00",
          level: "INFO",
          module: "market",
          stage: "quote_fetch",
          message: "开始拉取行情",
          retryable: false,
        },
      ],
      next_after_id: 1,
      has_more: false,
    })
    .mockResolvedValue({
      rows: [
        {
          id: 2,
          task_id: "task_running",
          time: "15:28:53.941",
          timestamp: "2025-05-20T15:28:53.941+08:00",
          level: "WARN",
          module: "ai",
          stage: "stream_timeout",
          message: "模型流式响应耗时过长",
          retryable: true,
        },
      ],
      next_after_id: 2,
      has_more: false,
    });

  render(
    <AntApp>
      <TaskLogDrawer open task={runningTask} onClose={vi.fn()} />
    </AntApp>,
  );

  await vi.waitFor(() => {
    expect(screen.getByText("开始拉取行情")).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: /暂停滚动/ }));

  await act(async () => {
    vi.advanceTimersByTime(3100);
  });

  await vi.waitFor(() => {
    expect(taskLogsList).toHaveBeenCalledWith(expect.objectContaining({ afterId: 1 }));
    expect(screen.getByText("模型流式响应耗时过长")).toBeInTheDocument();
  });
});
