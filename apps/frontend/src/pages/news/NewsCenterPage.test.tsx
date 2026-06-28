/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../../test/setupDom";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { App as AntApp } from "antd";
import { MemoryRouter } from "react-router-dom";
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

function renderPage(initialEntry: string | { pathname: string; state?: unknown } = "/news") {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <AntApp>
        <NewsCenterPage />
      </AntApp>
    </MemoryRouter>,
  );
}

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
            ref_id: "9",
            url: "https://news.example.com/9",
            symbol: "CN:SZ:300308",
            title: "光模块订单增长",
            summary: "800G 需求拉动",
            source: "财联社",
            source_time: "2026-06-22T10:00:00Z",
            score: 1,
            highlights: ["光模块"],
            tags: ["CPO", "光模块"],
            sentiment: "positive",
          },
        ],
      };
    }
    if (command === "open_external_url") {
      return { success: true };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

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
  expect(screen.getByText("CPO")).toBeInTheDocument();
  expect(screen.getByText("看涨")).toBeInTheDocument();
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

test("资讯中心后发个股资讯不会被较慢市场资讯覆盖", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  let resolveMarketNews: ((value: unknown) => void) | undefined;
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "watchlist_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [{ id: 1, symbol: "CN:SZ:300308", name: "中际旭创", code: "300308", sort_order: 1, tags: null, note: null }],
        },
      };
    }
    if (command === "news_market") {
      return new Promise((resolve) => {
        resolveMarketNews = resolve;
      });
    }
    if (command === "news_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: "stock-news",
            source: "财联社",
            title: "中际旭创个股资讯",
            summary: "个股摘要",
            symbols: ["CN:SZ:300308"],
            tags: ["公司"],
          }],
        },
      };
    }
    if (command === "news_stats") {
      return { code: 0, message: "ok", data: { total_count: 0, source_count: 0, sentiment_summary: "暂未接入情绪分类" } };
    }
    if (command === "news_hot_topics") {
      return { code: 0, message: "ok", data: { industries: [], mentioned_stocks: [] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  openCombobox(0);
  await waitFor(() => {
    expect(Array.from(document.querySelectorAll(".ant-select-item-option")).some((item) => item.textContent === "中际旭创")).toBe(true);
  });
  clickSelectOption("中际旭创");

  expect(await screen.findByText("中际旭创个股资讯")).toBeInTheDocument();
  resolveMarketNews?.({
    code: 0,
    message: "ok",
    data: {
      items: [{
        id: "market-news",
        source: "新浪财经",
        title: "较慢返回的市场资讯",
        summary: "市场摘要",
        tags: ["市场"],
      }],
    },
  });

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "news_market", payload: { market: "CN", limit: 80 } });
  });
  expect(screen.getByText("中际旭创个股资讯")).toBeInTheDocument();
  expect(screen.queryByText("较慢返回的市场资讯")).not.toBeInTheDocument();
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

  renderPage();

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

test("资讯中心搜索结果缺少高亮字段时仍展示资讯", async () => {
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
            doc_uid: "news:null-highlights",
            doc_type: "news",
            ref_id: "17",
            symbol: "CN:SH:600000",
            title: "伊朗相关资讯",
            summary: "搜索结果摘要",
            source: "财联社",
            source_time: "2026-06-28T09:19:00Z",
            score: 1,
            highlights: null,
          },
        ],
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

  await waitFor(() => {
    expect(calls.map((call) => call.command)).toEqual(expect.arrayContaining(["news_market", "news_stats", "news_hot_topics"]));
  });
  calls.length = 0;

  fireEvent.change(screen.getByPlaceholderText("输入关键词，支持标题/摘要"), { target: { value: "伊朗" } });
  await waitFor(() => {
    expect(screen.getByPlaceholderText("输入关键词，支持标题/摘要")).toHaveValue("伊朗");
  });
  fireEvent.click(screen.getByRole("button", { name: /刷新资讯/ }));

  await waitFor(() => {
    expect(screen.getByText("伊朗相关资讯")).toBeInTheDocument();
  });
  expect(screen.getByText("搜索结果摘要")).toBeInTheDocument();
  expect(calls.map((call) => call.command)).toContain("search_news");
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
              sentiment: request.forceRefresh ? "positive" : "neutral",
            },
          ],
        },
      };
    }
    if (command === "news_stats") {
      return {
        code: 0,
        message: "ok",
        data: {
          total_count: 1,
          source_count: 1,
          latest_published_at: "2026-06-27T08:10:00Z",
          sentiment_positive_count: 1,
          sentiment_neutral_count: 0,
          sentiment_negative_count: 0,
          sentiment_summary: "利好 1 条，中性 0 条，利空 0 条。",
        },
      };
    }
    if (command === "news_hot_topics") {
      return { code: 0, message: "ok", data: { industries: [], mentioned_stocks: [], updated_at: "2026-06-27T08:10:00Z" } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage();

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
  expect(screen.getByText("看涨")).toBeInTheDocument();
  expect(calls).toContainEqual({ command: "news_market", payload: { market: "CN", limit: 80, forceRefresh: true } });
});

test("资讯中心从分析页来源股票进入时读取该股票资讯且关联股票可搜索切换", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "watchlist_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            { id: 1, symbol: "CN:SH:600519", name: "贵州茅台", code: "600519", sort_order: 1, tags: null, note: null },
            { id: 2, symbol: "CN:SZ:300308", name: "中际旭创", code: "300308", sort_order: 2, tags: null, note: null },
          ],
        },
      };
    }
    if (command === "news_list") {
      const request = payload as { symbol: string };
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: request.symbol === "CN:SZ:301217" ? 301217 : 300308,
              source: "东方财富公告",
              title: request.symbol === "CN:SZ:301217" ? "铜冠铜箔来源股票资讯" : "中际旭创搜索股票资讯",
              summary: "个股摘要",
              symbols: [request.symbol],
              tags: ["公司"],
              sentiment: "neutral",
            },
          ],
        },
      };
    }
    if (command === "news_stats") {
      return { code: 0, message: "ok", data: { total_count: 0, source_count: 0, sentiment_summary: "暂未接入情绪分类" } };
    }
    if (command === "news_hot_topics") {
      return { code: 0, message: "ok", data: { industries: [], mentioned_stocks: [] } };
    }
    if (command === "stock_search") {
      return { code: 0, message: "ok", data: [{ symbol: "CN:SZ:300308", name: "中际旭创", code: "300308", market: "CN", exchange: "SZ" }] };
    }
    throw new Error(`unexpected command ${command}`);
  });

  renderPage({
    pathname: "/news",
    state: { newsStock: { symbol: "CN:SZ:301217", name: "铜冠铜箔", code: "301217" } },
  });

  expect(await screen.findByText("铜冠铜箔来源股票资讯")).toBeInTheDocument();
  expect(calls).toContainEqual({ command: "watchlist_list", payload: {} });
  expect(calls).toContainEqual({ command: "news_list", payload: { symbol: "CN:SZ:301217", limit: 80 } });

  calls.length = 0;

  openCombobox(0);
  await waitFor(() => {
    expect(Array.from(document.querySelectorAll(".ant-select-item-option")).some((item) => item.textContent === "中际旭创")).toBe(true);
  });
  clickSelectOption("中际旭创");

  await waitFor(() => {
    expect(screen.getByText("中际旭创搜索股票资讯")).toBeInTheDocument();
  });
  expect(calls).toContainEqual({ command: "news_list", payload: { symbol: "CN:SZ:300308", limit: 80 } });
});

function openCombobox(index: number) {
  const combobox = screen.getAllByRole("combobox")[index];
  if (!combobox) {
    throw new Error(`missing combobox ${index}`);
  }
  fireEvent.mouseDown(combobox);
}

function clickSelectOption(label: string) {
  const option = Array.from(document.querySelectorAll(".ant-select-item-option")).find((item) => item.textContent === label);
  if (!option) {
    throw new Error(`missing option ${label}`);
  }
  fireEvent.click(option);
}
