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

afterEach(() => {
  clearMocks();
  cleanup();
});

test("报告历史页查询时只调用 search_reports 并展示报告范围结果", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
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
  expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
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
