/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { App, AppErrorBoundary } from "./App";
import * as AppModule from "./App";

const dialogOpenMock = vi.hoisted(() => vi.fn());
const notificationIsPermissionGrantedMock = vi.hoisted(() => vi.fn());
const notificationRequestPermissionMock = vi.hoisted(() => vi.fn());
const notificationSendMock = vi.hoisted(() => vi.fn());

vi.mock("@tauri-apps/plugin-dialog", () => ({
  open: dialogOpenMock,
}));

vi.mock("@tauri-apps/plugin-notification", () => ({
  isPermissionGranted: notificationIsPermissionGrantedMock,
  requestPermission: notificationRequestPermissionMock,
  sendNotification: notificationSendMock,
}));

afterEach(() => {
  clearMocks();
  dialogOpenMock.mockReset();
  notificationIsPermissionGrantedMock.mockReset();
  notificationRequestPermissionMock.mockReset();
  notificationSendMock.mockReset();
  cleanup();
  window.location.hash = "";
  vi.useRealTimers();
});

const dashboardFixture = {
  watchlist: { up_count: 2, down_count: 1, flat_count: 0 },
  recent_reports: [
    { id: 1, title: "浦发银行分析", symbol: "600000.SH", risk_summary: "注意估值波动" },
  ],
  recent_tasks: [{ id: "task-1", title: "生成分析报告", status: "SUCCESS", progress: 100 }],
  market_news: [{ title: "A 股早盘新闻", source: "eastmoney", published_at: "2026-06-19T09:30:00Z" }],
  risk_tips: ["仅作研究辅助，不构成投资建议"],
  provider_statuses: [{ name: "market", source: "sina", available: true, last_error: "" }],
};

const emptyDashboardFixture = {
  watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
  recent_reports: [],
  recent_tasks: [],
  market_news: [],
  risk_tips: ["仅作研究辅助，不构成投资建议"],
  provider_statuses: [{ name: "market", source: "unconfigured", available: false, last_error: "provider_unconfigured" }],
};

test("首版主导航和路由范围只包含 MVP 页面", () => {
  const routeModule = AppModule as typeof AppModule & {
    APP_NAV_ITEMS?: unknown;
    APP_ROUTE_PATHS?: unknown;
  };

  expect(routeModule.APP_NAV_ITEMS).toEqual([
    { path: "/", label: "概览" },
    { path: "/watchlist", label: "自选股" },
    { path: "/analysis", label: "AI 分析" },
    { path: "/reports", label: "报告历史" },
    { path: "/news", label: "资讯中心" },
    { path: "/tasks", label: "任务历史" },
    { path: "/settings", label: "设置" },
  ]);
  expect(routeModule.APP_ROUTE_PATHS).toEqual([
    "/",
    "/watchlist",
    "/stocks/:symbol",
    "/news",
    "/scheduler",
    "/analysis",
    "/reports",
    "/tasks",
    "/settings",
    "/ai-settings",
  ]);
});

test("App 启动后通过 typed invoke service 展示 core health 状态", async () => {
  mockIPC((command) => {
    if (command === "dashboard_summary") {
      return { code: 0, message: "ok", data: dashboardFixture };
    }
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "market_quote") {
      return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 3181.3, change: -12.18, change_percent: -0.38 } };
    }
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
    expect(screen.getByText(/本地核心服务已连接/)).toBeInTheDocument();
  });
  expect(screen.getByText(/版本\s*0\.1\.0/)).toBeInTheDocument();
});

test("App 提供基础路由外壳和错误边界", async () => {
  mockIPC((command) => {
    if (command === "dashboard_summary") {
      return { code: 0, message: "ok", data: dashboardFixture };
    }
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "market_quote") {
      return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 3181.3, change: -12.18, change_percent: -0.38 } };
    }
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

  expect(screen.getByRole("navigation", { name: "主导航" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "概览" })).toHaveAttribute("href", "#/");
  expect(screen.getByRole("link", { name: "自选股" })).toHaveAttribute("href", "#/watchlist");
  expect(screen.getByRole("link", { name: "AI 分析" })).toHaveAttribute("href", "#/analysis");
  expect(screen.getByRole("link", { name: "资讯中心" })).toHaveAttribute("href", "#/news");

  await waitFor(() => {
    expect(screen.getByText(/本地核心服务已连接/)).toBeInTheDocument();
  });
});

test("Dashboard 首页按截图结构展示总览桌面", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "dashboard_summary":
        return {
          code: 0,
          message: "ok",
          data: dashboardFixture,
        };
      case "watchlist_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 1, symbol: "600519.SH", sort_order: 10 },
              { id: 2, symbol: "300750.SZ", sort_order: 20 },
            ],
          },
        };
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: args.symbol,
            price: args.symbol === "600519.SH" ? 1647.03 : args.symbol === "300750.SZ" ? 182.65 : 3181.3,
            change: args.symbol === "300750.SZ" ? 2.31 : -12.18,
            change_percent: args.symbol === "300750.SZ" ? 1.28 : -0.38,
            quote_time: "2024-05-20T16:00:05+08:00",
            provider: "sina",
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "欢迎使用投研罗盘" })).toBeInTheDocument();
  });
  expect(screen.getByPlaceholderText("搜索股票名称 / 代码 / 拼音")).toBeInTheDocument();
  expect(screen.getByText("市场数据已更新")).toBeInTheDocument();
  expect(screen.getByText("市场概览")).toBeInTheDocument();
  expect(screen.getByText("我的自选")).toBeInTheDocument();
  expect(screen.getByText("最近报告")).toBeInTheDocument();
  expect(screen.getByText("最近任务")).toBeInTheDocument();
  expect(screen.getAllByText("600519.SH").length).toBeGreaterThan(0);
  expect(screen.getAllByText("300750.SZ").length).toBeGreaterThan(0);
  expect(screen.queryByText("贵州茅台")).not.toBeInTheDocument();
  expect(screen.queryByText("宁德时代")).not.toBeInTheDocument();
  expect(screen.getByText("北向净流入")).toBeInTheDocument();
  expect(screen.getByText("暂未接入")).toBeInTheDocument();
  expect(screen.getByText("仅供研究，不构成投资建议")).toBeInTheDocument();
  expect(screen.getByText("仅作研究辅助，不构成投资建议")).toBeInTheDocument();
  expect(document.querySelector(".ant-badge-count")).not.toBeInTheDocument();
  expect(calls.map((call) => call.command)).toContain("dashboard_summary");
  expect(calls.map((call) => call.command)).toContain("watchlist_list");
});

test("Dashboard 顶部在行情全部失败时不展示市场数据已更新", async () => {
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "dashboard_summary":
        return {
          code: 0,
          message: "ok",
          data: dashboardFixture,
        };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 50000, message: "provider unavailable", data: null };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "欢迎使用投研罗盘" })).toBeInTheDocument();
  });
  expect(screen.queryByText("市场数据已更新")).not.toBeInTheDocument();
  expect(screen.getByText("市场数据暂不可用")).toBeInTheDocument();
  expect(screen.getByText("--:--:--")).toBeInTheDocument();
});

test("顶部搜索通过后端股票搜索跳转到首个真实结果", async () => {
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: emptyDashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote": {
        const args = payload as { symbol?: string };
        return { code: 0, message: "ok", data: { symbol: args.symbol, price: 0, change_percent: 0 } };
      }
      case "stock_search":
        return {
          code: 0,
          message: "ok",
          data: [{ symbol: "600000.SH", name: "浦发银行", code: "600000", market: "CN", exchange: "SH" }],
        };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: {} } };
      case "news_list":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  const search = await screen.findByPlaceholderText("搜索股票名称 / 代码 / 拼音");
  fireEvent.change(search, { target: { value: "浦发银行" } });
  fireEvent.keyDown(search, { key: "Enter", code: "Enter" });

  await waitFor(() => {
    expect(window.location.hash).toBe("#/stocks/600000.SH");
  });
  expect(calls).toContainEqual({ command: "stock_search", payload: { keyword: "浦发银行" } });
});

test("Dashboard 首页展示无数据和 Provider 异常状态", async () => {
  mockIPC((command) => {
    if (command === "dashboard_summary") {
      return { code: 0, message: "ok", data: emptyDashboardFixture };
    }
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "market_quote") {
      return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change: 0, change_percent: 0 } };
    }
    return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "欢迎使用投研罗盘" })).toBeInTheDocument();
  });
  expect(screen.getByText("暂无分析报告")).toBeInTheDocument();
  expect(screen.getByText("暂无任务记录")).toBeInTheDocument();
  expect(screen.getByText("不可用")).toBeInTheDocument();
  expect(screen.getByText("provider_unconfigured")).toBeInTheDocument();
});

test("自选股页面读取真实列表行情并支持搜索添加、更新和删除后重加", async () => {
  window.location.hash = "#/watchlist";
  const calls: Array<{ command: string; payload?: any }> = [];
  let items = [{ id: 1, symbol: "600000.SH", sort_order: 10, tags: ["银行"], note: "低估值观察" }];
  let quoteRefreshes = 0;

  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items } };
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: args.symbol,
            price: args.symbol === "600000.SH" ? (quoteRefreshes > 0 ? 7.25 : 7.12) : 9.88,
            change_percent: args.symbol === "600000.SH" ? 1.42 : -0.3,
            quote_time: "2026-06-19T10:00:00Z",
            provider: "sina",
          },
        };
      case "stock_search":
        return {
          code: 0,
          message: "ok",
          data: [{ symbol: "600001.SH", name: "邯郸钢铁", code: "600001", market: "CN", exchange: "SH" }],
        };
      case "watchlist_create": {
        const created = { id: 2, symbol: args.payload.symbol, sort_order: 20, tags: ["钢铁"], note: "重加观察" };
        items = [...items, created];
        return { code: 0, message: "ok", data: created };
      }
      case "watchlist_update": {
        items = items.map((item) =>
          item.id === args.payload.id
            ? { ...item, sort_order: args.payload.sort_order, tags: args.payload.tags, note: args.payload.note }
            : item,
        );
        return { code: 0, message: "ok", data: items.find((item) => item.id === args.payload.id) };
      }
      case "watchlist_delete":
        items = items.filter((item) => item.id !== args.id);
        return { code: 0, message: "ok", data: { id: args.id } };
      case "scheduler_refresh_symbol":
        quoteRefreshes += 1;
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 101,
                run_key: "refresh-600000",
                status: "success",
                data_type: "quote",
                scope_key: args.payload.symbol,
              },
            ],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "自选股" })).toBeInTheDocument();
  });
  expect(screen.getByText("600000.SH")).toBeInTheDocument();
  expect(screen.getByText("7.12")).toBeInTheDocument();
  expect(screen.getByText("1.42%")).toBeInTheDocument();
  expect(calls.some((call) => call.command === "market_quote" && call.payload?.symbol === "600000.SH")).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "刷新 600000.SH" }));
  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "scheduler_refresh_symbol",
      payload: { payload: { symbol: "600000.SH", data_type: "quote" } },
    });
  });
  await waitFor(() => {
    expect(screen.getByText("7.25")).toBeInTheDocument();
  });

  fireEvent.change(screen.getByLabelText("搜索股票"), { target: { value: "邯郸" } });
  fireEvent.click(screen.getByRole("button", { name: "搜索" }));

  await waitFor(() => {
    expect(screen.getByText("邯郸钢铁")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "添加 600001.SH" }));

  await waitFor(() => {
    expect(screen.getByText("600001.SH")).toBeInTheDocument();
  });

  fireEvent.change(screen.getByLabelText("排序 600001.SH"), { target: { value: "30" } });
  fireEvent.change(screen.getByLabelText("标签 600001.SH"), { target: { value: "钢铁,低估值" } });
  fireEvent.change(screen.getByLabelText("备注 600001.SH"), { target: { value: "重加观察更新" } });
  fireEvent.click(screen.getByRole("button", { name: "保存 600001.SH" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "watchlist_update")).toBe(true);
  });

  fireEvent.click(screen.getByRole("button", { name: "删除 600001.SH" }));

  await waitFor(() => {
    expect(screen.queryByText("600001.SH")).not.toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: "添加 600001.SH" }));

  await waitFor(() => {
    expect(screen.getByText("600001.SH")).toBeInTheDocument();
  });
});

test("自选股页面重复添加时显示后端业务错误", async () => {
  window.location.hash = "#/watchlist";
  mockIPC((command) => {
    switch (command) {
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "stock_search":
        return {
          code: 0,
          message: "ok",
          data: [{ symbol: "600000.SH", name: "浦发银行", code: "600000", market: "CN", exchange: "SH" }],
        };
      case "watchlist_create":
        return { code: 40005, message: "watchlist_symbol_duplicate", data: null };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "自选股" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("搜索股票"), { target: { value: "浦发" } });
  fireEvent.click(screen.getByRole("button", { name: "搜索" }));

  await waitFor(() => {
    expect(screen.getByText("浦发银行")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "添加 600000.SH" }));

  await waitFor(() => {
    expect(screen.getByText("watchlist_symbol_duplicate")).toBeInTheDocument();
  });
});

test("个股详情页展示行情、K 线、指标和 HTTPS 新闻，并支持 period/adjust 切换", async () => {
  window.location.hash = "#/stocks/600000.SH";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: args.symbol,
            price: 7.12,
            change_percent: 1.42,
            open: 7.01,
            high: 7.2,
            low: 6.98,
            pre_close: 7.02,
            volume: 1234000,
            amount: 8780000,
            quote_time: "2026-06-19T10:00:00Z",
            provider: "sina",
          },
        };
      case "market_kline":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                symbol: args.symbol,
                period: args.period,
                adjust: args.adjust,
                trade_date: "2026-06-18",
                open: 7.01,
                high: 7.1,
                low: 6.98,
                close: 7.05,
                volume: 1000000,
                amount: 7050000,
                provider: "tencent",
              },
              {
                symbol: args.symbol,
                period: args.period,
                adjust: args.adjust,
                trade_date: "2026-06-19",
                open: 7.05,
                high: 7.2,
                low: 7.02,
                close: 7.12,
                volume: 1234000,
                amount: 8780000,
                provider: "tencent",
              },
            ],
          },
        };
      case "market_indicators":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: args.symbol,
            period: args.period,
            adjust: args.adjust,
            indicators: {
              ma: { ma5: [7.01, 7.12] },
              rsi: [54.8, 55.1],
            },
          },
        };
      case "news_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                source: "eastmoney",
                title: "浦发银行 HTTPS 新闻",
                url: "https://example.com/news/1",
                summary: "银行板块动态",
                published_at: "2026-06-19T10:00:00Z",
              },
              {
                id: 2,
                source: "unsafe",
                title: "浦发银行 HTTP 新闻",
                url: "http://example.com/news/2",
                summary: "不应展示",
              },
            ],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "600000.SH" })).toBeInTheDocument();
  });
  expect(screen.getAllByText("7.12").length).toBeGreaterThan(0);
  expect(screen.getByText("1.42%")).toBeInTheDocument();
  expect(screen.getByText("2026-06-19")).toBeInTheDocument();
  expect(screen.getByText("ma.ma5")).toBeInTheDocument();
  expect(screen.getByText("55.1")).toBeInTheDocument();
  expect(screen.getByText("浦发银行 HTTPS 新闻")).toBeInTheDocument();
  expect(screen.queryByText("浦发银行 HTTP 新闻")).not.toBeInTheDocument();

  fireEvent.change(screen.getByLabelText("周期"), { target: { value: "week" } });
  fireEvent.change(screen.getByLabelText("复权"), { target: { value: "hfq" } });

  await waitFor(() => {
    expect(
      calls.some(
        (call) =>
          call.command === "market_kline" &&
          call.payload?.period === "week" &&
          call.payload?.adjust === "hfq",
      ),
    ).toBe(true);
  });
});

test("个股详情页支持刷新当前股票并复用调度队列", async () => {
  window.location.hash = "#/stocks/600000.SH";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: { symbol: args.symbol, price: 7.12, change_percent: 1.42, provider: "sina" },
        };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: args.symbol, period: args.period, adjust: args.adjust, indicators: {} } };
      case "news_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "scheduler_refresh_symbol":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 19,
                run_key: "user_request:quote:CN:SH:600000:2026-06-19:1",
                trigger_type: "user_request",
                status: "running",
              },
            ],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "600000.SH" })).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "刷新单股" }));

  await waitFor(() => {
    expect(screen.getByText("正在刷新当前股票数据")).toBeInTheDocument();
  });
  expect(calls).toContainEqual({
    command: "scheduler_refresh_symbol",
    payload: {
      payload: {
        symbol: "600000.SH",
        data_type: "all",
        period: "day",
        adjust: "qfq",
        limit: 120,
      },
    },
  });
});

test("个股详情页刷新完成后重新读取当前缓存", async () => {
  window.location.hash = "#/stocks/600000.SH";
  const calls: Array<{ command: string; payload?: any }> = [];
  let price = 7.12;
  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: { symbol: args.symbol, price, change_percent: 1.42, provider: "sina" },
        };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: args.symbol, period: args.period, adjust: args.adjust, indicators: {} } };
      case "news_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "scheduler_refresh_symbol":
        price = 7.35;
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 19,
                run_key: "user_request:quote:CN:SH:600000:2026-06-19:1",
                trigger_type: "user_request",
                status: "success",
              },
            ],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "600000.SH" })).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "刷新单股" }));

  await waitFor(() => {
    expect(screen.getByText("刷新完成，已读取最新缓存")).toBeInTheDocument();
  });
  expect(screen.getAllByText("7.35").length).toBeGreaterThan(0);
  expect(calls.filter((call) => call.command === "market_quote")).toHaveLength(2);
});

test("个股详情页在 K 线和新闻为空时展示明确空状态", async () => {
  window.location.hash = "#/stocks/600000.SH";
  mockIPC((command, payload) => {
    const args = payload as any;
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: { symbol: args.symbol, price: 7.12, provider: "sina" },
        };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_indicators":
        return {
          code: 0,
          message: "ok",
          data: { symbol: args.symbol, period: args.period, adjust: args.adjust, indicators: {} },
        };
      case "news_list":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("暂无 K 线数据")).toBeInTheDocument();
  });
  expect(screen.getByText("暂无技术指标")).toBeInTheDocument();
  expect(screen.getByText("暂无相关新闻")).toBeInTheDocument();
});

test("资讯中心读取市场和个股新闻，过滤非 HTTPS 外链并支持标签筛选", async () => {
  window.location.hash = "#/news";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "news_market":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                source: "eastmoney",
                title: "A 股午间资讯",
                url: "https://example.com/market/1",
                summary: "市场情绪回暖",
                published_at: "2026-06-19T11:30:00Z",
                tags: ["市场"],
              },
            ],
          },
        };
      case "news_list":
        expect(args.symbol).toBe("600000.SH");
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 2,
                source: "eastmoney",
                title: "浦发银行 HTTPS 新闻",
                url: "https://example.com/symbol/2",
                summary: "银行板块动态",
                published_at: "2026-06-19T10:00:00Z",
                symbols: ["600000.SH"],
                tags: ["银行"],
              },
              {
                id: 3,
                source: "unsafe",
                title: "浦发银行 HTTP 新闻",
                url: "http://example.com/symbol/3",
                summary: "不应展示",
                symbols: ["600000.SH"],
                tags: ["银行"],
              },
            ],
          },
        };
      case "open_external_url":
        return { ok: true };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "资讯中心" })).toBeInTheDocument();
  });
  expect(screen.getByText("A 股午间资讯")).toBeInTheDocument();
  expect(screen.getByText("https://example.com/market/1")).toBeInTheDocument();
  expect(screen.queryByText("公告")).not.toBeInTheDocument();
  expect(screen.queryByText("研报")).not.toBeInTheDocument();
  expect(screen.queryByText("资金流")).not.toBeInTheDocument();

  fireEvent.change(screen.getByLabelText("股票代码"), { target: { value: "600000.SH" } });
  fireEvent.click(screen.getByRole("button", { name: "查询个股新闻" }));

  await waitFor(() => {
    expect(screen.getByText("浦发银行 HTTPS 新闻")).toBeInTheDocument();
  });
  expect(screen.queryByText("浦发银行 HTTP 新闻")).not.toBeInTheDocument();
  expect(calls).toContainEqual({ command: "news_market", payload: { market: "CN", limit: 50 } });
  expect(calls).toContainEqual({ command: "news_list", payload: { symbol: "600000.SH", limit: 50 } });

  fireEvent.change(screen.getByLabelText("标签筛选"), { target: { value: "银行" } });

  await waitFor(() => {
    expect(screen.queryByText("A 股午间资讯")).not.toBeInTheDocument();
  });
  expect(screen.getByText("浦发银行 HTTPS 新闻")).toBeInTheDocument();
  fireEvent.click(screen.getAllByRole("button", { name: "打开新闻外链" })[0]);
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "open_external_url", payload: { url: "https://example.com/symbol/2" } });
  });
});

test("资讯中心展示空状态和读取失败状态", async () => {
  window.location.hash = "#/news";
  mockIPC((command) => {
    if (command === "news_market") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("暂无资讯")).toBeInTheDocument();
  });

  cleanup();
  clearMocks();
  window.location.hash = "#/news";
  mockIPC((command) => {
    if (command === "news_market") {
      return { code: 50201, message: "news_provider_unavailable", data: null };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("资讯读取失败")).toBeInTheDocument();
  });
  expect(screen.getByText("news_provider_unavailable")).toBeInTheDocument();
});

test("模型配置页保存 API Key 后只展示脱敏字段并清空明文输入", async () => {
  window.location.hash = "#/ai-settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "prompt_templates_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "ai_config_save":
        return {
          code: 0,
          message: "ok",
          data: {
            config: {
              ...args.payload,
              id: 3,
              api_key_ref: "local-vault://ai-config/custom-3",
              masked_api_key: "sk-...cret",
              has_api_key: true,
              api_key: undefined,
            },
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "模型配置" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("配置名称"), { target: { value: "自定义接入点" } });
  fireEvent.change(screen.getByLabelText("接入点"), { target: { value: "https://llm.example.com" } });
  fireEvent.change(screen.getByLabelText("模型名称"), { target: { value: "gpt-4.1-mini" } });
  fireEvent.change(screen.getByLabelText("API Key"), { target: { value: "sk-live-secret" } });
  fireEvent.click(screen.getByRole("button", { name: "保存模型配置" }));

  await waitFor(() => {
    expect(screen.getByText("sk-...cret")).toBeInTheDocument();
  });
  expect(screen.queryByText("sk-live-secret")).not.toBeInTheDocument();
  expect(screen.queryByDisplayValue("sk-live-secret")).not.toBeInTheDocument();
  expect(calls.some((call) => call.command === "ai_config_save" && call.payload?.payload?.api_key === "sk-live-secret")).toBe(true);
});

test("模型连通性测试失败时不把密钥文本展示到页面", async () => {
  window.location.hash = "#/ai-settings";
  mockIPC((command) => {
    switch (command) {
      case "ai_config_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                name: "OpenAI 主配置",
                provider: "openai-compatible",
                base_url: "https://api.openai.com",
                api_key_ref: "local-vault://ai-config/openai-1",
                masked_api_key: "sk-...abcd",
                has_api_key: true,
                model_name: "gpt-4.1-mini",
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 120,
                stream_enabled: true,
                is_default: true,
              },
            ],
          },
        };
      case "prompt_templates_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "ai_config_test":
        return { code: 50201, message: "upstream failed sk-live-secret", data: null };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("OpenAI 主配置")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "测试 OpenAI 主配置" }));

  await waitFor(() => {
    expect(screen.getByText(/upstream failed/)).toBeInTheDocument();
  });
  expect(screen.queryByText("sk-live-secret")).not.toBeInTheDocument();
});

test("Prompt 模板页拒绝未支持变量且不会提交创建 command", async () => {
  window.location.hash = "#/ai-settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "prompt_templates_list":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "Prompt 模板" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("模板名称"), { target: { value: "非法变量模板" } });
  fireEvent.change(screen.getByLabelText("模板内容"), { target: { value: "分析 {{unsupported_var}}" } });
  fireEvent.click(screen.getByRole("button", { name: "保存 Prompt 模板" }));

  await waitFor(() => {
    expect(screen.getByText("变量 unsupported_var 不在首版白名单")).toBeInTheDocument();
  });
  expect(calls.some((call) => call.command === "prompt_templates_create")).toBe(false);
});

test("AI 分析页面创建任务后订阅事件并展示最终报告", async () => {
  window.location.hash = "#/analysis";
  notificationIsPermissionGrantedMock.mockResolvedValue(true);
  const calls: Array<{ command: string; payload?: any }> = [];
  const originalBlob = globalThis.Blob;
  const exportedBlobs: Array<{ parts: string[]; type: string }> = [];
  const writeText = vi.fn<(text: string) => Promise<void>>(() => Promise.resolve());
  const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
  class TestBlob {
    parts: string[];
    type: string;

    constructor(parts: BlobPart[], options?: BlobPropertyBag) {
      this.parts = parts.map((part) => String(part));
      this.type = options?.type ?? "";
    }
  }
  Object.defineProperty(globalThis, "Blob", { value: TestBlob, configurable: true });
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  Object.defineProperty(URL, "createObjectURL", {
    value: vi.fn((blob: { parts: string[]; type: string }) => {
      exportedBlobs.push(blob);
      return "blob:analysis-report-markdown";
    }),
    configurable: true,
  });
  Object.defineProperty(URL, "revokeObjectURL", { value: vi.fn(), configurable: true });
  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "ai_config_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                name: "OpenAI 主配置",
                provider: "openai-compatible",
                base_url: "https://api.example.com",
                api_key_ref: "local-vault://ai-config/openai-1",
                masked_api_key: "sk-...abcd",
                has_api_key: true,
                model_name: "gpt-4.1-mini",
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 120,
                stream_enabled: true,
                is_default: true,
              },
            ],
          },
        };
      case "prompt_templates_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{ id: 7, name: "综合分析模板", type: "stock_full", description: "", content: "{{stock_name}}", variables: ["stock_name"], is_builtin: false }],
          },
        };
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [{ key: "notifications.task_terminal", value: "true" }] } };
      case "analysis_task_create":
        expect(args.payload).not.toHaveProperty("resolved_api_key");
        return { code: 0, message: "ok", data: { task_id: "analysis-1", status: "PENDING" } };
      case "analysis_task_subscribe":
        return { emitted: 3, last_event_id: 3 };
      case "task_events":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 1, event: "TASK_STARTED", data: { status: "RUNNING" } },
              { id: 2, event: "TASK_CHUNK", data: { content: "阶段观点" } },
              { id: 3, event: "TASK_SUCCESS", data: { report_id: 9 } },
            ],
          },
        };
      case "report_list":
        return { code: 0, message: "ok", data: { items: [{ id: 9, task_id: "analysis-1", symbol: "600000.SH", title: "浦发银行分析" }] } };
      case "report_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 9,
            task_id: "analysis-1",
            symbol: "600000.SH",
            title: "浦发银行分析",
            content_markdown: "## 结论\n保持观察",
            risk_summary: "波动风险",
            input_snapshot: "{\"user_position\":\"满仓\"}",
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "AI 分析" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("股票代码"), { target: { value: "600000.SH" } });
  fireEvent.click(screen.getByRole("button", { name: "开始分析" }));

  await waitFor(() => {
    expect(screen.getByText("阶段观点")).toBeInTheDocument();
  });
  expect(screen.getByText("## 结论")).toBeInTheDocument();
  expect(screen.getByText("保持观察")).toBeInTheDocument();
  expect(calls.map((call) => call.command)).toContain("analysis_task_subscribe");
  await waitFor(() => {
    expect(notificationSendMock).toHaveBeenCalledWith({
      title: "AI 分析已完成",
      body: "600000.SH 分析报告已生成",
    });
  });
  expect(notificationRequestPermissionMock).not.toHaveBeenCalled();

  fireEvent.click(screen.getByRole("button", { name: "复制 Markdown 9" }));
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("## 结论"));
  });
  expect(writeText.mock.calls[0][0]).not.toContain("input_snapshot");
  expect(writeText.mock.calls[0][0]).not.toContain("满仓");

  fireEvent.click(screen.getByRole("button", { name: "导出 Markdown 9" }));
  await waitFor(() => {
    expect(clickSpy).toHaveBeenCalled();
  });
  const exportedText = exportedBlobs[0].parts.join("");
  expect(exportedText).toContain("## 结论");
  expect(exportedText).not.toContain("input_snapshot");
  expect(exportedText).not.toContain("满仓");
  Object.defineProperty(globalThis, "Blob", { value: originalBlob, configurable: true });
  clickSpy.mockRestore();
});

test("AI 分析页面停止生成后展示取消状态", async () => {
  window.location.hash = "#/analysis";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "ai_config_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                name: "OpenAI 主配置",
                provider: "openai-compatible",
                base_url: "https://api.example.com",
                api_key_ref: "local-vault://ai-config/openai-1",
                masked_api_key: "sk-...abcd",
                has_api_key: true,
                model_name: "gpt-4.1-mini",
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 120,
                stream_enabled: true,
                is_default: true,
              },
            ],
          },
        };
      case "prompt_templates_list":
        return { code: 0, message: "ok", data: { items: [{ id: 7, name: "综合分析模板", type: "stock_full", description: "", content: "{{stock_name}}", variables: ["stock_name"], is_builtin: false }] } };
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [{ key: "notifications.task_terminal", value: "true" }] } };
      case "analysis_task_create":
        return { code: 0, message: "ok", data: { task_id: "analysis-2", status: "PENDING" } };
      case "analysis_task_subscribe":
        return { emitted: 1, last_event_id: 1 };
      case "task_events":
        return { code: 0, message: "ok", data: { items: [{ id: 1, event: "TASK_STARTED", data: { status: "RUNNING" } }] } };
      case "analysis_task_cancel":
        return { code: 0, message: "ok", data: { task_id: "analysis-2", status: "CANCELLED" } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "AI 分析" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("股票代码"), { target: { value: "600000.SH" } });
  fireEvent.click(screen.getByRole("button", { name: "开始分析" }));

  await waitFor(() => {
    expect(screen.getByRole("button", { name: "停止生成" })).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "停止生成" }));

  await waitFor(() => {
    expect(screen.getByText("CANCELLED")).toBeInTheDocument();
  });
  expect(calls.some((call) => call.command === "analysis_task_cancel" && call.payload?.taskId === "analysis-2")).toBe(true);
});

test("AI 分析页面展示失败任务事件的明确原因", async () => {
  window.location.hash = "#/analysis";
  notificationIsPermissionGrantedMock.mockResolvedValue(false);
  notificationRequestPermissionMock.mockResolvedValue("granted");
  mockIPC((command) => {
    switch (command) {
      case "ai_config_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                name: "OpenAI 主配置",
                provider: "openai-compatible",
                base_url: "https://api.example.com",
                api_key_ref: "local-vault://ai-config/openai-1",
                masked_api_key: "sk-...abcd",
                has_api_key: true,
                model_name: "gpt-4.1-mini",
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 120,
                stream_enabled: true,
                is_default: true,
              },
            ],
          },
        };
      case "prompt_templates_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{ id: 7, name: "综合分析模板", type: "stock_full", description: "", content: "{{stock_name}}", variables: ["stock_name"], is_builtin: false }],
          },
        };
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [{ key: "notifications.task_terminal", value: "true" }] } };
      case "analysis_task_create":
        return { code: 0, message: "ok", data: { task_id: "analysis-failed", status: "PENDING" } };
      case "analysis_task_subscribe":
        return { emitted: 2, last_event_id: 2 };
      case "task_events":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 1, event: "TASK_STARTED", data: { status: "RUNNING" } },
              { id: 2, event: "TASK_FAILED", data: { error_message: "provider timeout" } },
            ],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "AI 分析" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("股票代码"), { target: { value: "600000.SH" } });
  fireEvent.click(screen.getByRole("button", { name: "开始分析" }));

  await waitFor(() => {
    expect(screen.getByText("FAILED")).toBeInTheDocument();
  });
  expect(screen.getByText("provider timeout")).toBeInTheDocument();
  await waitFor(() => {
    expect(notificationSendMock).toHaveBeenCalledWith({
      title: "AI 分析失败",
      body: "provider timeout",
    });
  });
  expect(notificationRequestPermissionMock).toHaveBeenCalledTimes(1);
});

test("AI 分析页面尊重任务通知关闭设置", async () => {
  window.location.hash = "#/analysis";
  notificationIsPermissionGrantedMock.mockResolvedValue(true);
  mockIPC((command) => {
    switch (command) {
      case "ai_config_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                name: "OpenAI 主配置",
                provider: "openai-compatible",
                base_url: "https://api.example.com",
                api_key_ref: "local-vault://ai-config/openai-1",
                masked_api_key: "sk-...abcd",
                has_api_key: true,
                model_name: "gpt-4.1-mini",
                temperature: 0.2,
                max_tokens: 4096,
                timeout_seconds: 120,
                stream_enabled: true,
                is_default: true,
              },
            ],
          },
        };
      case "prompt_templates_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{ id: 7, name: "综合分析模板", type: "stock_full", description: "", content: "{{stock_name}}", variables: ["stock_name"], is_builtin: false }],
          },
        };
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [{ key: "notifications.task_terminal", value: "false" }] } };
      case "analysis_task_create":
        return { code: 0, message: "ok", data: { task_id: "analysis-muted", status: "PENDING" } };
      case "analysis_task_subscribe":
        return { emitted: 2, last_event_id: 2 };
      case "task_events":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 1, event: "TASK_STARTED", data: { status: "RUNNING" } },
              { id: 2, event: "TASK_SUCCESS", data: { report_id: 9 } },
            ],
          },
        };
      case "report_get":
        return {
          code: 0,
          message: "ok",
          data: { id: 9, task_id: "analysis-muted", symbol: "600000.SH", title: "浦发银行分析", content_markdown: "## 结论\n保持观察", risk_summary: "" },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "AI 分析" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("股票代码"), { target: { value: "600000.SH" } });
  fireEvent.click(screen.getByRole("button", { name: "开始分析" }));

  await waitFor(() => {
    expect(screen.getByText("SUCCESS")).toBeInTheDocument();
  });
  expect(notificationSendMock).not.toHaveBeenCalled();
});

test("报告历史页面读取报告列表、详情并支持删除", async () => {
  window.location.hash = "#/reports";
  const calls: Array<{ command: string; payload?: any }> = [];
  const originalBlob = globalThis.Blob;
  const exportedBlobs: Array<{ parts: string[]; type: string }> = [];
  const writeText = vi.fn<(text: string) => Promise<void>>(() => Promise.resolve());
  const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
  class TestBlob {
    parts: string[];
    type: string;

    constructor(parts: BlobPart[], options?: BlobPropertyBag) {
      this.parts = parts.map((part) => String(part));
      this.type = options?.type ?? "";
    }
  }
  Object.defineProperty(globalThis, "Blob", { value: TestBlob, configurable: true });
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  Object.defineProperty(URL, "createObjectURL", {
    value: vi.fn((blob: { parts: string[]; type: string }) => {
      exportedBlobs.push(blob);
      return "blob:report-markdown";
    }),
    configurable: true,
  });
  Object.defineProperty(URL, "revokeObjectURL", { value: vi.fn(), configurable: true });
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "report_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{ id: 9, task_id: "analysis-1", symbol: "600000.SH", title: "浦发银行分析", risk_summary: "波动风险", created_at: "2026-06-19T10:00:00Z" }],
          },
        };
      case "report_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 9,
            task_id: "analysis-1",
            symbol: "600000.SH",
            title: "浦发银行分析",
            content_markdown: "## 结论\n保持观察",
            risk_summary: "波动风险",
            input_snapshot: "{\"user_position\":\"满仓\"}",
          },
        };
      case "report_delete":
        return { code: 0, message: "ok", data: { id: 9 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "报告历史" })).toBeInTheDocument();
  });
  expect(screen.getByText("浦发银行分析")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "查看报告 9" }));

  await waitFor(() => {
    expect(screen.getByText("## 结论")).toBeInTheDocument();
  });
  expect(screen.getByText("保持观察")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "复制 Markdown 9" }));

  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("## 结论"));
  });
  expect(writeText.mock.calls[0][0]).not.toContain("input_snapshot");
  expect(writeText.mock.calls[0][0]).not.toContain("满仓");

  fireEvent.click(screen.getByRole("button", { name: "导出 Markdown 9" }));
  await waitFor(() => {
    expect(clickSpy).toHaveBeenCalled();
  });
  const exportedText = exportedBlobs[0].parts.join("");
  expect(exportedText).toContain("## 结论");
  expect(exportedText).not.toContain("input_snapshot");
  expect(exportedText).not.toContain("满仓");
  Object.defineProperty(globalThis, "Blob", { value: originalBlob, configurable: true });
  clickSpy.mockRestore();

  fireEvent.click(screen.getByRole("button", { name: "删除报告 9" }));
  await waitFor(() => {
    expect(screen.queryByText("浦发银行分析")).not.toBeInTheDocument();
  });
  expect(calls.some((call) => call.command === "report_delete" && call.payload?.id === 9)).toBe(true);
});

test("任务历史页面读取任务详情、事件回放并订阅新增事件", async () => {
  window.location.hash = "#/tasks";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "task_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{ id: "analysis-1", type: "ANALYSIS", status: "RUNNING", title: "浦发银行分析", progress: 50 }],
          },
        };
      case "task_get":
        return { code: 0, message: "ok", data: { id: "analysis-1", type: "ANALYSIS", status: "RUNNING", title: "浦发银行分析", progress: 50 } };
      case "task_events":
        if ((payload as any)?.afterEventId === 1) {
          return {
            code: 0,
            message: "ok",
            data: {
              items: [
                { id: 2, event: "TASK_SUCCESS", data: { status: "SUCCESS", report_id: 9 }, created_at: "2026-06-19T10:02:00Z" },
              ],
            },
          };
        }
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 1, event: "TASK_STARTED", data: { status: "RUNNING" }, created_at: "2026-06-19T10:00:00Z" },
            ],
          },
        };
      case "analysis_task_subscribe":
        return { emitted: 1, last_event_id: 2 };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务历史" })).toBeInTheDocument();
  });
  expect(screen.getByText("RUNNING")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "查看任务 analysis-1" }));

  await waitFor(() => {
    expect(screen.getByText("TASK_SUCCESS")).toBeInTheDocument();
  });
  expect(calls).toContainEqual({ command: "analysis_task_subscribe", payload: { taskId: "analysis-1", afterEventId: 1 } });
  expect(calls).toContainEqual({ command: "task_events", payload: { taskId: "analysis-1", afterEventId: 1 } });
});

test("任务历史页面展示失败任务错误原因", async () => {
  window.location.hash = "#/tasks";
  mockIPC((command) => {
    switch (command) {
      case "task_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{ id: "analysis-failed", type: "ANALYSIS", status: "FAILED", title: "浦发银行分析", progress: 50, error_message: "provider timeout" }],
          },
        };
      case "task_get":
        return { code: 0, message: "ok", data: { id: "analysis-failed", type: "ANALYSIS", status: "FAILED", title: "浦发银行分析", progress: 50, error_message: "provider timeout" } };
      case "task_events":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 1, event: "TASK_STARTED", data: { status: "RUNNING" }, created_at: "2026-06-19T10:00:00Z" },
              { id: 2, event: "TASK_FAILED", data: { message: "provider timeout" }, created_at: "2026-06-19T10:01:00Z" },
            ],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务历史" })).toBeInTheDocument();
  });
  expect(screen.getByText("FAILED")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "查看任务 analysis-failed" }));

  await waitFor(() => {
    expect(screen.getByText("TASK_FAILED")).toBeInTheDocument();
  });
  expect(screen.getAllByText("provider timeout").length).toBeGreaterThan(0);
});

test("设置中心读取真实设置并拒绝带凭据的代理 URL", async () => {
  window.location.hash = "#/settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { key: "proxy_url", value: "http://127.0.0.1:7890" },
              { key: "proxy_credential_ref", value: "local-vault://proxy/default" },
              { key: "window.close_to_tray", value: "true" },
              { key: "notifications.task_terminal", value: "false" },
              { key: "update.manifest_url", value: "https://updates.example.com/manifest.json" },
              { key: "update.allowed_hosts", value: "updates.example.com,cdn.example.com" },
            ],
          },
        };
      case "workspace_get":
        return { code: 0, message: "ok", data: { path: "/Users/demo/InvestCompass" } };
      case "cache_stats":
        return { code: 0, message: "ok", data: { total_bytes: 30, items: [{ target: "quote", bytes: 30, label: "行情缓存", cleanable: true }] } };
      case "providers_status":
        return { code: 0, message: "ok", data: [{ name: "Market", source: "unconfigured", available: false, last_error: "未配置" }] };
      case "autostart_get":
        return { enabled: true };
      case "check_update":
        return { code: 0, message: "ok", data: { current_version: "0.1.0", latest_version: "0.1.1", has_update: true, release_notes: "修复问题" } };
      case "cache_clean":
        return { code: 0, message: "ok", data: { cleaned_targets: ["quote"] } };
      case "autostart_set":
        return { enabled: Boolean((payload as { enabled?: boolean }).enabled) };
      case "settings_set":
      case "workspace_set":
        return { code: 0, message: "ok", data: { saved_keys: ["proxy_url"] } };
      case "export_logs":
        return { file_path: "/Users/demo/InvestCompass/invest-compass.log", file_name: "invest-compass.log" };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "设置中心" })).toBeInTheDocument();
  });
  expect(screen.getByDisplayValue("/Users/demo/InvestCompass")).toBeInTheDocument();
  expect(screen.getByDisplayValue("https://updates.example.com/manifest.json")).toBeInTheDocument();
  expect(screen.getByDisplayValue("updates.example.com,cdn.example.com")).toBeInTheDocument();
  expect(screen.getByText("行情缓存")).toBeInTheDocument();
  expect(screen.getByText("unconfigured")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "进入任务调度" })).toHaveAttribute("href", "#/scheduler");
  expect(screen.getByLabelText("关闭到托盘")).toBeChecked();
  expect(screen.getByLabelText("开机自动启动")).toBeChecked();
  expect(screen.getByLabelText("任务成功/失败通知")).not.toBeChecked();

  fireEvent.change(screen.getByLabelText("代理 URL"), { target: { value: "http://user:pass@127.0.0.1:7890" } });
  fireEvent.click(screen.getByRole("button", { name: "保存设置" }));
  expect(await screen.findByText("代理 URL 不能包含用户名或密码")).toBeInTheDocument();
  expect(calls.some((call) => call.command === "settings_set")).toBe(false);

  fireEvent.change(screen.getByLabelText("代理 URL"), { target: { value: "http://127.0.0.1:7890" } });
  fireEvent.change(screen.getByLabelText("更新 Manifest URL"), {
    target: { value: " https://updates.invest-compass.example/manifest.json " },
  });
  fireEvent.change(screen.getByLabelText("更新允许域名"), {
    target: { value: " updates.invest-compass.example, cdn.invest-compass.example " },
  });
  fireEvent.click(screen.getByLabelText("关闭到托盘"));
  fireEvent.click(screen.getByLabelText("开机自动启动"));
  fireEvent.click(screen.getByLabelText("任务成功/失败通知"));
  fireEvent.click(screen.getByRole("button", { name: "保存设置" }));
  await waitFor(() => {
    expect(
      calls.some(
        (call) =>
          call.command === "settings_set" &&
          call.payload?.payload?.items?.some(
            (item: { key: string; value: string }) =>
              item.key === "update.manifest_url" && item.value === "https://updates.invest-compass.example/manifest.json",
          ) &&
          call.payload?.payload?.items?.some(
            (item: { key: string; value: string }) => item.key === "update.allowed_hosts" && item.value === "updates.invest-compass.example, cdn.invest-compass.example",
          ) &&
          call.payload?.payload?.items?.some(
            (item: { key: string; value: string }) => item.key === "window.close_to_tray" && item.value === "false",
          ) &&
          call.payload?.payload?.items?.some(
            (item: { key: string; value: string }) => item.key === "notifications.task_terminal" && item.value === "true",
          ),
      ),
    ).toBe(true);
  });
  expect(calls.some((call) => call.command === "autostart_set" && call.payload?.enabled === false)).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "清理 quote" }));
  await waitFor(() => {
    expect(calls.some((call) => call.command === "cache_clean" && call.payload?.payload?.targets?.[0] === "quote")).toBe(true);
  });
  fireEvent.click(screen.getByRole("button", { name: "检查更新" }));
  expect(await screen.findByText("0.1.1")).toBeInTheDocument();

  dialogOpenMock.mockResolvedValue("/Users/demo/InvestCompass/logs");
  fireEvent.click(screen.getByRole("button", { name: "选择目录" }));
  await waitFor(() => {
    expect(dialogOpenMock).toHaveBeenCalledWith({ directory: true, multiple: false });
  });
  expect(screen.getByLabelText("日志导出目录")).toHaveValue("/Users/demo/InvestCompass/logs");

  fireEvent.click(screen.getByRole("button", { name: "导出日志" }));
  expect(await screen.findByText("/Users/demo/InvestCompass/invest-compass.log")).toBeInTheDocument();
  expect(calls.some((call) => call.command === "export_logs" && call.payload?.targetDir === "/Users/demo/InvestCompass/logs")).toBe(true);

  fireEvent.change(screen.getByLabelText("日志导出目录"), { target: { value: "" } });
  fireEvent.click(screen.getByRole("button", { name: "导出日志" }));
  expect(await screen.findByText("日志导出目录不能为空")).toBeInTheDocument();

  fireEvent.change(screen.getByLabelText("日志导出目录"), { target: { value: "  /Users/demo/InvestCompass  " } });
  fireEvent.click(screen.getByRole("button", { name: "导出日志" }));
  expect(await screen.findByText("/Users/demo/InvestCompass/invest-compass.log")).toBeInTheDocument();
  expect(calls.some((call) => call.command === "export_logs" && call.payload?.targetDir === "/Users/demo/InvestCompass")).toBe(true);
});

test("任务调度页面读取真实调度接口并支持立即执行", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-06-19T09:00:00+08:00"));
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "scheduler_status":
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 0, running_runs: 0, failed_runs: 0 },
        };
      case "scheduler_job_types":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新" }] },
        };
      case "scheduler_jobs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "A 股开盘行情刷新",
                cron_type: "cn_a_share_quote_refresh",
                cron_expr: "30 9 * * 1-5",
                enabled: true,
                market: "CN",
                last_status: "failed",
                last_error: "market_provider_unconfigured",
              },
            ],
          },
        };
      case "scheduler_runs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 11,
                run_key: "run-11",
                trigger_type: "missed_today",
                status: "failed",
                target_date: "2026-06-19",
                started_at: "2026-06-19T09:30:00Z",
                finished_at: "2026-06-19T09:31:00Z",
                fetched_count: 1,
                written_count: 0,
              },
            ],
          },
        };
      case "scheduler_runs_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 11,
            run_key: "run-11",
            trigger_type: "missed_today",
            status: "failed",
            target_date: "2026-06-19",
            started_at: "2026-06-19T09:30:00Z",
            finished_at: "2026-06-19T09:31:00Z",
            source: "startup_restore",
            scope_key: "CN:SH:600519",
            cron_type: "cn_a_share_quote_refresh",
            data_type: "quote",
            fetched_count: 1,
            written_count: 0,
            error_message: "market_provider_unconfigured",
          },
        };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: [
            { name: "market-provider", source: "sina-tencent-market", available: true },
            { name: "news-provider", source: "sina-live", available: true },
          ],
        };
      case "scheduler_jobs_run_now":
        return {
          code: 0,
          message: "ok",
          data: { id: 12, run_key: "run-12", trigger_type: "user_request", status: "queued" },
        };
      case "scheduler_jobs_set_enabled":
        return { code: 0, message: "ok", data: { id: 7, enabled: false } };
      case "scheduler_jobs_backfill":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ id: 13, run_key: "run-13", trigger_type: "catchup_gap", status: "queued" }] },
        };
      case "scheduler_jobs_save":
        return {
          code: 0,
          message: "ok",
          data: { id: 14, name: "新建调度任务", cron_type: "cn_a_share_quote_refresh", enabled: true },
        };
      case "scheduler_refresh_symbol":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ id: 15, run_key: "run-15", trigger_type: "user_request", status: "queued" }] },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  fireEvent.click(screen.getByRole("link", { name: "任务调度" }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务调度" })).toBeInTheDocument();
  });
  expect(screen.getAllByText("A 股开盘行情刷新").length).toBeGreaterThan(0);
  expect(screen.getByText("2026-06-19 09:30")).toBeInTheDocument();
  expect(screen.getAllByText("failed").length).toBeGreaterThan(0);
  expect(screen.getByText("market_provider_unconfigured")).toBeInTheDocument();
  expect(screen.getAllByText("missed_today").length).toBeGreaterThan(0);
  expect(screen.getByText("2026-06-19T09:30:00Z")).toBeInTheDocument();
  expect(screen.getByText("2026-06-19T09:31:00Z")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "查看详情" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_runs_get")).toBe(true);
  });
  expect(screen.getByText("执行详情")).toBeInTheDocument();
  expect(screen.getByText("startup_restore")).toBeInTheDocument();
  expect(screen.getByText("CN:SH:600519")).toBeInTheDocument();
  expect(screen.getAllByText("2026-06-19T09:30:00Z").length).toBeGreaterThan(0);
  expect(screen.getAllByText("2026-06-19T09:31:00Z").length).toBeGreaterThan(0);

  fireEvent.click(screen.getByRole("button", { name: "立即执行" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_jobs_run_now")).toBe(true);
  });

  fireEvent.click(screen.getByRole("button", { name: "停用任务" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_jobs_set_enabled")).toBe(true);
  });

  fireEvent.change(screen.getByLabelText("名称"), { target: { value: "新建调度任务" } });
  fireEvent.click(screen.getByRole("button", { name: "创建任务" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_jobs_save")).toBe(true);
  });

  fireEvent.change(screen.getByLabelText("任务"), { target: { value: "7" } });
  fireEvent.change(screen.getAllByLabelText("Symbol，可选")[1], { target: { value: "600000.SH 000001.SZ" } });
  fireEvent.click(screen.getByRole("button", { name: "生成补偿" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_jobs_backfill")).toBe(true);
  });

  fireEvent.change(screen.getByLabelText("股票代码"), { target: { value: "600000.SH" } });
  fireEvent.click(screen.getByRole("button", { name: "刷新单股" }));

  await waitFor(() => {
    expect(calls.some((call) => call.command === "scheduler_refresh_symbol")).toBe(true);
  });
});

test("任务调度页面支持按执行状态和触发类型过滤运行记录", async () => {
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "scheduler_status":
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 1, running_runs: 0, failed_runs: 1 },
        };
      case "scheduler_job_types":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新" }] },
        };
      case "scheduler_jobs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "A 股开盘行情刷新",
                cron_type: "cn_a_share_quote_refresh",
                cron_expr: "30 9 * * 1-5",
                enabled: true,
                market: "CN",
              },
            ],
          },
        };
      case "scheduler_runs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 21, run_key: "run-failed", trigger_type: "missed_today", status: "failed", target_date: "2026-06-19" },
              { id: 22, run_key: "run-queued", trigger_type: "user_request", status: "queued", target_date: "2026-06-19" },
            ],
          },
        };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: [
            { name: "market-provider", source: "sina-tencent-market", available: true },
            { name: "news-provider", source: "sina-live", available: true },
          ],
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  fireEvent.click(screen.getByRole("link", { name: "任务调度" }));

  await waitFor(() => {
    expect(screen.getByText("run-failed")).toBeInTheDocument();
  });
  expect(screen.getByText("run-queued")).toBeInTheDocument();

  fireEvent.change(screen.getByLabelText("执行状态"), { target: { value: "failed" } });
  expect(screen.getByText("run-failed")).toBeInTheDocument();
  expect(screen.queryByText("run-queued")).not.toBeInTheDocument();

  fireEvent.change(screen.getByLabelText("触发类型"), { target: { value: "user_request" } });
  expect(screen.getByText("暂无执行记录")).toBeInTheDocument();
});

test("任务调度页面补偿日期超范围时不保留旧成功提示", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "scheduler_status":
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 0, running_runs: 0, failed_runs: 0 },
        };
      case "scheduler_job_types":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新" }] },
        };
      case "scheduler_jobs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "A 股开盘行情刷新",
                cron_type: "cn_a_share_quote_refresh",
                cron_expr: "30 9 * * 1-5",
                enabled: true,
                market: "CN",
              },
            ],
          },
        };
      case "scheduler_runs_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: [
            { name: "market-provider", source: "sina-tencent-market", available: true },
            { name: "news-provider", source: "sina-live", available: true },
          ],
        };
      case "scheduler_jobs_backfill":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ id: 31, run_key: "run-31", trigger_type: "catchup_gap", status: "queued" }] },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  fireEvent.click(screen.getByRole("link", { name: "任务调度" }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务调度" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("任务"), { target: { value: "7" } });
  fireEvent.change(screen.getByLabelText("开始日期"), { target: { value: "2026-06-01" } });
  fireEvent.change(screen.getByLabelText("结束日期"), { target: { value: "2026-06-01" } });
  fireEvent.click(screen.getByRole("button", { name: "生成补偿" }));

  await waitFor(() => {
    expect(screen.getByText("已生成 1 条补偿执行记录")).toBeInTheDocument();
  });

  fireEvent.change(screen.getByLabelText("结束日期"), { target: { value: "2026-07-01" } });
  fireEvent.click(screen.getByRole("button", { name: "生成补偿" }));

  expect(screen.getByText("单次补偿最多覆盖 30 个自然日")).toBeInTheDocument();
  expect(screen.queryByText("已生成 1 条补偿执行记录")).not.toBeInTheDocument();
  expect(calls.filter((call) => call.command === "scheduler_jobs_backfill")).toHaveLength(1);
});

test("任务调度页面读取执行详情失败时清理旧详情并展示错误", async () => {
  let detailCalls = 0;
  mockIPC((command, payload) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "scheduler_status":
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 0, running_runs: 0, failed_runs: 1 },
        };
      case "scheduler_job_types":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新" }] },
        };
      case "scheduler_jobs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "A 股开盘行情刷新",
                cron_type: "cn_a_share_quote_refresh",
                cron_expr: "30 9 * * 1-5",
                enabled: true,
                market: "CN",
              },
            ],
          },
        };
      case "scheduler_runs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 41, run_key: "run-ok", trigger_type: "missed_today", status: "failed", target_date: "2026-06-19" },
              { id: 42, run_key: "run-error", trigger_type: "user_request", status: "failed", target_date: "2026-06-19" },
            ],
          },
        };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: [
            { name: "market-provider", source: "sina-tencent-market", available: true },
            { name: "news-provider", source: "sina-live", available: true },
          ],
        };
      case "scheduler_runs_get":
        detailCalls += 1;
        if ((payload as { id?: number }).id === 42) {
          return { code: "scheduler_run_missing", message: "执行记录不存在", data: null };
        }
        return {
          code: 0,
          message: "ok",
          data: {
            id: 41,
            run_key: "run-ok",
            trigger_type: "missed_today",
            status: "failed",
            source: "startup_restore",
            scope_key: "CN:SH:600519",
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  fireEvent.click(screen.getByRole("link", { name: "任务调度" }));

  await waitFor(() => {
    expect(screen.getByText("run-ok")).toBeInTheDocument();
  });
  fireEvent.click(screen.getAllByRole("button", { name: "查看详情" })[0]);

  await waitFor(() => {
    expect(screen.getByText("startup_restore")).toBeInTheDocument();
  });

  fireEvent.click(screen.getAllByRole("button", { name: "查看详情" })[1]);

  await waitFor(() => {
    expect(screen.getByText("执行记录不存在")).toBeInTheDocument();
  });
  expect(screen.queryByText("startup_restore")).not.toBeInTheDocument();
  expect(detailCalls).toBe(2);
});

test("任务调度页面展示 Provider 不可用状态并禁用立即执行", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "scheduler_status":
        return {
          code: 0,
          message: "ok",
          data: { jobs_total: 1, jobs_enabled: 1, queued_runs: 0, running_runs: 0, failed_runs: 0 },
        };
      case "scheduler_job_types":
        return {
          code: 0,
          message: "ok",
          data: { items: [{ cron_type: "cn_a_share_quote_refresh", label: "A 股行情刷新" }] },
        };
      case "scheduler_jobs_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                name: "A 股开盘行情刷新",
                cron_type: "cn_a_share_quote_refresh",
                cron_expr: "30 9 * * 1-5",
                enabled: true,
                market: "CN",
              },
            ],
          },
        };
      case "scheduler_runs_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: [
            { name: "market-provider", source: "unconfigured", available: false, last_error: "market_provider_unconfigured" },
            { name: "news-provider", source: "sina-live", available: true },
          ],
        };
      case "scheduler_jobs_run_now":
        return {
          code: 0,
          message: "ok",
          data: { id: 12, run_key: "run-12", trigger_type: "user_request", status: "queued" },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  fireEvent.click(screen.getByRole("link", { name: "任务调度" }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务调度" })).toBeInTheDocument();
  });
  expect(calls.some((call) => call.command === "providers_status")).toBe(true);
  expect(screen.getByText("market_provider_unconfigured")).toBeInTheDocument();

  const runNowButton = screen.getByRole("button", { name: "立即执行" });
  expect(runNowButton).toBeDisabled();
  fireEvent.click(runNowButton);
  expect(calls.some((call) => call.command === "scheduler_jobs_run_now")).toBe(false);
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
