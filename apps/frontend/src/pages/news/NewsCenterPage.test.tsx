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
    if (command === "news_market") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "news_stats") {
      return {
        code: 0,
        message: "ok",
        data: { total_count: 0, source_count: 0, sentiment_summary: "暂未接入情绪分类" },
      };
    }
    if (command === "news_hot_topics") {
      return { code: 0, message: "ok", data: { industries: [], mentioned_stocks: [] } };
    }
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

  await waitFor(() => {
    expect(calls.map((call) => call.command)).toEqual(expect.arrayContaining(["news_market", "news_stats", "news_hot_topics"]));
  });
  expect(screen.queryByText("按热度")).not.toBeInTheDocument();
  expect(screen.queryByText("按相关性")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "切换资讯视图" })).not.toBeInTheDocument();
  calls.length = 0;

  fireEvent.change(screen.getByPlaceholderText("输入关键词，支持标题/摘要"), { target: { value: "光模块" } });
  await waitFor(() => {
    expect(screen.getByPlaceholderText("输入关键词，支持标题/摘要")).toHaveValue("光模块");
  });
  fireEvent.click(screen.getByRole("button", { name: /刷新资讯/ }));

  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "search_news",
      payload: {
        payload: {
          keyword: "光模块",
          symbols: [],
          limit: 80,
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
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "news_market") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "news_stats") {
      return {
        code: 0,
        message: "ok",
        data: { total_count: 0, source_count: 0, sentiment_summary: "暂未接入情绪分类" },
      };
    }
    if (command === "news_hot_topics") {
      return { code: 0, message: "ok", data: { industries: [], mentioned_stocks: [] } };
    }
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

  await waitFor(() => {
    expect(calls.map((call) => call.command)).toEqual(expect.arrayContaining(["news_market", "news_stats", "news_hot_topics"]));
  });
  calls.length = 0;

  fireEvent.change(screen.getByPlaceholderText("输入关键词，支持标题/摘要"), { target: { value: "不存在" } });
  await waitFor(() => {
    expect(screen.getByPlaceholderText("输入关键词，支持标题/摘要")).toHaveValue("不存在");
  });
  fireEvent.click(screen.getByRole("button", { name: /刷新资讯/ }));

  await waitFor(() => {
    expect(calls.map((call) => call.command)).toContain("search_news");
    expect(screen.getByText("仅搜索资讯中心，暂无匹配资讯")).toBeInTheDocument();
  });
});

test("资讯中心手动刷新市场资讯时绕过缓存抓取最新数据", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "news_market") {
      const request = payload as { forceRefresh?: boolean };
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: request.forceRefresh ? "fresh" : "cached",
              source: "新浪财经",
              title: request.forceRefresh ? "最新市场资讯" : "缓存市场资讯",
              url: "https://example.com/news",
              summary: "市场摘要",
              published_at: request.forceRefresh ? "2026-06-27T08:10:00Z" : "2026-06-27T06:42:00Z",
              tags: ["市场"],
            },
          ],
        },
      };
    }
    if (command === "news_stats") {
      return {
        code: 0,
        message: "ok",
        data: { total_count: 1, source_count: 1, latest_published_at: "2026-06-27T08:10:00Z", sentiment_summary: "暂未接入情绪分类" },
      };
    }
    if (command === "news_hot_topics") {
      return { code: 0, message: "ok", data: { industries: [], mentioned_stocks: [], updated_at: "2026-06-27T08:10:00Z" } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(
    <AntApp>
      <NewsCenterPage />
    </AntApp>,
  );

  await waitFor(() => {
    expect(screen.getByText("缓存市场资讯")).toBeInTheDocument();
  });
  expect(calls).toContainEqual({ command: "news_market", payload: { market: "CN", limit: 80 } });
  calls.length = 0;

  const refreshButton = screen.getByRole("button", { name: /刷新资讯/ });
  await waitFor(() => {
    expect(refreshButton).not.toBeDisabled();
  });
  fireEvent.click(refreshButton);

  await waitFor(() => {
    expect(screen.getByText("最新市场资讯")).toBeInTheDocument();
  });
  expect(calls).toContainEqual({ command: "news_market", payload: { market: "CN", limit: 80, forceRefresh: true } });
});
