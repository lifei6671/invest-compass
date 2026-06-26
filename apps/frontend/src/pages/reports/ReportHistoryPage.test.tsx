/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";
import { ReportHistoryPage } from "./ReportHistoryPage";

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
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

let useAppSpy: { mockRestore: () => void } | null = null;

afterEach(() => {
  useAppSpy?.mockRestore();
  useAppSpy = null;
  clearMocks();
  cleanup();
});

test("报告历史页加载真实报告列表、刷新重载并删除后隐藏报告", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  let listRevision = 0;
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "report_list") {
      listRevision += 1;
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 7,
              task_id: "analysis-7",
              symbol: "CN:SH:600519",
              title: listRevision === 1 ? "贵州茅台 个股综合分析" : "贵州茅台 刷新后综合分析",
              analysis_type: "stock_full",
              model_name: "DeepSeek-V3",
              risk_summary: "消费复苏与估值波动",
              created_at: "2026-06-22T09:00:00Z",
              updated_at: "2026-06-22T09:00:00Z",
            },
            {
              id: 8,
              task_id: "analysis-8",
              symbol: "CN:SZ:002409",
              title: "雅克科技 技术面分析",
              analysis_type: "technical",
              model_name: "Qwen2.5-72B",
              risk_summary: "趋势反转失败；量能不足",
              created_at: "2026-06-23T10:00:00Z",
              updated_at: "2026-06-23T10:00:00Z",
            },
          ],
        },
      };
    }
    if (command === "report_stats") {
      return {
        code: 0,
        message: "ok",
        data: {
          total: 2,
          unique_symbols: 2,
          latest_created_at: "2026-06-23T10:00:00Z",
          analysis_types: [
            { name: "stock_full", count: 1 },
            { name: "technical", count: 1 },
          ],
          top_models: [
            { name: "DeepSeek-V3", count: 1 },
            { name: "Qwen2.5-72B", count: 1 },
          ],
        },
      };
    }
    if (command === "report_delete") {
      const deletePayload = payload as { id: number };
      return {
        code: 0,
        message: "ok",
        data: { id: deletePayload.id },
      };
    }
    if (command === "report_update") {
      const updatePayload = payload as { id: number; favorite: boolean };
      return {
        code: 0,
        message: "ok",
        data: {
          id: updatePayload.id,
          task_id: "analysis-7",
          symbol: "CN:SH:600519",
          title: "贵州茅台 刷新后综合分析",
          favorite: updatePayload.favorite,
        },
      };
    }
    if (command === "report_export") {
      return { saved: true, file_path: "/tmp/贵州茅台.md", file_name: "贵州茅台.md" };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/reports"]}>
      <AntApp>
        <ReportHistoryPage />
      </AntApp>
    </MemoryRouter>,
  );

  await waitFor(() => {
    expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
  });
  expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
  expect(screen.getByText("（共 2 条）")).toBeInTheDocument();
  expect(screen.getAllByText("DeepSeek-V3").length).toBeGreaterThan(0);
  expect(screen.getAllByText("Qwen2.5-72B").length).toBeGreaterThan(0);
  expect(screen.getAllByText("(50%)").length).toBeGreaterThan(0);

  fireEvent.click(screen.getByLabelText("刷新报告列表"));
  await waitFor(() => {
    expect(screen.getByText("贵州茅台 刷新后综合分析")).toBeInTheDocument();
  });

  fireEvent.click(screen.getByLabelText("收藏 贵州茅台 刷新后综合分析"));
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "report_update", payload: { id: 7, favorite: true } });
  });

  fireEvent.click(screen.getByLabelText("下载 贵州茅台 刷新后综合分析"));
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "report_export", payload: { id: 7 } });
  });

  fireEvent.click(screen.getByLabelText("删除 贵州茅台 刷新后综合分析"));
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "report_delete", payload: { id: 7 } });
  });
  expect(screen.queryByText("贵州茅台 刷新后综合分析")).not.toBeInTheDocument();
  expect(screen.getByText("（共 1 条）")).toBeInTheDocument();
  expect(calls.map((call) => call.command)).not.toContain("search_news");
  expect(calls.map((call) => call.command)).not.toContain("search_watchlist_notes");
});

test("报告历史页读取报告统计并批量删除选中报告", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "report_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 7,
              task_id: "analysis-7",
              symbol: "CN:SH:600519",
              title: "贵州茅台 个股综合分析",
              analysis_type: "stock_full",
              model_name: "DeepSeek-V3",
              risk_summary: "消费复苏与估值波动",
              created_at: "2026-06-22T09:00:00Z",
              updated_at: "2026-06-22T09:00:00Z",
            },
            {
              id: 8,
              task_id: "analysis-8",
              symbol: "CN:SZ:002409",
              title: "雅克科技 技术面分析",
              analysis_type: "technical",
              model_name: "Qwen2.5-72B",
              risk_summary: "趋势反转失败；量能不足",
              created_at: "2026-06-23T10:00:00Z",
              updated_at: "2026-06-23T10:00:00Z",
            },
          ],
        },
      };
    }
    if (command === "report_stats") {
      return {
        code: 0,
        message: "ok",
        data: {
          total: 2,
          unique_symbols: 2,
          latest_created_at: "2026-06-23T10:00:00Z",
          analysis_types: [
            { name: "stock_full", count: 1 },
            { name: "technical", count: 1 },
          ],
          top_models: [
            { name: "DeepSeek-V3", count: 1 },
            { name: "Qwen2.5-72B", count: 1 },
          ],
        },
      };
    }
    if (command === "report_batch_delete") {
      return { code: 0, message: "ok", data: { ids: [7, 8] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/reports"]}>
      <AntApp>
        <ReportHistoryPage />
      </AntApp>
    </MemoryRouter>,
  );

  await waitFor(() => {
    expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
  });
  expect(calls).toContainEqual({ command: "report_stats", payload: {} });
  expect(screen.getAllByText("(50%)").length).toBeGreaterThan(0);

  fireEvent.click(screen.getAllByRole("checkbox")[0]);
  fireEvent.mouseDown(screen.getByText("批量操作"));
  fireEvent.click(await screen.findByText("批量删除"));

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "report_batch_delete", payload: { ids: [7, 8] } });
  });
  expect(screen.queryByText("贵州茅台 个股综合分析")).not.toBeInTheDocument();
  expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
});

test("报告统计读取失败不阻塞报告列表展示", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "report_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 7,
              task_id: "analysis-7",
              symbol: "CN:SH:600519",
              title: "贵州茅台 个股综合分析",
              analysis_type: "stock_full",
              model_name: "DeepSeek-V3",
              risk_summary: "消费复苏与估值波动",
              created_at: "2026-06-22T09:00:00Z",
              updated_at: "2026-06-22T09:00:00Z",
            },
          ],
        },
      };
    }
    if (command === "report_stats") {
      throw new Error("report stats unavailable");
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/reports"]}>
      <AntApp>
        <ReportHistoryPage />
      </AntApp>
    </MemoryRouter>,
  );

  await waitFor(() => {
    expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
  });
  expect(calls).toContainEqual({ command: "report_stats", payload: {} });
  expect(screen.getByText("消费复苏与估值波动")).toBeInTheDocument();
});

test("报告历史页时间范围筛选使用后端报告日期而不是展示文本", async () => {
  mockIPC((command) => {
    if (command === "report_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 7,
              task_id: "analysis-7",
              symbol: "CN:SH:600519",
              title: "贵州茅台 个股综合分析",
              analysis_type: "stock_full",
              model_name: "DeepSeek-V3",
              risk_summary: "消费复苏与估值波动",
              created_at: "2026-06-22T09:00:00Z",
              updated_at: "2026-06-22T09:00:00Z",
            },
            {
              id: 8,
              task_id: "analysis-8",
              symbol: "CN:SZ:002409",
              title: "雅克科技 技术面分析",
              analysis_type: "technical",
              model_name: "Qwen2.5-72B",
              risk_summary: "趋势反转失败；量能不足",
              created_at: "2026-06-23T10:00:00Z",
              updated_at: "2026-06-23T10:00:00Z",
            },
          ],
        },
      };
    }
    if (command === "report_stats") {
      return {
        code: 0,
        message: "ok",
        data: { total: 2, unique_symbols: 2, analysis_types: [], top_models: [] },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/reports"]}>
      <AntApp>
        <ReportHistoryPage />
      </AntApp>
    </MemoryRouter>,
  );

  await waitFor(() => {
    expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
    expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
  });

  fireEvent.input(screen.getByPlaceholderText("开始日期"), { target: { value: "2026-06-23" } });
  fireEvent.input(screen.getByPlaceholderText("结束日期"), { target: { value: "2026-06-23" } });
  fireEvent.click(screen.getByLabelText("查询报告"));

  expect(screen.queryByText("贵州茅台 个股综合分析")).not.toBeInTheDocument();
  expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
});

test("报告历史页查询时只调用 search_reports 并展示报告范围结果", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "report_list") {
      return {
        code: 0,
        message: "ok",
        data: { items: [] },
      };
    }
    if (command === "report_stats") {
      return {
        code: 0,
        message: "ok",
        data: { total: 0, unique_symbols: 0, analysis_types: [], top_models: [] },
      };
    }
    if (command === "search_reports") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "report:7",
            doc_type: "report",
            ref_id: "7",
            symbol: "CN:SH:600519",
            title: "贵州茅台 个股综合分析",
            summary: "消费复苏与估值波动",
            source: "analysis_report",
            source_time: "2026-06-22T09:00:00Z",
            score: 1,
            highlights: ["消费复苏"],
          },
        ],
      };
    }
    if (command === "report_delete") {
      const deletePayload = payload as { id: number };
      return {
        code: 0,
        message: "ok",
        data: { id: deletePayload.id },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/reports"]}>
      <AntApp>
        <ReportHistoryPage />
      </AntApp>
    </MemoryRouter>,
  );

  fireEvent.change(screen.getByPlaceholderText("输入股票名称 / 代码 / 拼音"), { target: { value: "茅台" } });
  fireEvent.click(screen.getByLabelText("查询报告"));

  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "search_reports",
      payload: {
        payload: {
          keyword: "茅台",
          symbols: [],
          limit: 20,
          offset: 0,
          sort: "relevance",
        },
      },
    });
  });
  await waitFor(() => {
    expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
  });
  expect(screen.getByText("消费复苏与估值波动")).toBeInTheDocument();
  fireEvent.click(screen.getByLabelText("删除 贵州茅台 个股综合分析"));
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "report_delete", payload: { id: 7 } });
  });
  expect(screen.queryByText("贵州茅台 个股综合分析")).not.toBeInTheDocument();
  expect(screen.queryByText("资讯结果")).not.toBeInTheDocument();
  expect(screen.queryByText("自选备注")).not.toBeInTheDocument();
  expect(calls.map((call) => call.command)).not.toContain("search_news");
  expect(calls.map((call) => call.command)).not.toContain("search_watchlist_notes");
  expect(calls.map((call) => call.command)).not.toContain("search_global");
});

test("报告历史页清空关键字查询后恢复完整报告列表", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "report_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 7,
              task_id: "analysis-7",
              symbol: "CN:SH:600519",
              title: "贵州茅台 个股综合分析",
              analysis_type: "stock_full",
              model_name: "DeepSeek-V3",
              risk_summary: "消费复苏与估值波动",
              created_at: "2026-06-22T09:00:00Z",
              updated_at: "2026-06-22T09:00:00Z",
            },
            {
              id: 8,
              task_id: "analysis-8",
              symbol: "CN:SZ:002409",
              title: "雅克科技 技术面分析",
              analysis_type: "technical",
              model_name: "Qwen2.5-72B",
              risk_summary: "趋势反转失败",
              created_at: "2026-06-23T10:00:00Z",
              updated_at: "2026-06-23T10:00:00Z",
            },
          ],
        },
      };
    }
    if (command === "report_stats") {
      return { code: 0, message: "ok", data: { total: 2, unique_symbols: 2, analysis_types: [], top_models: [] } };
    }
    if (command === "search_reports") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "report:7",
            doc_type: "report",
            ref_id: "7",
            symbol: "CN:SH:600519",
            title: "贵州茅台 个股综合分析",
            summary: "消费复苏与估值波动",
            source: "DeepSeek-V3",
            source_time: "2026-06-22T09:00:00Z",
            score: 1,
            highlights: ["消费复苏"],
          },
        ],
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/reports"]}>
      <AntApp>
        <ReportHistoryPage />
      </AntApp>
    </MemoryRouter>,
  );

  await waitFor(() => {
    expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
  });

  const keywordInput = screen.getByPlaceholderText("输入股票名称 / 代码 / 拼音");
  fireEvent.change(keywordInput, { target: { value: "茅台" } });
  fireEvent.click(screen.getByLabelText("查询报告"));

  await waitFor(() => {
    expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
  });

  fireEvent.change(keywordInput, { target: { value: "" } });
  fireEvent.click(screen.getByLabelText("查询报告"));

  await waitFor(() => {
    expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
  });
  expect(calls.filter((call) => call.command === "report_list")).toHaveLength(2);
});

test("报告历史页清空关键字但全量刷新失败时不提示查询完成", async () => {
  const message = {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
  useAppSpy = vi.spyOn(AntApp, "useApp").mockReturnValue({
    message: message as unknown as ReturnType<typeof AntApp.useApp>["message"],
    notification: {} as ReturnType<typeof AntApp.useApp>["notification"],
    modal: {} as ReturnType<typeof AntApp.useApp>["modal"],
  });

  const calls: Array<{ command: string; payload?: unknown }> = [];
  let listCalls = 0;
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "report_list") {
      listCalls += 1;
      if (listCalls > 1) {
        throw new Error("报告列表读取失败");
      }
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 8,
              task_id: "analysis-8",
              symbol: "CN:SZ:002409",
              title: "雅克科技 技术面分析",
              analysis_type: "technical",
              model_name: "Qwen2.5-72B",
              risk_summary: "趋势反转失败",
              created_at: "2026-06-23T10:00:00Z",
              updated_at: "2026-06-23T10:00:00Z",
            },
          ],
        },
      };
    }
    if (command === "report_stats") {
      return { code: 0, message: "ok", data: { total: 1, unique_symbols: 1, analysis_types: [], top_models: [] } };
    }
    if (command === "search_reports") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "report:7",
            doc_type: "report",
            ref_id: "7",
            symbol: "CN:SH:600519",
            title: "贵州茅台 个股综合分析",
            summary: "消费复苏与估值波动",
            source: "DeepSeek-V3",
            source_time: "2026-06-22T09:00:00Z",
            score: 1,
            highlights: ["消费复苏"],
          },
        ],
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/reports"]}>
      <AntApp>
        <ReportHistoryPage />
      </AntApp>
    </MemoryRouter>,
  );

  await waitFor(() => {
    expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
  });

  const keywordInput = screen.getByPlaceholderText("输入股票名称 / 代码 / 拼音");
  fireEvent.change(keywordInput, { target: { value: "茅台" } });
  fireEvent.click(screen.getByLabelText("查询报告"));

  await waitFor(() => {
    expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
  });
  message.success.mockClear();

  fireEvent.change(keywordInput, { target: { value: "" } });
  fireEvent.click(screen.getByLabelText("查询报告"));

  await waitFor(() => {
    expect(message.error).toHaveBeenCalledWith("报告列表读取失败");
  });
  expect(message.success).not.toHaveBeenCalledWith("查询完成");
  expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
  expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
  expect(calls.filter((call) => call.command === "report_list")).toHaveLength(2);
});

test("报告历史页重置筛选但全量刷新失败时不提示重置成功", async () => {
  const message = {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
  useAppSpy = vi.spyOn(AntApp, "useApp").mockReturnValue({
    message: message as unknown as ReturnType<typeof AntApp.useApp>["message"],
    notification: {} as ReturnType<typeof AntApp.useApp>["notification"],
    modal: {} as ReturnType<typeof AntApp.useApp>["modal"],
  });

  const calls: Array<{ command: string; payload?: unknown }> = [];
  let listCalls = 0;
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "report_list") {
      listCalls += 1;
      if (listCalls > 1) {
        throw new Error("报告列表读取失败");
      }
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 8,
              task_id: "analysis-8",
              symbol: "CN:SZ:002409",
              title: "雅克科技 技术面分析",
              analysis_type: "technical",
              model_name: "Qwen2.5-72B",
              risk_summary: "趋势反转失败",
              created_at: "2026-06-23T10:00:00Z",
              updated_at: "2026-06-23T10:00:00Z",
            },
          ],
        },
      };
    }
    if (command === "report_stats") {
      return { code: 0, message: "ok", data: { total: 1, unique_symbols: 1, analysis_types: [], top_models: [] } };
    }
    if (command === "search_reports") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "report:7",
            doc_type: "report",
            ref_id: "7",
            symbol: "CN:SH:600519",
            title: "贵州茅台 个股综合分析",
            summary: "消费复苏与估值波动",
            source: "DeepSeek-V3",
            source_time: "2026-06-22T09:00:00Z",
            score: 1,
            highlights: ["消费复苏"],
          },
        ],
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/reports"]}>
      <AntApp>
        <ReportHistoryPage />
      </AntApp>
    </MemoryRouter>,
  );

  await waitFor(() => {
    expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
  });

  const keywordInput = screen.getByPlaceholderText("输入股票名称 / 代码 / 拼音");
  fireEvent.change(keywordInput, { target: { value: "茅台" } });
  fireEvent.click(screen.getByLabelText("查询报告"));

  await waitFor(() => {
    expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
  });
  message.success.mockClear();

  fireEvent.click(screen.getByLabelText("重置筛选"));

  await waitFor(() => {
    expect(message.error).toHaveBeenCalledWith("报告列表读取失败");
  });
  expect(message.success).not.toHaveBeenCalledWith("筛选条件已重置");
  expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
  expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
  expect(calls.filter((call) => call.command === "report_list")).toHaveLength(2);
});

test("报告历史页远程搜索结果继续应用日期筛选", async () => {
  mockIPC((command) => {
    if (command === "report_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "report_stats") {
      return { code: 0, message: "ok", data: { total: 0, unique_symbols: 0, analysis_types: [], top_models: [] } };
    }
    if (command === "search_reports") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "report:7",
            doc_type: "report",
            ref_id: "7",
            symbol: "CN:SH:600519",
            title: "贵州茅台 个股综合分析",
            summary: "消费复苏与估值波动",
            source: "DeepSeek-V3",
            source_time: "2026-06-22T09:00:00Z",
            score: 1,
            highlights: ["消费复苏"],
          },
          {
            doc_uid: "report:8",
            doc_type: "report",
            ref_id: "8",
            symbol: "CN:SZ:002409",
            title: "雅克科技 技术面分析",
            summary: "趋势反转失败",
            source: "Qwen2.5-72B",
            source_time: "2026-06-23T10:00:00Z",
            score: 1,
            highlights: ["趋势"],
          },
        ],
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <MemoryRouter initialEntries={["/reports"]}>
      <AntApp>
        <ReportHistoryPage />
      </AntApp>
    </MemoryRouter>,
  );

  fireEvent.change(screen.getByPlaceholderText("输入股票名称 / 代码 / 拼音"), { target: { value: "分析" } });
  fireEvent.input(screen.getByPlaceholderText("开始日期"), { target: { value: "2026-06-22" } });
  fireEvent.input(screen.getByPlaceholderText("结束日期"), { target: { value: "2026-06-22" } });
  fireEvent.click(screen.getByLabelText("查询报告"));

  await waitFor(() => {
    expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
  });
  expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
});
