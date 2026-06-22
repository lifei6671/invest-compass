/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { App as AntApp } from "antd";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import type { TaskLogDetail, TaskLogListResult } from "../../../services/taskLogs";
import { taskLogsList } from "../../../services/taskLogs";
import { TaskLogDrawer } from "./TaskLogDrawer";
import type { TaskItem } from "../types";

const mocks = vi.hoisted(() => ({
  selectDirectory: vi.fn(),
  taskEvents: vi.fn(),
  taskLogContext: vi.fn(),
  taskLogDiagnosis: vi.fn(),
  taskLogGet: vi.fn(),
  taskLogSummary: vi.fn(),
  taskLogsExport: vi.fn(),
  taskLogsList: vi.fn(),
}));

vi.mock("../../../services/coreClient", () => ({
  selectDirectory: mocks.selectDirectory,
  taskEvents: mocks.taskEvents,
}));

vi.mock("../../../services/taskLogs", () => ({
  taskLogContext: mocks.taskLogContext,
  taskLogDiagnosis: mocks.taskLogDiagnosis,
  taskLogGet: mocks.taskLogGet,
  taskLogSummary: mocks.taskLogSummary,
  taskLogsExport: mocks.taskLogsExport,
  taskLogsList: mocks.taskLogsList,
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

const writeTextMock = vi.hoisted(() => vi.fn());
Object.defineProperty(navigator, "clipboard", {
  value: { writeText: writeTextMock },
  writable: true,
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

const failedTask: TaskItem = {
  ...runningTask,
  id: "task_failed",
  status: "FAILED",
  progress: 28,
  errorSummary: "模型服务响应超时",
};

const successTask: TaskItem = {
  ...runningTask,
  id: "task_success",
  status: "SUCCESS",
  progress: 100,
};

const baseRows: TaskLogListResult["rows"] = [
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
  {
    id: 3,
    task_id: "task_running",
    time: "15:29:46.120",
    timestamp: "2025-05-20T15:29:46.120+08:00",
    level: "ERROR",
    module: "ai",
    stage: "stream_failed",
    message: "Provider 响应超时，任务终止",
    retryable: true,
  },
];

function logResult(rows = baseRows): TaskLogListResult {
  return {
    rows,
    next_after_id: rows.at(-1)?.id ?? 0,
    has_more: false,
  };
}

function detailFor(id: number): TaskLogDetail {
  const row = baseRows.find((item) => item.id === id) ?? baseRows[0];
  return {
    ...row,
    raw_json: JSON.stringify({
      timestamp: row.timestamp,
      level: row.level,
      module: row.module,
      stage: row.stage,
      message: row.message,
      request_id: "req_7d29",
      trace_id: "trace_91aa",
    }),
  };
}

function renderDrawer(task: TaskItem = runningTask) {
  return render(
    <AntApp>
      <TaskLogDrawer open task={task} onClose={vi.fn()} />
    </AntApp>,
  );
}

beforeEach(() => {
  mocks.taskLogsList.mockResolvedValue(logResult());
  mocks.taskLogGet.mockImplementation((id: number) => Promise.resolve(detailFor(id)));
  mocks.taskLogSummary.mockImplementation((taskId: string) =>
    Promise.resolve({
      title: "生益科技 个股综合分析",
      task_id: taskId,
      task_type: "AI 分析",
      stock: "600183.SH    生益科技",
      model: "DeepSeek-V3",
      started_at: "2025-05-20 15:28:34",
      duration: "00:03:12",
      request_id: "req_test",
      trace_id: "trace_test",
      status: taskId === "task_success" ? "SUCCESS" : taskId === "task_failed" ? "FAILED" : "RUNNING",
    }),
  );
  mocks.taskLogDiagnosis.mockImplementation((taskId: string) =>
    Promise.resolve({
      task_id: taskId,
      summary: taskId === "task_failed" ? "模型服务响应超时，可检查代理或更换模型后重试。" : "当前任务暂无错误诊断。",
      causes: taskId === "task_failed" ? ["模型服务响应超时", "代理配置异常"] : [],
      suggestions: taskId === "task_failed" ? ["检查代理设置", "更换 AI 模型后重试"] : [],
      retryable: taskId === "task_failed",
      error_code: taskId === "task_failed" ? "AI_STREAM_TIMEOUT" : undefined,
      error_stage: taskId === "task_failed" ? "stream_failed" : undefined,
    }),
  );
  mocks.taskLogContext.mockImplementation((taskId: string) =>
    Promise.resolve({
      task_id: taskId,
      stock: "生益科技 CN:SH:600183",
      analysis_type: "个股综合分析",
      model: "DeepSeek-V3",
      prompt_template: "默认个股分析模板",
      quote_status: "已加载",
      kline_status: "120 条",
      indicator_status: "MA / MACD / RSI / KDJ / BOLL",
      news_status: "36 条",
      user_position: "未提供",
      data_updated_at: "2025-05-20 15:30:00",
    }),
  );
  mocks.taskEvents.mockResolvedValue({ items: [] });
  mocks.selectDirectory.mockResolvedValue("/tmp/invest-compass-logs");
  mocks.taskLogsExport.mockResolvedValue({
    file_path: "/tmp/invest-compass-logs/task.log",
    file_name: "task.log",
  });
  writeTextMock.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  Object.values(mocks).forEach((mock) => mock.mockReset());
  writeTextMock.mockReset();
});

describe("TaskLogDrawer", () => {
  test("renders summary", async () => {
    renderDrawer();

    expect(await screen.findByText("完整日志")).toBeInTheDocument();
    expect(screen.getByText("任务执行详情与排障信息")).toBeInTheDocument();
    expect(await screen.findByText("req_test")).toBeInTheDocument();
    expect(screen.getByText("trace_test")).toBeInTheDocument();
    expect(screen.getByText("运行中")).toBeInTheDocument();
  });

  test("keeps drawer mask on wide screens", async () => {
    renderDrawer();

    expect(await screen.findByText("完整日志")).toBeInTheDocument();
    expect(document.querySelector(".task-log-drawer-root .ant-drawer-mask")).toBeInTheDocument();
  });

  test("loads log table", async () => {
    renderDrawer();

    expect(await screen.findByText("开始拉取行情")).toBeInTheDocument();
    expect(screen.getByText("模型流式响应耗时过长")).toBeInTheDocument();
    expect(screen.getByText("Provider 响应超时，任务终止")).toBeInTheDocument();
  });

  test("filters by level", async () => {
    renderDrawer();
    await screen.findByText("开始拉取行情");

    fireEvent.mouseDown(screen.getByText("全部级别"));
    const errorOption = Array.from(document.querySelectorAll(".ant-select-item-option")).find(
      (element) => element.textContent === "ERROR",
    );
    expect(errorOption).toBeTruthy();
    fireEvent.click(errorOption as Element);

    await vi.waitFor(() => {
      expect(taskLogsList).toHaveBeenLastCalledWith(expect.objectContaining({ level: "ERROR" }));
    });
  });

  test("filters only error", async () => {
    renderDrawer();
    await screen.findByText("开始拉取行情");

    fireEvent.click(screen.getByLabelText("仅看错误"));

    await vi.waitFor(() => {
      expect(taskLogsList).toHaveBeenLastCalledWith(expect.objectContaining({ onlyError: true }));
    });
  });

  test("appends logs while running", async () => {
    vi.useFakeTimers();
    mocks.taskLogsList
      .mockResolvedValueOnce(logResult([baseRows[0]]))
      .mockResolvedValue({
        rows: [baseRows[1]],
        next_after_id: 2,
        has_more: false,
      });

    renderDrawer();
    await vi.waitFor(() => expect(screen.getByText("开始拉取行情")).toBeInTheDocument());

    await act(async () => {
      vi.advanceTimersByTime(3100);
    });

    await vi.waitFor(() => {
      expect(taskLogsList).toHaveBeenCalledWith(expect.objectContaining({ afterId: 1 }));
      expect(screen.getByText("模型流式响应耗时过长")).toBeInTheDocument();
    });
  });

  test("stops polling after terminal status", async () => {
    renderDrawer(successTask);
    await screen.findByText("开始拉取行情");

    const callCount = mocks.taskLogsList.mock.calls.length;

    expect(mocks.taskLogsList).toHaveBeenCalledTimes(callCount);
  });

  test("opens raw JSON detail", async () => {
    renderDrawer();

    await screen.findByText("开始拉取行情");
    fireEvent.click(screen.getByText("模型流式响应耗时过长"));

    await vi.waitFor(() => {
      expect(mocks.taskLogGet).toHaveBeenCalledWith(2);
      expect(screen.getByText(/stream_timeout/)).toBeInTheDocument();
    });
  });

  test("shows diagnosis for failed task", async () => {
    renderDrawer(failedTask);

    expect(await screen.findByText(/错误摘要：模型服务响应超时/)).toBeInTheDocument();
    fireEvent.click(screen.getByText("错误诊断"));

    expect(await screen.findByText("可能原因")).toBeInTheDocument();
    expect(screen.getByText("代理配置异常")).toBeInTheDocument();
    expect(screen.getAllByText("检查代理设置").length).toBeGreaterThan(0);
  });

  test("renders context summary", async () => {
    renderDrawer();
    await screen.findByText("开始拉取行情");

    fireEvent.click(screen.getByText("上下文摘要"));

    expect(await screen.findByText("默认个股分析模板")).toBeInTheDocument();
    expect(screen.getByText("36 条")).toBeInTheDocument();
    expect(screen.getByText("上下文摘要仅用于排障展示，已脱敏且不包含完整 Prompt、API Key 或用户隐私输入。")).toBeInTheDocument();
  });

  test("exports redacted logs", async () => {
    renderDrawer();
    await screen.findByText("开始拉取行情");

    fireEvent.click(screen.getByRole("button", { name: /导出脱敏日志/ }));

    await vi.waitFor(() => {
      expect(mocks.selectDirectory).toHaveBeenCalled();
      expect(mocks.taskLogsExport).toHaveBeenCalledWith("task_running", "/tmp/invest-compass-logs");
    });
  });

  test("暂停自动滚动后仍继续增量拉取运行中任务日志", async () => {
    vi.useFakeTimers();
    mocks.taskLogsList
      .mockResolvedValueOnce(logResult([baseRows[0]]))
      .mockResolvedValue({
        rows: [baseRows[1]],
        next_after_id: 2,
        has_more: false,
      });

    renderDrawer();

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
});
