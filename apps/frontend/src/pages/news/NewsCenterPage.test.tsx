/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { afterEach, expect, test, vi } from "vitest";
import { NewsCenterPage } from "./NewsCenterPage";

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

test("资讯中心刷新搜索时只调用 search_news 并展示资讯范围结果", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "search_news") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "news:9",
            doc_type: "news",
            ref_id: "https://news.example.com/9",
            symbol: "CN:SZ:300308",
            title: "光模块订单增长",
            summary: "800G 需求拉动",
            source: "财联社",
            source_time: "2026-06-22T10:00:00Z",
            score: 1,
            highlights: ["光模块"],
          },
        ],
      };
    }
    if (command === "open_external_url") {
      return { success: true };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <NewsCenterPage />
    </AntApp>,
  );

  fireEvent.change(screen.getByPlaceholderText("输入关键词，支持标题/摘要"), { target: { value: "光模块" } });
  fireEvent.click(screen.getByRole("button", { name: /刷新资讯/ }));

  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "search_news",
      payload: {
        payload: {
          keyword: "光模块",
          symbols: [],
          limit: 20,
          offset: 0,
          sort: "relevance",
        },
      },
    });
  });
  expect(screen.getByText("光模块订单增长")).toBeInTheDocument();
  expect(screen.getByText("800G 需求拉动")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /查看原文/ }));
  expect(calls).toContainEqual({
    command: "open_external_url",
    payload: { url: "https://news.example.com/9" },
  });
  expect(screen.queryByText("贵州茅台 个股综合分析")).not.toBeInTheDocument();
  expect(screen.queryByText("自选备注")).not.toBeInTheDocument();
  expect(calls.map((call) => call.command)).not.toContain("search_reports");
  expect(calls.map((call) => call.command)).not.toContain("search_watchlist_notes");
  expect(calls.map((call) => call.command)).not.toContain("search_global");
});

test("资讯中心搜索无结果时说明仅搜索资讯中心", async () => {
  mockIPC((command) => {
    if (command === "search_news") {
      return { code: 0, message: "ok", data: [] };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <NewsCenterPage />
    </AntApp>,
  );

  fireEvent.change(screen.getByPlaceholderText("输入关键词，支持标题/摘要"), { target: { value: "不存在" } });
  fireEvent.click(screen.getByRole("button", { name: /刷新资讯/ }));

  await waitFor(() => {
    expect(screen.getByText("仅搜索资讯中心，暂无匹配资讯")).toBeInTheDocument();
  });
});
