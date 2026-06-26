/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import "@testing-library/jest-dom/vitest";
import "../test/setupDom";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { App as AntApp } from "antd";
import { Suspense } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";
import { App, AppErrorBoundary, StockDetailRoute } from "./App";
import * as AppModule from "./App";
import { FullscreenKlinePage } from "../pages/chart/FullscreenKlinePage";
import { appDatePickerLocale } from "../lib/antdLocale";
import { useDashboardStore } from "../stores/dashboardStore";

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

const dialogOpenMock = vi.hoisted(() => vi.fn());
const notificationIsPermissionGrantedMock = vi.hoisted(() => vi.fn());
const notificationRequestPermissionMock = vi.hoisted(() => vi.fn());
const notificationSendMock = vi.hoisted(() => vi.fn());
const eventListenMock = vi.hoisted(() => vi.fn((_event: string, _handler: (event: { payload: unknown }) => void) => Promise.resolve(vi.fn())));

vi.mock("@tauri-apps/plugin-dialog", () => ({
  open: dialogOpenMock,
}));

vi.mock("@tauri-apps/plugin-notification", () => ({
  isPermissionGranted: notificationIsPermissionGrantedMock,
  requestPermission: notificationRequestPermissionMock,
  sendNotification: notificationSendMock,
}));

vi.mock("@tauri-apps/api/event", () => ({
  listen: eventListenMock,
}));

afterEach(() => {
  clearMocks();
  useDashboardStore.setState({ state: null, loading: false, error: null, lastLoadedAt: null });
  dialogOpenMock.mockReset();
  notificationIsPermissionGrantedMock.mockReset();
  notificationRequestPermissionMock.mockReset();
  notificationSendMock.mockReset();
  eventListenMock.mockReset();
  AppModule.resetBootReadyForTest();
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

const basicSettingsGetPayload = {
  keys: ["app.theme", "app.language", "market.default", "quote.refresh_interval", "kline.default_period", "kline.default_adjust"],
};

const notificationSettingsGetPayload = {
  keys: [
    "notifications.in_app_enabled",
    "notifications.system_enabled",
    "notifications.task_success",
    "notifications.task_failed",
    "notifications.provider_error",
  ],
};

const dataSourceSettingsGetPayload = {
  keys: [
    "data_source.default_market_source",
    "data_source.quote_refresh_interval",
    "data_source.news_sync_interval",
  ],
};

function withoutGlobalNotificationUnreadCalls<TCall extends { command: string; payload?: unknown }>(calls: TCall[]): TCall[] {
  let skippedAutoRefreshSettings = false;
  return calls.filter((call) => {
    if (["notifications_unread_count", "core_health", "providers_status"].includes(call.command)) {
      return false;
    }
    if (!skippedAutoRefreshSettings && isTopBarAutoRefreshSettingsCall(call)) {
      skippedAutoRefreshSettings = true;
      return false;
    }
    return true;
  });
}

function isTopBarAutoRefreshSettingsCall(call: { command: string; payload?: unknown }) {
  if (call.command !== "settings_get" || typeof call.payload !== "object" || call.payload === null) {
    return false;
  }
  const keys = (call.payload as { keys?: unknown }).keys;
  return (
    Array.isArray(keys) &&
    keys.length === 2 &&
    keys[0] === "quote.refresh_interval" &&
    keys[1] === "data_source.quote_refresh_interval"
  );
}

const defaultAIConfig = {
  id: 1,
  name: "DeepSeek",
  provider: "openai-compatible",
  base_url: "https://api.deepseek.com",
  api_key_ref: "local-vault://ai-config/deepseek-1",
  masked_api_key: "sk-...seek",
  has_api_key: true,
  model_name: "DeepSeek-V3",
  temperature: 0.2,
  max_tokens: 4096,
  timeout_seconds: 120,
  stream_enabled: true,
  is_default: true,
};

const dataSourceCredentialListFixture = {
  providers: [
    { id: "eastmoney", name: "EastMoney", capability: "基础证券列表可访问 / K线受限", status: "limited", authType: "none", iconType: "eastmoney" },
    { id: "sina", name: "新浪财经", capability: "股票搜索 / 实时行情", status: "normal", authType: "none", iconType: "sina" },
    { id: "tencent", name: "腾讯财经", capability: "K线 / 复权K线", status: "normal", authType: "none", iconType: "tencent" },
    { id: "akshare", name: "AkShare", capability: "基础数据", status: "normal", authType: "none", iconType: "akshare" },
    { id: "alpha-vantage", name: "Alpha Vantage", capability: "海外行情", status: "not_configured", authType: "api_key", iconType: "alpha-vantage" },
    { id: "cls", name: "财联社", capability: "快讯 / 日历", status: "normal", authType: "cookie", iconType: "cls" },
    { id: "xueqiu", name: "雪球", capability: "讨论热度", status: "not_configured", authType: "cookie", iconType: "xueqiu" },
  ],
  configs: {
    "eastmoney": {
      providerId: "eastmoney",
      providerName: "EastMoney",
      capability: "基础证券列表可访问 / K线受限",
      authType: "none",
      baseUrl: "https://quote.eastmoney.com",
      credentialStatus: "limited",
      timeoutSeconds: 15,
      rateLimitPerMinute: 30,
      maskedCredential: "无需凭据，K线接口受限",
      note: "",
    },
    sina: {
      providerId: "sina",
      providerName: "新浪财经",
      capability: "股票搜索 / 实时行情",
      authType: "none",
      baseUrl: "https://hq.sinajs.cn",
      credentialStatus: "normal",
      timeoutSeconds: 15,
      rateLimitPerMinute: 60,
      maskedCredential: "无需凭据",
      note: "",
    },
    tencent: {
      providerId: "tencent",
      providerName: "腾讯财经",
      capability: "K线 / 复权K线",
      authType: "none",
      baseUrl: "https://web.ifzq.gtimg.cn",
      credentialStatus: "normal",
      timeoutSeconds: 15,
      rateLimitPerMinute: 60,
      maskedCredential: "无需凭据",
      note: "",
    },
    akshare: {
      providerId: "akshare",
      providerName: "AkShare",
      capability: "基础数据",
      authType: "none",
      baseUrl: "local://akshare",
      credentialStatus: "normal",
      timeoutSeconds: 15,
      rateLimitPerMinute: 30,
      maskedCredential: "",
      note: "",
    },
    "alpha-vantage": {
      providerId: "alpha-vantage",
      providerName: "Alpha Vantage",
      capability: "海外行情",
      authType: "api_key",
      baseUrl: "https://www.alphavantage.co",
      credentialStatus: "not_configured",
      timeoutSeconds: 15,
      rateLimitPerMinute: 30,
      maskedCredential: "",
      note: "",
    },
    cls: {
      providerId: "cls",
      providerName: "财联社",
      capability: "快讯 / 行业事件 / 日历",
      authType: "cookie",
      baseUrl: "https://www.cls.cn",
      credentialStatus: "normal",
      expiresAt: "2025-06-30 23:59",
      timeoutSeconds: 15,
      rateLimitPerMinute: 30,
      maskedCredential: "uid=****; token=****; session=****",
      note: "",
    },
    xueqiu: {
      providerId: "xueqiu",
      providerName: "雪球",
      capability: "讨论热度",
      authType: "cookie",
      baseUrl: "https://xueqiu.com",
      credentialStatus: "not_configured",
      timeoutSeconds: 15,
      rateLimitPerMinute: 30,
      maskedCredential: "",
      note: "",
    },
  },
  selectedProviderId: "cls",
  testTargets: [
    { label: "快讯接口（/api/flash）", value: "flash" },
    { label: "日历接口（/api/calendar）", value: "calendar" },
    { label: "行业事件接口（/api/events）", value: "events" },
  ],
  testResult: {
    status: "success",
    responseTimeMs: 186,
    testedAt: "2025-05-20 15:28:41",
    messages: ["行情接口可访问", "新闻接口已授权"],
  },
  overview: { configuredCount: 3, expiringSoonCount: 1, expiredCount: 1 },
  healthItems: [
    { name: "新浪行情源", status: "normal", rateLimitText: "60 次/分钟" },
    { name: "腾讯K线源", status: "normal", rateLimitText: "60 次/分钟" },
    { name: "新闻源", status: "normal", rateLimitText: "26 次/分钟" },
    { name: "海外源", status: "limited", rateLimitText: "12 次/分钟" },
  ],
  operationLogs: [
    { id: "1", action: "更新财联社 Cookie", status: "success", time: "15:28:41" },
    { id: "2", action: "测试 Alpha Vantage Key", status: "success", time: "15:20:13" },
    { id: "3", action: "清除雪球过期凭据", status: "success", time: "14:55:02" },
  ],
};

test("首版主导航和路由范围只包含 MVP 页面", () => {
  const routeModule = AppModule as typeof AppModule & {
    APP_NAV_ITEMS?: unknown;
    APP_ROUTE_PATHS?: unknown;
  };

  expect(routeModule.APP_NAV_ITEMS).toEqual([
    { path: "/", label: "总览" },
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
    "/analysis/running",
    "/reports",
    "/reports/:reportId",
    "/tasks",
    "/settings",
    "/ai-settings",
    "/chart/kline",
  ]);
});

test("全屏 K 线图路由不渲染普通工作台壳层", async () => {
  window.location.hash = "#/chart/kline";

  render(<App initialBootState="ready" />);

  expect(await screen.findByLabelText("全屏K线趋势图页")).toBeInTheDocument();
  expect(screen.getByText("未选择股票")).toBeInTheDocument();
  expect(screen.queryByRole("navigation", { name: "主导航" })).not.toBeInTheDocument();
  expect(screen.queryByPlaceholderText("搜索股票名称 / 代码 / 拼音")).not.toBeInTheDocument();
});

test("全屏 K 线图周线使用后端原生周 K 数据", async () => {
  window.location.hash = "#/chart/kline?symbol=600000.SH&period=week&adjust=qfq";
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.88,
            change_amount: 0.18,
            change_percent: 2.34,
            open: 7.65,
            high: 7.92,
            low: 7.58,
            pre_close: 7.7,
            volume: 456200,
            amount: 358900000,
            turnover_rate: 0.63,
            quote_time: "2026-06-25T10:42:18+08:00",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: { symbol: "600000.SH", name: "浦发银行", code: "600000", exchange: "SH", market: "CN", industry: "银行" },
        };
      case "market_kline":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-12", open: 7.4, high: 7.62, low: 7.35, close: 7.58, volume: 360000 },
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-13", open: 7.58, high: 7.81, low: 7.52, close: 7.7, volume: 420000 },
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-16", open: 7.65, high: 7.92, low: 7.58, close: 7.88, volume: 456200 },
            ],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("浦发银行")).toBeInTheDocument();
  expect(screen.getByText("600000")).toBeInTheDocument();
  expect(screen.getAllByText("7.88").length).toBeGreaterThan(0);
  expect(calls).toContainEqual({ command: "market_quote", payload: { symbol: "600000.SH" } });
  expect(calls).toContainEqual({ command: "stock_profile", payload: { symbol: "600000.SH" } });
  expect(calls).toContainEqual({ command: "market_kline", payload: { symbol: "600000.SH", period: "week", adjust: "qfq", limit: 120 } });
  expect(calls).toContainEqual({
    command: "market_indicators",
    payload: { symbol: "600000.SH", period: "week", adjust: "qfq", limit: 120, indicators: ["ma", "rsi", "macd", "kdj", "boll"] },
  });
});

test("全屏 K 线图指数资料缺失时仍展示指数行情和 K 线", async () => {
  window.location.hash = "#/chart/kline?symbol=000001.SH&period=day&adjust=qfq";
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "000001.SH",
            price: 4102.03,
            change_amount: -4.92,
            change_percent: -0.12,
            open: 4116.58,
            high: 4122.4,
            low: 4090.12,
            pre_close: 4106.95,
            volume: 98000000,
            amount: 452300000000,
            quote_time: "2026-06-25T15:00:03+08:00",
          },
        };
      case "stock_profile":
        throw new Error("sidecar http error: HTTP status client error (404 Not Found) for url (http://127.0.0.1:54045/api/stocks/profile)");
      case "market_kline":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { symbol: "000001.SH", period: "day", adjust: "qfq", trade_date: "2026-06-24", open: 4100, high: 4120, low: 4090, close: 4102.03, volume: 98000000 },
              { symbol: "000001.SH", period: "day", adjust: "qfq", trade_date: "2026-06-25", open: 4116.58, high: 4122.4, low: 4090.12, close: 4102.03, volume: 98000000 },
            ],
          },
        };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", period: "day", adjust: "qfq", indicators: {} } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("000001.SH")).toBeInTheDocument();
  expect(screen.getByText("000001")).toBeInTheDocument();
  expect(screen.getAllByText("4102.03").length).toBeGreaterThan(0);
  expect(screen.queryByText("全屏行情数据读取失败")).not.toBeInTheDocument();
  expect(calls).toContainEqual({ command: "stock_profile", payload: { symbol: "000001.SH" } });
});

test("全屏 K 线图后复权指标读取失败时保留真实 K 线展示", async () => {
  window.location.hash = "#/chart/kline?symbol=600000.SH&period=day&adjust=qfq";
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.88,
            change_amount: 0.18,
            change_percent: 2.34,
            open: 7.65,
            high: 7.92,
            low: 7.58,
            pre_close: 7.7,
            volume: 456200,
            amount: 358900000,
            turnover_rate: 0.63,
            quote_time: "2026-06-25T10:42:18+08:00",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: { symbol: "600000.SH", name: "浦发银行", code: "600000", exchange: "SH", market: "CN", industry: "银行" },
        };
      case "market_kline": {
        const request = payload as { adjust: string; period: string };
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { symbol: "600000.SH", period: request.period, adjust: request.adjust, trade_date: "2026-06-23", open: 7.62, high: 7.82, low: 7.58, close: 7.76, volume: 320000 },
              { symbol: "600000.SH", period: request.period, adjust: request.adjust, trade_date: "2026-06-24", open: 7.76, high: 7.9, low: 7.7, close: 7.8, volume: 380000 },
              { symbol: "600000.SH", period: request.period, adjust: request.adjust, trade_date: "2026-06-25", open: 7.8, high: 7.92, low: 7.74, close: 7.88, volume: 456200 },
            ],
          },
        };
      }
      case "market_indicators":
        throw new Error("sidecar http error: HTTP status server error (502 Bad Gateway) for url (http://127.0.0.1:49978/api/market/indicators)");
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("浦发银行")).toBeInTheDocument();
  expect(screen.queryByText("全屏行情数据读取失败")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "后复权" }));

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "market_kline", payload: { symbol: "600000.SH", period: "day", adjust: "hfq", limit: 120 } });
  });
  expect(calls).toContainEqual({
    command: "market_indicators",
    payload: { symbol: "600000.SH", period: "day", adjust: "hfq", limit: 120, indicators: ["ma", "rsi", "macd", "kdj", "boll"] },
  });
  expect(screen.getAllByText("7.88").length).toBeGreaterThan(0);
  expect(screen.queryByText("全屏行情数据读取失败")).not.toBeInTheDocument();
});

test("全屏 K 线图后复权 K 线读取失败时保留上一份真实图表状态并提示错误", async () => {
  window.location.hash = "#/chart/kline?symbol=600000.SH&period=day&adjust=qfq";
  const calls: Array<{ command: string; payload?: unknown }> = [];
  let klineCallCount = 0;
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.88,
            change_amount: 0.18,
            change_percent: 2.34,
            open: 7.65,
            high: 7.92,
            low: 7.58,
            pre_close: 7.7,
            volume: 456200,
            amount: 358900000,
            turnover_rate: 0.63,
            quote_time: "2026-06-25T10:42:18+08:00",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: { symbol: "600000.SH", name: "浦发银行", code: "600000", exchange: "SH", market: "CN", industry: "银行" },
        };
      case "market_kline": {
        klineCallCount += 1;
        if (klineCallCount > 1) {
          throw new Error("sidecar http error: HTTP status server error (502 Bad Gateway) for url (http://127.0.0.1:54304/api/market/kline)");
        }
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-24", open: 7.76, high: 7.9, low: 7.7, close: 7.8, volume: 380000 },
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-25", open: 7.8, high: 7.92, low: 7.74, close: 7.88, volume: 456200 },
            ],
          },
        };
      }
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: {} } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("浦发银行")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "后复权" }));

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "market_kline", payload: { symbol: "600000.SH", period: "day", adjust: "hfq", limit: 120 } });
  });
  expect(await screen.findByText("全屏行情数据读取失败")).toBeInTheDocument();
  expect(screen.getByText(/HTTP status server error/)).toBeInTheDocument();
  expect(screen.getByText("浦发银行")).toBeInTheDocument();
  expect(screen.getAllByText("7.88").length).toBeGreaterThan(0);
});

test("全屏 K 线图月线读取失败时保留上一份真实图表状态并提示错误", async () => {
  window.location.hash = "#/chart/kline?symbol=600000.SH&period=day&adjust=qfq";
  const calls: Array<{ command: string; payload?: unknown }> = [];
  let klineCallCount = 0;
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.88,
            change_amount: 0.18,
            change_percent: 2.34,
            open: 7.65,
            high: 7.92,
            low: 7.58,
            pre_close: 7.7,
            volume: 456200,
            amount: 358900000,
            turnover_rate: 0.63,
            quote_time: "2026-06-25T10:42:18+08:00",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: { symbol: "600000.SH", name: "浦发银行", code: "600000", exchange: "SH", market: "CN", industry: "银行" },
        };
      case "market_kline": {
        klineCallCount += 1;
        if (klineCallCount > 1) {
          throw new Error("sidecar http error: HTTP status server error (502 Bad Gateway) for url (http://127.0.0.1:51383/api/market/kline)");
        }
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-23", open: 7.62, high: 7.82, low: 7.58, close: 7.76, volume: 320000 },
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-24", open: 7.76, high: 7.9, low: 7.7, close: 7.8, volume: 380000 },
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-25", open: 7.8, high: 7.92, low: 7.74, close: 7.88, volume: 456200 },
            ],
          },
        };
      }
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: {} } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("浦发银行")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "月K" }));

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "market_kline", payload: { symbol: "600000.SH", period: "month", adjust: "qfq", limit: 120 } });
  });
  expect(await screen.findByText("全屏行情数据读取失败")).toBeInTheDocument();
  expect(screen.getByText(/HTTP status server error/)).toBeInTheDocument();
  expect(screen.getByText("浦发银行")).toBeInTheDocument();
  expect(screen.getAllByText("7.88").length).toBeGreaterThan(0);
});

test("全屏 K 线图周 K 切到月 K 时重新读取后端原生周期", async () => {
  window.location.hash = "#/chart/kline?symbol=600000.SH&period=week&adjust=qfq";
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.88,
            change_amount: 0.18,
            change_percent: 2.34,
            open: 7.65,
            high: 7.92,
            low: 7.58,
            pre_close: 7.7,
            volume: 456200,
            amount: 358900000,
            turnover_rate: 0.63,
            quote_time: "2026-06-25T10:42:18+08:00",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: { symbol: "600000.SH", name: "浦发银行", code: "600000", exchange: "SH", market: "CN", industry: "银行" },
        };
      case "market_kline":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-05-29", open: 7.1, high: 7.5, low: 7.0, close: 7.4, volume: 280000 },
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-25", open: 7.6, high: 7.92, low: 7.58, close: 7.88, volume: 456200 },
            ],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("浦发银行")).toBeInTheDocument();
  await waitFor(() => {
    expect(calls.filter((call) => call.command === "market_kline")).toHaveLength(1);
  });
  fireEvent.click(screen.getByRole("button", { name: "月K" }));

  await waitFor(() => {
    expect(calls.filter((call) => call.command === "market_kline")).toHaveLength(2);
  });
  expect(calls.filter((call) => call.command === "market_kline")).toEqual([
    { command: "market_kline", payload: { symbol: "600000.SH", period: "week", adjust: "qfq", limit: 120 } },
    { command: "market_kline", payload: { symbol: "600000.SH", period: "month", adjust: "qfq", limit: 120 } },
  ]);
});

test("全屏 K 线图顶部搜索选择股票后跳转到对应 K 线详情", async () => {
  window.location.hash = "#/chart/kline?symbol=600000.SH&period=day&adjust=qfq";
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "market_quote": {
        const symbol = (payload as { symbol: string }).symbol;
        return {
          code: 0,
          message: "ok",
          data: {
            symbol,
            price: symbol === "CN:SZ:301217" ? 183.27 : 7.88,
            change_amount: symbol === "CN:SZ:301217" ? -3.42 : 0.18,
            change_percent: symbol === "CN:SZ:301217" ? -1.83 : 2.34,
            open: 7.65,
            high: 7.92,
            low: 7.58,
            pre_close: 7.7,
            volume: 456200,
            amount: 358900000,
            turnover_rate: 0.63,
            quote_time: "2026-06-25T10:42:18+08:00",
          },
        };
      }
      case "stock_profile": {
        const symbol = (payload as { symbol: string }).symbol;
        return {
          code: 0,
          message: "ok",
          data:
            symbol === "CN:SZ:301217"
              ? { symbol, name: "铜冠铜箔", code: "301217", exchange: "SZ", market: "CN", industry: "元器件" }
              : { symbol, name: "浦发银行", code: "600000", exchange: "SH", market: "CN", industry: "银行" },
        };
      }
      case "market_kline":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { symbol: (payload as { symbol: string }).symbol, period: "day", adjust: "qfq", trade_date: "2026-06-24", open: 7.76, high: 7.9, low: 7.7, close: 7.8, volume: 380000 },
              { symbol: (payload as { symbol: string }).symbol, period: "day", adjust: "qfq", trade_date: "2026-06-25", open: 7.8, high: 7.92, low: 7.74, close: 7.88, volume: 456200 },
            ],
          },
        };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: {} } };
      case "stock_search":
        return {
          code: 0,
          message: "ok",
          data: [{ symbol: "CN:SZ:301217", name: "铜冠铜箔", code: "301217", market: "A股", exchange: "SZ" }],
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("浦发银行")).toBeInTheDocument();
  const search = screen.getByPlaceholderText("搜索股票 / 指数");
  fireEvent.change(search, { target: { value: "铜冠铜箔" } });
  fireEvent.keyDown(search, { key: "Enter" });

  expect(await screen.findByRole("button", { name: "铜冠铜箔 301217" })).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "铜冠铜箔 301217" }));

  await waitFor(() => {
    expect(window.location.hash).toBe("#/chart/kline?symbol=CN%3ASZ%3A301217&period=day&adjust=qfq");
  });
  expect(calls).toContainEqual({ command: "stock_search", payload: { keyword: "铜冠铜箔" } });
  expect(await screen.findByText("铜冠铜箔")).toBeInTheDocument();
});

test("全屏 K 线图顶部移除占位工具并通过返回按钮回到来源页", async () => {
  mockIPC((command) => {
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.88,
            change_amount: 0.18,
            change_percent: 2.34,
            open: 7.65,
            high: 7.92,
            low: 7.58,
            pre_close: 7.7,
            volume: 456200,
            amount: 358900000,
            turnover_rate: 0.63,
            quote_time: "2026-06-25T10:42:18+08:00",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: { symbol: "600000.SH", name: "浦发银行", code: "600000", exchange: "SH", market: "CN", industry: "银行" },
        };
      case "market_kline":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{ symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-25", open: 7.8, high: 7.92, low: 7.74, close: 7.88, volume: 456200 }],
          },
        };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: {} } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(
    <MemoryRouter initialEntries={[{ pathname: "/chart/kline", search: "?symbol=600000.SH&period=day&adjust=qfq", state: { from: "/watchlist" } }]}>
      <AntApp>
        <Routes>
          <Route path="/chart/kline" element={<FullscreenKlinePage />} />
          <Route path="/watchlist" element={<ChartSourceProbe />} />
        </Routes>
      </AntApp>
    </MemoryRouter>,
  );

  expect(await screen.findByText("浦发银行")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "设置" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "全屏" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "更多" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "趋势线" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "水平线" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "画线工具" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "清除画线" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "图表设置" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "快照" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "布局" })).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "返回" }));

  expect(screen.getByText("source:/watchlist")).toBeInTheDocument();
});

test("全屏 K 线图分钟周期直连后端分钟 K 线", async () => {
  window.location.hash = "#/chart/kline?symbol=600000.SH&period=day&adjust=qfq";
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.88,
            change_amount: 0.18,
            change_percent: 2.34,
            open: 7.65,
            high: 7.92,
            low: 7.58,
            pre_close: 7.7,
            volume: 456200,
            amount: 358900000,
            turnover_rate: 0.63,
            quote_time: "2026-06-25T10:42:18+08:00",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: { symbol: "600000.SH", name: "浦发银行", code: "600000", exchange: "SH", market: "CN", industry: "银行" },
        };
      case "market_kline": {
        const period = (payload as { period: string }).period;
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { symbol: "600000.SH", period, adjust: "qfq", trade_date: "2026-06-25 10:30", open: 7.7, high: 7.82, low: 7.68, close: 7.78, volume: 120000 },
              { symbol: "600000.SH", period, adjust: "qfq", trade_date: "2026-06-25 10:35", open: 7.78, high: 7.9, low: 7.76, close: 7.88, volume: 156200 },
            ],
          },
        };
      }
      case "market_indicators": {
        const period = (payload as { period: string }).period;
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            period,
            adjust: "qfq",
            indicators: {
              ma: { ma5: [7.78, 7.82], ma10: [7.7, 7.75], ma20: [7.6, 7.66], ma60: [7.4, 7.45] },
              macd: { dif: [0.03, 0.04], dea: [0.01, 0.02], bar: [0.04, 0.05] },
              kdj: { k: [58, 62], d: [51, 54], j: [72, 78] },
            },
          },
        };
      }
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("浦发银行")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "5分" }));

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "market_kline", payload: { symbol: "600000.SH", period: "5m", adjust: "qfq", limit: 120 } });
  });
  expect(calls).toContainEqual({
    command: "market_indicators",
    payload: { symbol: "600000.SH", period: "5m", adjust: "qfq", limit: 120, indicators: ["ma", "rsi", "macd", "kdj", "boll"] },
  });
});

test("全屏 K 线图分时周期读取后端分时走势", async () => {
  window.location.hash = "#/chart/kline?symbol=600000.SH&period=minute&adjust=qfq";
  const calls: Array<{ command: string; payload?: unknown }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.88,
            change_amount: 0.18,
            change_percent: 2.34,
            open: 7.65,
            high: 7.92,
            low: 7.58,
            pre_close: 7.7,
            volume: 456200,
            amount: 358900000,
            turnover_rate: 0.63,
            quote_time: "2026-06-25T10:42:18+08:00",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: { symbol: "600000.SH", name: "浦发银行", code: "600000", exchange: "SH", market: "CN", industry: "银行" },
        };
      case "market_kline":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { symbol: "600000.SH", period: "minute", adjust: "qfq", trade_date: "2026-06-25 10:30", open: 7.7, high: 7.82, low: 7.68, close: 7.78, volume: 120000 },
              { symbol: "600000.SH", period: "minute", adjust: "qfq", trade_date: "2026-06-25 10:31", open: 7.78, high: 7.9, low: 7.76, close: 7.88, volume: 156200 },
            ],
          },
        };
      case "market_indicators":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            period: "minute",
            adjust: "qfq",
            indicators: {
              ma: { ma5: [7.78, 7.82], ma10: [7.7, 7.75], ma20: [7.6, 7.66], ma60: [7.4, 7.45] },
              macd: { dif: [0.03, 0.04], dea: [0.01, 0.02], bar: [0.04, 0.05] },
              kdj: { k: [58, 62], d: [51, 54], j: [72, 78] },
            },
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("浦发银行")).toBeInTheDocument();
  expect(calls).toContainEqual({ command: "market_kline", payload: { symbol: "600000.SH", period: "minute", adjust: "qfq", limit: 240 } });
  expect(calls).toContainEqual({
    command: "market_indicators",
    payload: { symbol: "600000.SH", period: "minute", adjust: "qfq", limit: 240, indicators: ["ma", "rsi", "macd", "kdj", "boll"] },
  });
});

test("隐藏验证页不通过主导航或辅助导航暴露", async () => {
  window.location.hash = "#/not-found";
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0", dbStatus: "ok" } };
      case "providers_status":
        return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("页面不存在")).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "任务调度" })).not.toBeInTheDocument();
});

test("旧 ai-settings 路由复用设置中心模型配置页", async () => {
  window.location.hash = "#/ai-settings";
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0", dbStatus: "ok" } };
      case "providers_status":
        return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByRole("tab", { name: "模型设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("模型配置列表")).toBeInTheDocument();
});

test("启动初始化期间展示等待页并锁定业务入口", async () => {
  const calls: string[] = [];
  mockIPC((command) => {
    calls.push(command);
    if (command === "app_boot_status") {
      return {
        code: 0,
        message: "ok",
        data: {
          ready: false,
          progress: 68,
          currentStepId: "sqlite_migration",
          steps: [
            { id: "sidecar", index: 1, title: "启动 Go Core Sidecar", status: "completed", badgeText: "已完成" },
            { id: "workspace", index: 2, title: "检查本地工作区与目录权限", status: "completed", badgeText: "已完成" },
            { id: "sqlite_migration", index: 3, title: "SQLite 数据库迁移 / 重建索引", status: "running", badgeText: "进行中" },
            { id: "tokenizer", index: 4, title: "加载分词器（GSE / 全文检索词典）", status: "pending", badgeText: "等待中" },
            { id: "data_source", index: 5, title: "初始化数据源配置", status: "pending", badgeText: "等待中" },
            { id: "market_cache", index: 6, title: "同步基础行情快照与资讯缓存", status: "pending", badgeText: "等待中" },
            { id: "ready", index: 7, title: "完成基础检查并进入工作台", status: "pending", badgeText: "等待中" },
          ],
          taskDetail: {
            taskId: "boot-1",
            elapsed: "00:00:01",
            currentStage: "SQLite 数据库迁移 / 重建索引",
            remaining: "计算中",
          },
          logs: [{ id: "1", time: "15:29:41", status: "success", message: "sidecar ready" }],
        },
      };
    }
    if (command === "logs_open_directory") {
      return { opened: true };
    }
    throw new Error(`initialization should not call ${command}`);
  });

  render(<App initialBootState="initializing" bootStatusPollIntervalMs={60_000} />);

  expect(screen.getByRole("heading", { name: "正在初始化本地数据环境" })).toBeInTheDocument();
  expect(screen.getByText("初始化进度")).toBeInTheDocument();
  expect(screen.getByLabelText("初始化进度 10%")).toBeInTheDocument();
  expect(screen.getByText("waiting app boot status")).toBeInTheDocument();

  expect(await screen.findByText("68%")).toBeInTheDocument();
  expect(screen.getByText("SQLite 数据库迁移 / 重建索引")).toBeInTheDocument();
  expect(screen.getByText("boot-1")).toBeInTheDocument();
  expect(screen.getByText("sidecar ready")).toBeInTheDocument();
  expect(screen.getByText("仅供研究，不构成投资建议")).toBeInTheDocument();
  expect(screen.queryByRole("navigation", { name: "主导航" })).not.toBeInTheDocument();
  expect(screen.queryByPlaceholderText("搜索股票名称 / 代码 / 拼音")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /查看初始化日志/ }));
  await waitFor(() => {
    expect(calls).toContain("logs_open_directory");
  });

  expect(window.location.hash).toBe("");
  await waitFor(() => {
    expect(calls).toEqual(["app_boot_status", "logs_open_directory"]);
  });
});

test("启动状态读取失败时停留初始化页并展示错误", async () => {
  mockIPC((command) => {
    if (command === "app_boot_status") {
      throw new Error("sidecar http error: http://127.0.0.1:50001/api/internal/boot");
    }
    throw new Error(`initialization should not call ${command}`);
  });

  render(<App initialBootState="initializing" bootStatusPollIntervalMs={60_000} />);

  expect(screen.getByRole("heading", { name: "正在初始化本地数据环境" })).toBeInTheDocument();
  expect(await screen.findByText("启动状态读取失败，请稍后重试")).toBeInTheDocument();
  expect(screen.getByText("启动状态读取失败")).toBeInTheDocument();
  expect(screen.queryByText("自选股涨跌分布")).not.toBeInTheDocument();
});

test("初始化等待完成后进入总览页面", async () => {
  vi.useFakeTimers();
  let bootStatusCallCount = 0;
  mockIPC((command) => {
    switch (command) {
      case "app_boot_status":
        bootStatusCallCount += 1;
        return {
          code: 0,
          message: "ok",
          data: {
            ready: bootStatusCallCount >= 2,
            progress: bootStatusCallCount >= 2 ? 100 : 68,
            currentStepId: bootStatusCallCount >= 2 ? "ready" : "sqlite_migration",
            steps: [
              { id: "sidecar", index: 1, title: "启动 Go Core Sidecar", status: "completed", badgeText: "已完成" },
              { id: "sqlite_migration", index: 3, title: "SQLite 数据库迁移 / 重建索引", status: bootStatusCallCount >= 2 ? "completed" : "running", badgeText: bootStatusCallCount >= 2 ? "已完成" : "进行中" },
              { id: "ready", index: 7, title: "完成基础检查并进入工作台", status: bootStatusCallCount >= 2 ? "completed" : "pending", badgeText: bootStatusCallCount >= 2 ? "已完成" : "等待中" },
            ],
            taskDetail: {
              taskId: "boot-1",
              elapsed: "00:00:01",
              currentStage: bootStatusCallCount >= 2 ? "完成基础检查并进入工作台" : "SQLite 数据库迁移 / 重建索引",
              remaining: bootStatusCallCount >= 2 ? "00:00:00" : "计算中",
            },
            logs: [{ id: "1", time: "15:29:41", status: "success", message: "sidecar ready" }],
          },
        };
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0", dbStatus: "ok" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { items: [] } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: emptyDashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change_percent: 0 } };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="initializing" bootStatusPollIntervalMs={20} minimumInitializationVisibleMs={0} />);

  expect(screen.getByRole("heading", { name: "正在初始化本地数据环境" })).toBeInTheDocument();
  await act(async () => {
    await Promise.resolve();
  });
  expect(bootStatusCallCount).toBe(1);

  await act(async () => {
    vi.advanceTimersByTime(20);
    await Promise.resolve();
  });
  await act(async () => {
    vi.advanceTimersByTime(800);
    await Promise.resolve();
  });
  vi.useRealTimers();

  expect(await screen.findByText("自选股涨跌分布")).toBeInTheDocument();
});

test("总览路由展示 dashboard store 读取的真实后端数据", async () => {
  mockIPC((command, payload) => {
    const args = payload as { symbol?: string };
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0", dbStatus: "ok" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { providers: [] } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: dashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: { symbol: args.symbol, price: 3000, change_percent: 0.2, quote_time: "2026-06-24T15:00:00+08:00" },
        };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  expect(await screen.findByText("A 股早盘新闻")).toBeInTheDocument();
  expect(screen.getByText("浦发银行分析")).toBeInTheDocument();
  expect(screen.getByText("生成分析报告")).toBeInTheDocument();
  expect(screen.queryByText("暂无热点数据")).not.toBeInTheDocument();
});

test("同一桌面会话已完成初始化后重新挂载不再回到初始化页", async () => {
  vi.useFakeTimers();
  let bootStatusCallCount = 0;
  mockIPC((command) => {
    switch (command) {
      case "app_boot_status":
        bootStatusCallCount += 1;
        return {
          code: 0,
          message: "ok",
          data: {
            ready: true,
            progress: 100,
            currentStepId: "ready",
            steps: [
              { id: "sidecar", index: 1, title: "启动 Go Core Sidecar", status: "completed", badgeText: "已完成" },
              { id: "ready", index: 7, title: "完成基础检查并进入工作台", status: "completed", badgeText: "已完成" },
            ],
            taskDetail: {
              taskId: "boot-1",
              elapsed: "00:00:01",
              currentStage: "完成基础检查并进入工作台",
              remaining: "00:00:00",
            },
            logs: [{ id: "1", time: "15:29:41", status: "success", message: "initialization completed" }],
          },
        };
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0", dbStatus: "ok" } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: emptyDashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change_percent: 0 } };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  const { unmount } = render(<App initialBootState="initializing" minimumInitializationVisibleMs={0} />);
  await act(async () => {
    await Promise.resolve();
    await vi.advanceTimersByTimeAsync(800);
  });
  expect(screen.getByRole("navigation", { name: "主导航" })).toBeInTheDocument();
  expect(screen.queryByRole("heading", { name: "正在初始化本地数据环境" })).not.toBeInTheDocument();
  expect(bootStatusCallCount).toBe(1);

  unmount();
  render(<App initialBootState="initializing" minimumInitializationVisibleMs={0} />);

  expect(screen.queryByRole("heading", { name: "正在初始化本地数据环境" })).not.toBeInTheDocument();
  expect(screen.getByRole("navigation", { name: "主导航" })).toBeInTheDocument();
  expect(bootStatusCallCount).toBe(1);
  vi.useRealTimers();
});

test("初始化首次即 ready 时仍保留启动页最短展示时间", async () => {
  vi.useFakeTimers();
  mockIPC((command) => {
    switch (command) {
      case "app_boot_status":
        return {
          code: 0,
          message: "ok",
          data: {
            ready: true,
            progress: 100,
            currentStepId: "ready",
            steps: [
              { id: "sidecar", index: 1, title: "启动 Go Core Sidecar", status: "completed", badgeText: "已完成" },
              { id: "ready", index: 7, title: "完成基础检查并进入工作台", status: "completed", badgeText: "已完成" },
            ],
            taskDetail: {
              taskId: "boot-1",
              elapsed: "00:00:01",
              currentStage: "完成基础检查并进入工作台",
              remaining: "00:00:00",
            },
            logs: [{ id: "1", time: "15:29:41", status: "success", message: "initialization completed" }],
          },
        };
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0", dbStatus: "ok" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { items: [] } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: emptyDashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change_percent: 0 } };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="initializing" bootStatusPollIntervalMs={20} />);

  expect(screen.getByRole("heading", { name: "正在初始化本地数据环境" })).toBeInTheDocument();
  await act(async () => {
    await Promise.resolve();
  });
  expect(screen.getByText("100%")).toBeInTheDocument();
  expect(screen.queryByText("自选股涨跌分布")).not.toBeInTheDocument();

  await act(async () => {
    vi.advanceTimersByTime(2999);
    await Promise.resolve();
  });
  expect(screen.queryByText("自选股涨跌分布")).not.toBeInTheDocument();

  await act(async () => {
    vi.advanceTimersByTime(1);
    await Promise.resolve();
  });
  vi.useRealTimers();

  expect(await screen.findByText("自选股涨跌分布")).toBeInTheDocument();
});

test("App 默认不伪装 core health，刷新后通过 typed invoke service 展示状态", async () => {
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
    if (command === "providers_status") {
      return {
        code: 0,
        message: "ok",
        data: { items: [{ name: "market", source: "sina", available: true, last_error: "" }] },
      };
    }
    expect(command).toBe("core_health");
    return {
      code: 0,
      message: "ok",
      data: {
        status: "ok",
        version: "0.1.0",
        dbStatus: "ok",
      },
    };
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("自选股涨跌分布")).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: /刷新/ }));

  await waitFor(() => {
    expect(screen.getByText(/本地核心服务已连接/)).toBeInTheDocument();
  });
  expect(screen.getByText(/版本\s*0\.1\.0/)).toBeInTheDocument();
});

test("运行期左下角状态在总览页也主动刷新 core 和数据源状态", async () => {
  const calls: string[] = [];
  mockIPC((command) => {
    calls.push(command);
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { version: "0.1.0", dbStatus: "ok" } };
      case "providers_status":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{ name: "market", source: "sina", available: true, last_error: "" }],
          },
        };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: { ...emptyDashboardFixture, provider_statuses: [] } };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 3181.3, change: 0, change_percent: 0 } };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(calls).toContain("providers_status");
  });
  const dataSourceRow = screen.getByText("数据源").closest("div");
  expect(dataSourceRow).not.toBeNull();
  expect(within(dataSourceRow as HTMLElement).getByText("正常")).toBeInTheDocument();
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
  expect(screen.getByRole("link", { name: "总览" })).toHaveAttribute("href", "#/");
  expect(screen.getByRole("link", { name: "自选股" })).toHaveAttribute("href", "#/watchlist");
  expect(screen.getByRole("link", { name: "AI 分析" })).toHaveAttribute("href", "#/analysis");
  expect(screen.getByRole("link", { name: "资讯中心" })).toHaveAttribute("href", "#/news");

  expect(screen.queryByText(/本地核心服务已连接/)).not.toBeInTheDocument();
});

test("Dashboard 首页展示真实数据空态并通过真实 command 读取", async () => {
  const calls: Array<{ command: string; payload?: unknown }> = [];
  useDashboardStore.setState({ state: null, loading: false, error: null, lastLoadedAt: null });
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { items: [] } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: emptyDashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change_percent: 0 } };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("自选股涨跌分布")).toBeInTheDocument();
  });
  await waitFor(() => {
    expect(calls.map((call) => call.command)).toContain("dashboard_summary");
  });
  expect(screen.getByPlaceholderText("搜索股票名称 / 代码 / 拼音")).toBeInTheDocument();
  expect(screen.getByText("今日热点")).toBeInTheDocument();
  expect(screen.getByText("最近分析报告")).toBeInTheDocument();
  expect(screen.getByText("最近任务状态")).toBeInTheDocument();
  expect(screen.getByText("上涨")).toBeInTheDocument();
  expect(screen.getByText("下跌")).toBeInTheDocument();
  expect(screen.getByText("平盘")).toBeInTheDocument();
  expect(screen.queryByText("暂无市场指数数据")).not.toBeInTheDocument();
  expect(await screen.findByText("暂无市场新闻")).toBeInTheDocument();
  expect(screen.queryByText("宁德时代深度分析")).not.toBeInTheDocument();
  expect(screen.queryByText("A股 已收盘")).not.toBeInTheDocument();
  expect(screen.queryByText("宁德时代（300750）深度分析报告")).not.toBeInTheDocument();
  expect(screen.getByText("仅供研究，不构成投资建议。")).toBeInTheDocument();
  expect(screen.getByText("仅作研究辅助，不构成投资建议")).toBeInTheDocument();
  expect(document.querySelector(".ant-badge-count")).not.toBeInTheDocument();
  expect(calls.map((call) => call.command)).toContain("dashboard_summary");
  expect(calls.map((call) => call.command)).toContain("watchlist_list");
  expect(calls.map((call) => call.command)).toContain("market_kline");
});

test("Dashboard 首页默认不展示伪造市场状态时间", async () => {
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { items: [] } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: emptyDashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change_percent: 0 } };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("自选股涨跌分布")).toBeInTheDocument();
  });
  expect(screen.queryByText("A股 已收盘")).not.toBeInTheDocument();
  expect(screen.queryByText((content) => content.includes("2025") && content.includes("15:30:00"))).not.toBeInTheDocument();
});

test("Dashboard 首页顶部状态在交易时段显示 A 股交易中", async () => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-06-25T09:56:50+08:00"));
  mockIPC((command, payload) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { items: [] } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: emptyDashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote": {
        const args = payload as { symbol?: string };
        return {
          code: 0,
          message: "ok",
          data: { symbol: args.symbol, price: 4102.03, change_percent: -0.21, quote_time: "2026-06-25T09:56:49+08:00" },
        };
      }
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
  expect(screen.getByText("A股 交易中")).toBeInTheDocument();
  expect(screen.getByText("A股 交易中").closest(".ant-tag")).toHaveClass("app-market-status-tag-success");
  expect(screen.queryByText("A股 已收盘")).not.toBeInTheDocument();
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
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [] } };
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
  expect(calls.map((call) => call.command)).not.toContain("search_reports");
  expect(calls.map((call) => call.command)).not.toContain("search_news");
  expect(calls.map((call) => call.command)).not.toContain("search_watchlist_notes");
  expect(calls.map((call) => call.command)).not.toContain("search_global");
  expect(screen.queryByText("查看全部搜索结果")).not.toBeInTheDocument();
  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "600000.SH" })).toBeInTheDocument();
  });
  expect(calls).toContainEqual({ command: "settings_get", payload: { keys: ["kline.default_period", "kline.default_adjust"] } });
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "market_kline", payload: { symbol: "600000.SH", period: "day", adjust: "qfq", limit: 120 } });
  });
  expect(screen.getByText("暂无新闻资讯")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /返回/ }));
  await waitFor(() => {
    expect(window.location.hash).toBe("#/");
  });
});

test("TopBar 通知按钮展示未读角标并点击通知跳转白名单页面", async () => {
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
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change: 0, change_percent: 0 } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 2 } };
      case "notifications_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 7,
                type: "task",
                level: "success",
                title: "任务完成",
                content: "浦发银行分析已完成",
                route: "/reports/3",
                is_read: false,
                created_at: "2026-06-23T09:00:00Z",
              },
            ],
            total: 1,
            limit: 20,
            offset: 0,
          },
        };
      case "notifications_mark_read":
        return { code: 0, message: "ok", data: { ok: true } };
      case "report_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 3,
            task_id: "task-report-3",
            symbol: "600000.SH",
            title: "浦发银行分析",
            analysis_type: "stock_full",
            model_name: "DeepSeek",
            content_markdown: "## 结论",
            risk_summary: "仅供研究",
            created_at: "2026-06-23T09:00:00Z",
            updated_at: "2026-06-23T09:00:00Z",
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("2")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /通知/ }));

  const notification = await screen.findByRole("button", { name: /通知：任务完成/ });
  expect(screen.getByText("浦发银行分析已完成")).toBeInTheDocument();
  fireEvent.click(notification);

  await waitFor(() => {
    expect(window.location.hash).toBe("#/reports/3");
  });
  expect(calls).toContainEqual({ command: "notifications_mark_read", payload: { payload: { ids: [7] } } });
});

test("TopBar 点击未知通知 route 只标记已读不跳转", async () => {
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
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change: 0, change_percent: 0 } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 1 } };
      case "notifications_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 8,
                type: "provider",
                level: "warning",
                title: "数据源异常",
                content: "Provider 状态异常",
                route: "/provider/debug/8",
                is_read: false,
                created_at: "2026-06-23T09:10:00Z",
              },
            ],
            total: 1,
            limit: 20,
            offset: 0,
          },
        };
      case "notifications_mark_read":
        return { code: 0, message: "ok", data: { ok: true } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await screen.findByText("1");
  fireEvent.click(screen.getByRole("button", { name: /通知/ }));
  fireEvent.click(await screen.findByRole("button", { name: /通知：数据源异常/ }));

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "notifications_mark_read", payload: { payload: { ids: [8] } } });
  });
  expect(window.location.hash).toBe("");
});

test("Dashboard 首页不展示旧版 Provider 异常空态", async () => {
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
    if (command === "market_kline") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("自选股涨跌分布")).toBeInTheDocument();
  });
  expect(screen.getByText("最近分析报告")).toBeInTheDocument();
  expect(screen.getByText("最近任务状态")).toBeInTheDocument();
  expect(screen.queryByText("Provider 异常")).not.toBeInTheDocument();
});

test("自选股页面默认展示空态并支持备注范围搜索", async () => {
  window.location.hash = "#/watchlist";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "watchlist_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "search_watchlist_notes") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "watchlist_note:1",
            doc_type: "watchlist_note",
            ref_id: "1",
            symbol: "CN:SZ:300308",
            title: "中际旭创",
            summary: "北美客户订单",
            source: "watchlist_note",
            source_time: "2026-06-22T10:22:00Z",
            score: 1,
            highlights: ["光模块"],
          },
        ],
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "自选股" })).toBeInTheDocument();
  });
  expect(screen.getByRole("radio", { name: /卡片视图/ })).toBeChecked();
  expect(screen.getByText("暂无匹配自选股")).toBeInTheDocument();
  expect(screen.queryByText("贵州茅台")).not.toBeInTheDocument();
  expect(screen.queryByText("宁德时代")).not.toBeInTheDocument();
  expect(screen.queryByText("自选概览")).not.toBeInTheDocument();
  expect(screen.queryByText("市场分布")).not.toBeInTheDocument();
  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();

  const searchInput = screen.getByPlaceholderText("搜索自选股");
  fireEvent.change(searchInput, { target: { value: "北美客户" } });
  fireEvent.keyDown(searchInput, { key: "Enter", code: "Enter" });
  await waitFor(() => {
    expect(screen.getByText("中际旭创")).toBeInTheDocument();
  });
  expect(screen.getByText("北美客户订单")).toBeInTheDocument();
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([
    {
      command: "watchlist_list",
      payload: {},
    },
    {
      command: "settings_get",
      payload: { keys: ["quote.refresh_interval", "data_source.quote_refresh_interval"] },
    },
    {
      command: "search_watchlist_notes",
      payload: { payload: { keyword: "北美客户", symbols: [], limit: 20, offset: 0, sort: "relevance" } },
    },
  ]);
  expect(calls.map((call) => call.command)).not.toContain("search_global");
  expect(calls.map((call) => call.command)).not.toContain("search_news");
});

test("自选股页面可切换表格视图且不暴露交易入口", async () => {
  window.location.hash = "#/watchlist";
  mockIPC((command) => {
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "自选股" })).toBeInTheDocument();
  });
  expect(screen.getByRole("radio", { name: /卡片视图/ })).toBeChecked();
  fireEvent.click(screen.getByRole("radio", { name: /表格视图/ }));
  expect(screen.getByRole("radio", { name: /表格视图/ })).toBeChecked();
  expect(screen.getByRole("columnheader", { name: "股票名称" })).toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
});

test("自选股页面使用后端返回的股票资料字段展示名称、行业和标签", async () => {
  window.location.hash = "#/watchlist";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "watchlist_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{
              id: 1,
              symbol: "600000.SH",
              sort_order: 10,
              tags: ["自选"],
              note: "低估值观察",
              name: "浦发银行",
              code: "600000",
              market: "CN",
              exchange: "SH",
              industry: "银行",
              concepts: ["低估值", "大金融"],
              list_date: "1999-11-10",
              status: "active",
              full_name: "上海浦东发展银行股份有限公司",
            }],
          },
        };
      case "market_quote":
        const quotePayload = payload as { symbol: string };
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: quotePayload.symbol,
            price: 7.12,
            change_amount: 0.1,
            change_percent: 1.42,
            amount: 8780000,
            turnover_rate: 1.08,
            pe: 5.6,
            quote_time: "2026-06-24T10:00:00Z",
          },
        };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("浦发银行")).toBeInTheDocument();
  });
  expect(screen.getByText("银行")).toBeInTheDocument();
  expect(screen.getByText("自选")).toBeInTheDocument();
  expect(screen.queryByText("未分类")).not.toBeInTheDocument();
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([
    { command: "watchlist_list", payload: {} },
    { command: "settings_get", payload: { keys: ["quote.refresh_interval", "data_source.quote_refresh_interval"] } },
  ]);
});

test("个股详情页进入后读取真实行情、K线、指标和新闻", async () => {
  window.location.hash = "#/stocks/600000.SH";
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
              { key: "kline.default_period", value: "day" },
              { key: "kline.default_adjust", value: "qfq" },
            ],
          },
        };
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.12,
            change_amount: 0.1,
            change_percent: 1.42,
            open: 7.01,
            high: 7.2,
            low: 6.98,
            pre_close: 7.02,
            volume: 1234000,
            amount: 8780000,
            turnover_rate: 1.08,
            quote_time: "2026-06-23T10:00:00Z",
            provider: "sina",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            name: "浦发银行",
            code: "600000",
            market: "CN",
            exchange: "SH",
            industry: "银行",
            concepts: ["低估值", "大金融"],
            list_date: "1999-11-10",
            status: "active",
            full_name: "上海浦东发展银行股份有限公司",
          },
        };
      case "market_kline": {
        const args = payload as { symbol: string; period: string; adjust: string; limit: number };
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                symbol: args.symbol,
                period: args.period,
                adjust: args.adjust,
                trade_date: "2026-06-23",
                open: 7.01,
                high: 7.2,
                low: 6.98,
                close: args.period === "week" ? 7.18 : 7.12,
                volume: 1234000,
                provider: "tencent",
              },
            ],
          },
        };
      }
      case "market_indicators": {
        const args = payload as { symbol: string; period: string; adjust: string; limit: number; indicators: string[] };
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: args.symbol,
            period: args.period,
            adjust: args.adjust,
            indicators: { ma5: args.period === "week" ? 7.18 : 7.12, macd_dif: 0.12, rsi6: 58.6 },
          },
        };
      }
      case "news_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                source: "财联社",
                title: "浦发银行发布最新经营动态",
                url: "https://example.com/news/1",
                published_at: "2026-06-23T09:30:00Z",
              },
            ],
          },
        };
      case "watchlist_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{
              id: 9,
              symbol: "600000.SH",
              sort_order: 10,
              tags: ["核心", "银行"],
              note: "跟踪净息差",
            }],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("浦发银行发布最新经营动态")).toBeInTheDocument();
  });
  expect(screen.getByRole("heading", { name: "浦发银行" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /600000\.SH/ })).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /个股详情/ })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: /返回/ })).toBeInTheDocument();
  expect(screen.getAllByText("7.12").length).toBeGreaterThan(0);
  expect(screen.getByRole("heading", { name: "K线图" })).toBeInTheDocument();
  expect(screen.getByLabelText("K线图")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /全屏/ })).toBeInTheDocument();
  expect(screen.getByText("MA.MA5")).toBeInTheDocument();
  expect(screen.getByText("MACD.DIF")).toBeInTheDocument();
  expect(screen.getByText("RSI.RSI6")).toBeInTheDocument();
  expect(screen.getByText("银行 / 暂无")).toBeInTheDocument();
  expect(screen.getAllByText("低估值").length).toBeGreaterThan(0);
  expect(screen.getByText("上海浦东发展银行股份有限公司")).toBeInTheDocument();
  expect(screen.getByText("核心")).toBeInTheDocument();
  expect(screen.getByText("跟踪净息差")).toBeInTheDocument();
  expect(screen.queryByText("生益科技：一季度归母净利润同比增长18.35% 产品结构持续优化")).not.toBeInTheDocument();
  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([
    { command: "settings_get", payload: { keys: ["kline.default_period", "kline.default_adjust"] } },
    { command: "settings_get", payload: { keys: ["quote.refresh_interval", "data_source.quote_refresh_interval"] } },
    { command: "market_quote", payload: { symbol: "600000.SH" } },
    { command: "stock_profile", payload: { symbol: "600000.SH" } },
    { command: "market_kline", payload: { symbol: "600000.SH", period: "day", adjust: "qfq", limit: 120 } },
    { command: "market_indicators", payload: { symbol: "600000.SH", period: "day", adjust: "qfq", limit: 120, indicators: ["ma", "rsi", "macd", "kdj", "boll"] } },
    { command: "news_list", payload: { symbol: "600000.SH", limit: 20 } },
    { command: "watchlist_list", payload: {} },
  ]);

  fireEvent.click(screen.getByRole("button", { name: /刷新行情/ }));
  await waitFor(() => {
    expect(calls.filter((call) => call.command === "market_quote")).toHaveLength(2);
  });
  fireEvent.click(screen.getByRole("button", { name: /发起 AI 分析/ }));
  expect(window.location.hash).toBe("#/analysis?symbol=600000.SH");
});

test("个股详情页全屏按钮进入全屏K线图并携带当前上下文", async () => {
  window.location.hash = "#/stocks/600000.SH";
  mockIPC((command, payload) => {
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [{ key: "kline.default_period", value: "day" }, { key: "kline.default_adjust", value: "qfq" }] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", price: 7.12, change_percent: 1.42 } };
      case "stock_profile":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", name: "浦发银行", code: "600000", exchange: "SH", industry: "银行" } };
      case "market_kline":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-20", open: 7.01, high: 7.2, low: 6.98, close: 7.12, volume: 1234000 },
              { symbol: "600000.SH", period: "day", adjust: "qfq", trade_date: "2026-06-23", open: 7.12, high: 7.28, low: 7.08, close: 7.22, volume: 1334000 },
            ],
          },
        };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: { ma5: 7.12 } } };
      case "news_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App initialBootState="ready" />);

  await screen.findByRole("heading", { name: "浦发银行" });
  fireEvent.click(screen.getByRole("button", { name: /全屏/ }));

  expect(window.location.hash).toBe("#/chart/kline?symbol=600000.SH&period=day&adjust=qfq");
  expect(await screen.findByLabelText("全屏K线趋势图页")).toBeInTheDocument();
});

test("个股详情页个股新闻读取失败时仍展示行情和基础资料", async () => {
  window.location.hash = "#/stocks/600000.SH";
  mockIPC((command, payload) => {
    switch (command) {
      case "settings_get":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { key: "kline.default_period", value: "day" },
              { key: "kline.default_adjust", value: "qfq" },
            ],
          },
        };
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.12,
            change_percent: 1.42,
            open: 7.01,
            high: 7.2,
            low: 6.98,
            pre_close: 7.02,
            volume: 1234000,
            amount: 8780000,
            quote_time: "2026-06-23T10:00:00Z",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            name: "浦发银行",
            code: "600000",
            market: "CN",
            exchange: "SH",
            industry: "银行",
          },
        };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: {} } };
      case "news_list":
        throw new Error("sidecar http error: HTTP status server error (502 Bad Gateway) for url (http://127.0.0.1:51932/api/news/list)");
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command} ${(payload as object | undefined) ? JSON.stringify(payload) : ""}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "浦发银行" })).toBeInTheDocument();
  });
  expect(screen.getAllByText("7.12").length).toBeGreaterThan(0);
  expect(screen.queryByText(/个股详情读取失败/)).not.toBeInTheDocument();
  expect(screen.queryByText(/Bad Gateway/)).not.toBeInTheDocument();
});

test("个股详情页自选股读取失败时仍展示行情和基础资料", async () => {
  window.location.hash = "#/stocks/600000.SH";
  mockIPC((command, payload) => {
    switch (command) {
      case "settings_get":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { key: "kline.default_period", value: "day" },
              { key: "kline.default_adjust", value: "qfq" },
            ],
          },
        };
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.12,
            change_percent: 1.42,
            open: 7.01,
            high: 7.2,
            low: 6.98,
            pre_close: 7.02,
            volume: 1234000,
            amount: 8780000,
            quote_time: "2026-06-23T10:00:00Z",
          },
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            name: "浦发银行",
            code: "600000",
            market: "CN",
            exchange: "SH",
            industry: "银行",
          },
        };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: {} } };
      case "news_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "watchlist_list":
        throw new Error("sidecar http error: HTTP status server error (503 Service Unavailable) for url (http://127.0.0.1:51932/api/watchlist/list)");
      default:
        throw new Error(`unexpected command ${command} ${(payload as object | undefined) ? JSON.stringify(payload) : ""}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "浦发银行" })).toBeInTheDocument();
  });
  expect(screen.getAllByText("7.12").length).toBeGreaterThan(0);
  expect(screen.queryByText(/个股详情读取失败/)).not.toBeInTheDocument();
  expect(screen.queryByText(/Service Unavailable/)).not.toBeInTheDocument();
});

test("个股详情页按基础设置定时刷新 K 线数据", async () => {
  vi.useFakeTimers();
  window.location.hash = "#/stocks/600000.SH";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get": {
        const keys = ((payload as { keys?: string[] })?.keys ?? []) as string[];
        return {
          code: 0,
          message: "ok",
          data: {
            items: keys.map((key) => ({
              key,
              value: key === "quote.refresh_interval" || key === "data_source.quote_refresh_interval" ? "15s" : key === "kline.default_adjust" ? "qfq" : "day",
            })),
          },
        };
      }
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", price: 7.12, change_percent: 1.42 } };
      case "stock_profile":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", name: "浦发银行", code: "600000", market: "CN", exchange: "SH" } };
      case "market_kline":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{
              symbol: "600000.SH",
              period: "day",
              adjust: "qfq",
              trade_date: "2026-06-23",
              open: 7.01,
              high: 7.2,
              low: 6.98,
              close: 7.12,
            }],
          },
        };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: { ma5: 7.12 } } };
      case "news_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(
    <MemoryRouter initialEntries={["/stocks/600000.SH"]}>
      <AntApp>
        <Routes>
          <Route path="/stocks/:symbol" element={<StockDetailRoute />} />
        </Routes>
      </AntApp>
    </MemoryRouter>,
  );

  await act(async () => {
    await vi.dynamicImportSettled();
    await Promise.resolve();
    await Promise.resolve();
    await vi.advanceTimersByTimeAsync(1_000);
  });
  expect(calls.filter((call) => call.command === "market_kline")).toHaveLength(1);

  await act(async () => {
    await vi.advanceTimersByTimeAsync(15_000);
    await Promise.resolve();
    await Promise.resolve();
  });

  expect(calls.filter((call) => call.command === "market_kline")).toHaveLength(2);
});

test("个股详情页编辑标签与备注复用真实自选股记录", async () => {
  window.location.hash = "#/stocks/600000.SH";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", price: 7.12 } };
      case "stock_profile":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", name: "浦发银行", code: "600000", market: "CN", exchange: "SH" } };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: {} } };
      case "news_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "watchlist_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{
              id: 9,
              symbol: "600000.SH",
              sort_order: 10,
              tags: ["核心"],
              note: "原备注",
            }],
          },
        };
      case "watchlist_update":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 9,
            symbol: "600000.SH",
            sort_order: 10,
            tags: ["核心", "银行"],
            note: "更新备注",
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("原备注")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "编辑" }));
  const dialog = screen.getByRole("dialog");
  fireEvent.change(within(dialog).getByPlaceholderText("多个标签用逗号、空格或顿号分隔"), { target: { value: "核心，银行" } });
  fireEvent.change(within(dialog).getByPlaceholderText("记录你对该股票的关注点"), { target: { value: "更新备注" } });
  fireEvent.click(within(dialog).getByRole("button", { name: /保\s*存/ }));

  await waitFor(() => {
    expect(screen.getByText("更新备注")).toBeInTheDocument();
  });
  expect(calls).toContainEqual({
    command: "watchlist_update",
    payload: { payload: { id: 9, sort_order: 10, tags: ["核心", "银行"], note: "更新备注" } },
  });
});

test("个股详情页切换周期会按真实周期重新读取K线和指标", async () => {
  window.location.hash = "#/stocks/600000.SH";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "settings_get":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", price: 7.12 } };
      case "market_kline": {
        const args = payload as { symbol: string; period: string; adjust: string; limit: number };
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                symbol: args.symbol,
                period: args.period,
                adjust: args.adjust,
                trade_date: "2026-06-23",
                open: 7.01,
                high: 7.2,
                low: 6.98,
                close: args.period === "week" ? 7.18 : 7.12,
                volume: 1234000,
              },
            ],
          },
        };
      }
      case "market_indicators": {
        const args = payload as { symbol: string; period: string; adjust: string; limit: number; indicators: string[] };
        return { code: 0, message: "ok", data: { symbol: args.symbol, period: args.period, adjust: args.adjust, indicators: { ma5: args.period === "week" ? 7.18 : 7.12 } } };
      }
      case "news_list":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "600000.SH" })).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "周K" }));
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "market_kline", payload: { symbol: "600000.SH", period: "week", adjust: "qfq", limit: 120 } });
  });
  expect(calls).toContainEqual({ command: "market_indicators", payload: { symbol: "600000.SH", period: "week", adjust: "qfq", limit: 120, indicators: ["ma", "rsi", "macd", "kdj", "boll"] } });
  fireEvent.click(screen.getByText("MACD"));
  expect(screen.getByText("MACD")).toHaveClass("text-[#1677ff]");
  expect(screen.queryByText("AI分析摘要")).not.toBeInTheDocument();
  expect(screen.queryByText("历史报告")).not.toBeInTheDocument();
});

test("旧 ai-settings 路由保存 API Key 后只展示脱敏字段并清空明文输入", async () => {
  window.location.hash = "#/ai-settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    const args = payload as any;
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0", dbStatus: "ok" } };
      case "providers_status":
        return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      case "ai_config_list":
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
    expect(screen.getByRole("tab", { name: "模型设置" })).toHaveAttribute("aria-selected", "true");
  });
  fireEvent.click(screen.getByRole("button", { name: "+ 新建配置" }));
  fireEvent.change(screen.getByLabelText("配置名称"), { target: { value: "自定义接入点" } });
  fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "https://llm.example.com" } });
  fireEvent.change(screen.getByLabelText("模型名"), { target: { value: "gpt-4.1-mini" } });
  fireEvent.change(screen.getByLabelText("API Key"), { target: { value: "sk-live-secret" } });
  fireEvent.click(screen.getByRole("button", { name: "保存配置" }));

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
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0", dbStatus: "ok" } };
      case "providers_status":
        return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
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
  fireEvent.click(screen.getByRole("button", { name: "测试连接 OpenAI 主配置" }));

  await waitFor(() => {
    expect(screen.getByText("连接失败")).toBeInTheDocument();
  });
  expect(screen.queryByText("sk-live-secret")).not.toBeInTheDocument();
});

test("Prompt 模板页拒绝未支持变量且不会提交创建 command", async () => {
  window.location.hash = "#/settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0", dbStatus: "ok" } };
      case "providers_status":
        return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      case "prompt_templates_list":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  fireEvent.click(await screen.findByRole("tab", { name: "Prompt 配置" }));
  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "Prompt 模板" })).toBeInTheDocument();
  });
  fireEvent.change(screen.getByLabelText("模板名称"), { target: { value: "非法变量模板" } });
  fireEvent.change(screen.getByLabelText("Prompt 内容"), { target: { value: "分析 {{unsupported_var}}" } });
  fireEvent.click(screen.getByRole("button", { name: "保存" }));

  await waitFor(() => {
    expect(screen.getByText("变量 unsupported_var 不在首版白名单")).toBeInTheDocument();
  });
  expect(calls.some((call) => call.command === "prompt_templates_create")).toBe(false);
});

test("AI 分析页读取真实模型、Prompt 和股票上下文后创建分析任务", async () => {
  window.location.hash = "#/analysis";
  const calls: Array<{ command: string; payload?: any }> = [];
  const writeText = vi.fn<(text: string) => Promise<void>>(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
      case "prompt_templates_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 11,
                key: "stock_full_v1",
                name: "个股综合模板",
                type: "stock_full",
                description: "综合分析",
                content: "分析 {{stock_name}}",
                variables: ["stock_name"],
                is_builtin: true,
                builtin_locked: true,
              },
              {
                id: 12,
                key: "technical_v1",
                name: "技术分析模板",
                type: "technical",
                description: "技术分析",
                content: "分析 {{kline_summary}}",
                variables: ["kline_summary"],
                is_builtin: true,
                builtin_locked: true,
              },
            ],
          },
        };
      case "stock_search":
        return {
          code: 0,
          message: "ok",
          data: [{ symbol: "600000.SH", name: "浦发银行", code: "600000", market: "CN", exchange: "SH" }],
        };
      case "stock_profile":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            name: "浦发银行",
            code: "600000",
            market: "CN",
            exchange: "SH",
            industry: "银行",
            list_date: "1999-11-10",
            full_name: "上海浦东发展银行股份有限公司",
          },
        };
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: {
            symbol: "600000.SH",
            price: 7.12,
            change_amount: 0.1,
            change_percent: 1.42,
            open: 7.01,
            high: 7.2,
            low: 6.98,
            amount: 8780000,
            volume: 1234000,
            turnover_rate: 1.08,
            quote_time: "2026-06-23T10:00:00Z",
          },
        };
      case "market_kline":
        return {
          code: 0,
          message: "ok",
          data: {
            items: Array.from({ length: 25 }, (_, index) => ({
              symbol: "600000.SH",
              period: "day",
              adjust: "qfq",
              trade_date: `2026-06-${String(index + 1).padStart(2, "0")}`,
              open: 6 + index / 100,
              high: 6.1 + index / 100,
              low: 5.9 + index / 100,
              close: 6 + index / 100,
            })),
          },
        };
      case "market_indicators":
        return { code: 0, message: "ok", data: { symbol: "600000.SH", period: "day", adjust: "qfq", indicators: { ma5: 7.1, ma10: 7.05, ma20: 6.98, macd_dif: 0.12, rsi6: 58.6 } } };
      case "news_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                source: "财联社",
                title: "浦发银行发布最新经营动态",
                url: "https://example.com/news/1",
                published_at: "2026-06-23T09:30:00Z",
              },
            ],
          },
        };
      case "analysis_task_create":
        return { code: 0, message: "ok", data: { task_id: "analysis-600000", status: "PENDING" } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "AI 分析" })).toBeInTheDocument();
  });
  expect(screen.getByText("基于行情、K线、新闻与技术指标生成研究报告")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "AI 分析" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByRole("button", { name: /返回自选/ })).toBeInTheDocument();

  expect(screen.getByText("分析配置")).toBeInTheDocument();
  expect(screen.getByText("股票选择")).toBeInTheDocument();
  expect(screen.queryByDisplayValue(/生益科技/)).not.toBeInTheDocument();
  expect(screen.getByText("个股综合分析")).toBeInTheDocument();
  await waitFor(() => {
    expect(screen.getByText("DeepSeek (DeepSeek-V3)")).toBeInTheDocument();
  });
  expect(screen.getByText("个股综合模板")).toBeInTheDocument();
  expect(screen.getByText("管理模板")).toBeInTheDocument();

  expect(screen.getByText("可选持仓上下文")).toBeInTheDocument();
  expect(screen.getByText("成本价（元）")).toBeInTheDocument();
  expect(screen.getByText("股数（股）")).toBeInTheDocument();
  expect(screen.getByText("风险偏好")).toBeInTheDocument();
  expect(screen.getByText("仅用于本次分析上下文，不落库")).toBeInTheDocument();

  expect(screen.getByText("数据上下文预览")).toBeInTheDocument();
  expect(screen.getByText("基础信息摘要")).toBeInTheDocument();
  expect(screen.getByText("最新行情摘要")).toBeInTheDocument();
  expect(screen.getByText("K线概况（日K）")).toBeInTheDocument();
  expect(screen.getByText("技术指标摘要")).toBeInTheDocument();
  expect(screen.getByText("相关新闻摘要")).toBeInTheDocument();
  const stockInput = screen.getAllByRole("combobox")[0];
  fireEvent.change(stockInput, { target: { value: "浦发" } });
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "stock_search", payload: { keyword: "浦发" } });
  });
  fireEvent.click(await screen.findByText("浦发银行"));
  await waitFor(() => {
    expect(screen.getByText("上海浦东发展银行股份有限公司")).toBeInTheDocument();
  });
  expect(screen.getByText("浦发银行发布最新经营动态")).toBeInTheDocument();
  expect(screen.getByText("7.12")).toBeInTheDocument();
  expect(calls).toContainEqual({ command: "stock_profile", payload: { symbol: "600000.SH" } });
  expect(calls).toContainEqual({ command: "market_quote", payload: { symbol: "600000.SH" } });
  expect(calls).toContainEqual({ command: "market_kline", payload: { symbol: "600000.SH", period: "day", adjust: "qfq", limit: 120 } });
  expect(calls).toContainEqual({
    command: "market_indicators",
    payload: { symbol: "600000.SH", period: "day", adjust: "qfq", limit: 120, indicators: ["ma", "rsi", "macd", "kdj", "boll"] },
  });
  expect(calls).toContainEqual({ command: "news_list", payload: { symbol: "600000.SH", limit: 20 } });

  expect(screen.getByText("输出预览")).toBeInTheDocument();
  expect(screen.getByText("分析 浦发银行")).toBeInTheDocument();
  expect(screen.queryByText("暂无输出内容")).not.toBeInTheDocument();
  fireEvent.mouseDown(screen.getByText("Markdown"));
  fireEvent.click(screen.getAllByRole("option", { name: "纯文本" })[0]);
  expect(screen.getByText("分析 浦发银行")).toBeInTheDocument();

  const stopButton = screen.getByRole("button", { name: /停止生成/ });
  expect(stopButton).toBeDisabled();
  expect(screen.getByRole("button", { name: /保存报告/ })).toBeDisabled();
  expect(screen.getByRole("button", { name: /复制 Markdown/ })).toBeEnabled();
  expect(screen.getByRole("button", { name: /导出 Markdown/ })).toBeEnabled();
  expect(writeText).not.toHaveBeenCalled();
  fireEvent.click(screen.getByLabelText("全屏预览"));
  const fullscreenDialog = screen.getAllByRole("dialog").find((dialog) => within(dialog).queryByRole("heading", { name: "全屏预览" }));
  if (!fullscreenDialog) {
    throw new Error("fullscreen preview dialog should open");
  }
  expect(within(fullscreenDialog).getByText("分析 浦发银行")).toBeInTheDocument();

  expect(screen.getByText("AI 输出需区分事实、推断和观点，仅供研究参考。")).toBeInTheDocument();
  expect(screen.getAllByText("仅供研究，不构成投资建议。").length).toBeGreaterThan(0);

  fireEvent.click(screen.getByRole("button", { name: /开始分析/ }));
  await waitFor(() => {
    expect(window.location.hash).toBe("#/analysis/running?taskId=analysis-600000");
    expect(screen.getByRole("heading", { name: "AI 分析任务" })).toBeInTheDocument();
  });
  expect(calls).toContainEqual({
    command: "analysis_task_create",
    payload: {
      payload: {
        symbol: "600000.SH",
        analysis_type: "stock_full",
        ai_config_id: 1,
        api_key_ref: "local-vault://ai-config/deepseek-1",
        prompt_template_id: 11,
        user_position: null,
      },
    },
  });

  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("自动交易")).not.toBeInTheDocument();
  expect(calls.map((call) => call.command)).not.toContain("search_global");
});

test("AI 分析页未配置模型时禁用任务创建", async () => {
  window.location.hash = "#/analysis";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "prompt_templates_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 11,
                key: "stock_full_v1",
                name: "个股综合模板",
                type: "stock_full",
                description: "综合分析",
                content: "分析 {{stock_name}}",
                variables: ["stock_name"],
                is_builtin: true,
                builtin_locked: true,
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
    expect(screen.getByRole("button", { name: /开始分析/ })).toBeDisabled();
  });
  fireEvent.click(screen.getByRole("button", { name: /开始分析/ }));
  expect(calls.map((call) => call.command)).not.toContain("analysis_task_create");
});

test("AI 分析运行页恢复任务事件、订阅增量事件并支持取消", async () => {
  window.location.hash = "#/analysis/running?taskId=analysis-1";
  const calls: Array<{ command: string; payload?: any }> = [];
  const writeText = vi.fn<(text: string) => Promise<void>>(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      case "task_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: "analysis-1",
            type: "ANALYSIS",
            status: "RUNNING",
            title: "600000.SH technical",
            progress: 35,
            started_at: "2026-06-24T12:00:00Z",
            created_at: "2026-06-24T12:00:00Z",
          },
        };
      case "task_events":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 1, event_type: "TASK_STARTED", payload: "{\"progress\":5}", created_at: "2026-06-24T12:00:01Z" },
              { id: 2, event_type: "TASK_PROGRESS", payload: "{\"progress\":35,\"message\":\"正在读取行情上下文\"}", created_at: "2026-06-24T12:00:02Z" },
              { id: 3, event_type: "TASK_LOG", payload: "{\"message\":\"K 线数据已加载\"}", created_at: "2026-06-24T12:00:03Z" },
              { id: 4, event_type: "TASK_CHUNK", payload: "{\"content\":\"## 技术结论\\n- 趋势偏强\"}", created_at: "2026-06-24T12:00:04Z" },
            ],
          },
        };
      case "analysis_task_subscribe":
        return { emitted: 0, last_event_id: 4 };
      case "analysis_task_cancel":
        return { code: 0, message: "ok", data: { task_id: "analysis-1", status: "CANCELLED" } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "600000.SH technical" })).toBeInTheDocument();
  });
  expect(screen.getByRole("link", { name: "AI 分析" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByText("AI 分析任务进行中，请稍候...")).toBeInTheDocument();
  expect(screen.getByText("RUNNING")).toBeInTheDocument();
  expect(screen.getByText("analysis-1")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /返回/ })).toBeInTheDocument();
  expect(calls).toContainEqual({ command: "task_get", payload: { taskId: "analysis-1" } });
  expect(calls).toContainEqual({ command: "task_events", payload: { taskId: "analysis-1", afterEventId: 0 } });
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "analysis_task_subscribe", payload: { taskId: "analysis-1", afterEventId: 4 } });
  });
  expect(eventListenMock).toHaveBeenCalledWith("analysis-task-event", expect.any(Function));

  expect(screen.getByText("任务步骤")).toBeInTheDocument();
  expect(screen.getByText("任务已开始")).toBeInTheDocument();
  expect(screen.getAllByText("正在读取行情上下文").length).toBeGreaterThan(0);

  expect(screen.getByText("流式输出")).toBeInTheDocument();
  expect(screen.getByText("自动滚动")).toBeInTheDocument();
  expect(screen.getByText("技术结论")).toBeInTheDocument();
  expect(screen.getByText("趋势偏强")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /复制当前内容/ }));
  expect(writeText).toHaveBeenCalledWith("## 技术结论\n- 趋势偏强");

  expect(screen.getByText("任务日志")).toBeInTheDocument();
  expect(screen.getByText("TASK_STARTED")).toBeInTheDocument();
  expect(screen.getByText("TASK_PROGRESS")).toBeInTheDocument();
  expect(screen.getByText("TASK_LOG")).toBeInTheDocument();
  expect(screen.getByText("TASK_CHUNK")).toBeInTheDocument();
  expect(screen.getAllByText("K 线数据已加载").length).toBeGreaterThan(0);

  const autoScrollSwitch = screen.getByRole("switch");
  expect(autoScrollSwitch).toBeChecked();
  fireEvent.click(autoScrollSwitch);
  expect(autoScrollSwitch).not.toBeChecked();
  fireEvent.click(screen.getByRole("button", { name: /^清\s*空$/ }));
  fireEvent.click(screen.getByRole("button", { name: "清空日志" }));

  const stopButton = screen.getByRole("button", { name: /停止生成/ });
  expect(stopButton).toBeEnabled();
  fireEvent.click(stopButton);
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "analysis_task_cancel", payload: { taskId: "analysis-1" } });
  });
  expect(screen.getByText("CANCELLED")).toBeInTheDocument();
  expect(stopButton).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: /后台运行/ }));

  expect(screen.getByText("任务完成后可在报告历史中查看完整内容。")).toBeInTheDocument();
  expect(screen.getByText("AI 输出需区分事实、推断和观点，仅供研究参考，不构成投资建议。")).toBeInTheDocument();
  expect(screen.getAllByText("仅供研究，不构成投资建议。").length).toBeGreaterThan(0);
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("自动交易")).not.toBeInTheDocument();
});

test("AI 分析运行页忽略非当前任务的实时事件", async () => {
  window.location.hash = "#/analysis/running?taskId=analysis-current";
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      case "task_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: "analysis-current",
            type: "ANALYSIS",
            status: "RUNNING",
            title: "600000.SH technical",
            progress: 20,
            started_at: "2026-06-24T12:00:00Z",
            created_at: "2026-06-24T12:00:00Z",
          },
        };
      case "task_events":
        return { code: 0, message: "ok", data: { items: [] } };
      case "analysis_task_subscribe":
        return { emitted: 0, last_event_id: 0 };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(eventListenMock).toHaveBeenCalledWith("analysis-task-event", expect.any(Function));
  });
  const eventHandler = eventListenMock.mock.calls[0][1] as (event: { payload: unknown }) => void;

  await act(async () => {
    eventHandler({
      payload: {
        id: 1,
        task_id: "analysis-old",
        event: "TASK_CHUNK",
        data: { content: "旧任务输出不应出现" },
        created_at: "2026-06-24T12:00:01Z",
      },
    });
  });

  expect(screen.queryByText("旧任务输出不应出现")).not.toBeInTheDocument();

  await act(async () => {
    eventHandler({
      payload: {
        id: 2,
        task_id: "analysis-current",
        event: "TASK_CHUNK",
        data: { content: "当前任务输出" },
        created_at: "2026-06-24T12:00:02Z",
      },
    });
  });

  expect(screen.getByText("当前任务输出")).toBeInTheDocument();
});

test("AI 分析运行页成功任务可跳转已生成报告", async () => {
  window.location.hash = "#/analysis/running?taskId=analysis-success";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "providers_status":
        return { code: 0, message: "ok", data: { items: [] } };
      case "notifications_unread_count":
        return { code: 0, message: "ok", data: { count: 0 } };
      case "task_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: "analysis-success",
            type: "ANALYSIS",
            status: "SUCCESS",
            title: "600000.SH technical",
            progress: 100,
            report_id: 88,
            started_at: "2026-06-24T12:00:00Z",
            finished_at: "2026-06-24T12:03:00Z",
            created_at: "2026-06-24T12:00:00Z",
          },
        };
      case "task_events":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { id: 1, event_type: "TASK_STARTED", payload: "{\"progress\":5}", created_at: "2026-06-24T12:00:01Z" },
              { id: 2, event_type: "TASK_CHUNK", payload: "{\"content\":\"## 技术结论\\n- 趋势偏强\"}", created_at: "2026-06-24T12:00:04Z" },
              { id: 3, event_type: "TASK_SUCCESS", payload: "{\"progress\":100}", created_at: "2026-06-24T12:03:00Z" },
            ],
          },
        };
      case "report_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 88,
            task_id: "analysis-success",
            stock_code: "600000.SH",
            stock_name: "浦发银行",
            analysis_type: "technical",
            title: "600000.SH technical",
            content_md: "## 技术结论\n- 趋势偏强",
            model_provider: "openai",
            model_name: "gpt-4.1-mini",
            created_at: "2026-06-24T12:03:00Z",
            updated_at: "2026-06-24T12:03:00Z",
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "600000.SH technical" })).toBeInTheDocument();
  });
  expect(calls).toContainEqual({ command: "task_get", payload: { taskId: "analysis-success" } });
  expect(calls).toContainEqual({ command: "task_events", payload: { taskId: "analysis-success", afterEventId: 0 } });
  expect(calls.some((call) => call.command === "analysis_task_subscribe")).toBe(false);
  expect(screen.getByText("SUCCESS")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /查看报告/ }));

  await waitFor(() => {
    expect(window.location.hash).toBe("#/reports/88");
  });
});

test("分析报告历史页面展示空态并按报告范围搜索", async () => {
  window.location.hash = "#/reports";
  const calls: Array<{ command: string; payload?: any }> = [];
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
            doc_uid: "report:2",
            doc_type: "report",
            ref_id: "2",
            symbol: "CN:SZ:002409",
            title: "雅克科技 技术面分析",
            summary: "趋势反转失败；量能不足；关键位失守",
            source: "analysis_report",
            source_time: "2025-05-20T14:42:00Z",
            score: 1,
            highlights: ["雅克科技"],
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

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "分析报告历史" })).toBeInTheDocument();
  });

  expect(screen.getByText("报告列表")).toBeInTheDocument();
  expect(screen.queryByText("生益科技 个股综合分析")).not.toBeInTheDocument();
  expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
  expect(screen.getByText("报告统计")).toBeInTheDocument();
  expect(screen.getByText("常用模型 TOP 5")).toBeInTheDocument();
  expect(screen.getByText("分析类型分布")).toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /新建分析/ }));
  await waitFor(() => {
    expect(window.location.hash).toBe("#/analysis");
  });
  window.location.hash = "#/reports";
  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "分析报告历史" })).toBeInTheDocument();
  });

  fireEvent.change(screen.getByPlaceholderText("输入股票名称 / 代码 / 拼音"), { target: { value: "雅克" } });
  fireEvent.click(screen.getByLabelText("查询报告"));
  await waitFor(() => {
    expect(screen.getByText("雅克科技 技术面分析")).toBeInTheDocument();
  });
  expect(screen.getByText("趋势反转失败；量能不足；关键位失守")).toBeInTheDocument();
  expect(screen.queryByText("生益科技 个股综合分析")).not.toBeInTheDocument();
  fireEvent.click(screen.getByLabelText("删除 雅克科技 技术面分析"));
  await waitFor(() => {
    expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
  });

  fireEvent.click(screen.getByLabelText("重置筛选"));
  await waitFor(() => {
    expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();
  });

  expect(screen.queryByRole("button", { name: "复制 生益科技 个股综合分析" })).not.toBeInTheDocument();
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([
    { command: "report_list", payload: {} },
    { command: "report_stats", payload: {} },
    {
      command: "search_reports",
      payload: { payload: { keyword: "雅克", symbols: [], limit: 20, offset: 0, sort: "relevance" } },
    },
    { command: "report_delete", payload: { id: 2 } },
    { command: "report_stats", payload: {} },
    { command: "report_list", payload: {} },
    { command: "report_stats", payload: {} },
  ]);
});

test("资讯中心页面加载真实市场新闻并按资讯范围搜索", async () => {
  window.location.hash = "#/news";
  const writeText = vi.fn(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "news_market") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: 31,
              source: "财联社",
              title: "A 股午后震荡，AI 算力方向活跃",
              url: "https://news.example.com/market-ai",
              summary: "市场新闻来自本地缓存或真实 Provider 回源。",
              published_at: "2026-06-24T13:30:00Z",
              symbols: ["CN:SH:000001"],
              tags: ["市场", "AI 算力"],
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
          total_count: 2,
          source_count: 2,
          latest_published_at: "2026-06-24T13:30:00Z",
          sentiment_summary: "暂未接入情绪分类，当前仅展示新闻缓存数量、来源和标签统计。",
        },
      };
    }
    if (command === "news_hot_topics") {
      return {
        code: 0,
        message: "ok",
        data: {
          industries: [{ name: "AI 算力", count: 2 }],
          mentioned_stocks: [{ symbol: "CN:SH:000001", count: 2 }],
          updated_at: "2026-06-24T13:30:00Z",
        },
      };
    }
    if (command === "search_news") {
      return {
        code: 0,
        message: "ok",
        data: [
          {
            doc_uid: "news:optical",
            doc_type: "news",
            ref_id: "https://news.example.com/optical",
            symbol: "CN:SZ:300308",
            title: "光模块厂商订单增长，AI 算力需求延续",
            summary: "机构认为北美 AI 算力资本开支仍处高位。",
            source: "同花顺资讯",
            source_time: "2026-06-22T10:22:00Z",
            score: 1,
            highlights: ["光模块"],
          },
        ],
      };
    }
    if (command === "open_external_url") {
      return { opened: true };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "资讯中心" })).toBeInTheDocument();
  });

  expect(screen.getByText("聚合个股新闻、市场新闻、行业事件与研究线索")).toBeInTheDocument();
  expect(screen.getByText("资讯列表")).toBeInTheDocument();
  await waitFor(() => {
    expect(screen.getByText("A 股午后震荡，AI 算力方向活跃")).toBeInTheDocument();
  });
  expect(screen.getByText("（共 1 条）")).toBeInTheDocument();
  expect(screen.getByText("市场新闻来自本地缓存或真实 Provider 回源。")).toBeInTheDocument();
  expect(screen.getByText("热点观察")).toBeInTheDocument();
  expect(screen.getAllByText("AI 算力").length).toBeGreaterThanOrEqual(2);
  expect(screen.getAllByText("CN:SH:000001").length).toBeGreaterThanOrEqual(2);
  expect(screen.getByText("暂未接入情绪分类，当前仅展示新闻缓存数量、来源和标签统计。")).toBeInTheDocument();
  expect(screen.getByText("数据源状态")).toBeInTheDocument();
  expect(screen.queryByText("15:30 更新")).not.toBeInTheDocument();
  expect(screen.queryByText("缓存总量：312 MB")).not.toBeInTheDocument();
  expect(screen.queryByText("生益科技：公司高端覆铜板产品订单饱满，持续提升 AI 服务器领域份额")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("公告专用入口")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /加入上下文/ })).not.toBeInTheDocument();

  fireEvent.change(screen.getByPlaceholderText("输入关键词，支持标题/摘要"), { target: { value: "光模块" } });
  fireEvent.click(screen.getByRole("button", { name: /刷新资讯/ }));
  await waitFor(() => {
    expect(screen.getByText("光模块厂商订单增长，AI 算力需求延续")).toBeInTheDocument();
  });
  fireEvent.click(screen.getAllByRole("button", { name: /查看原文/ })[0]);
  fireEvent.click(screen.getAllByRole("button", { name: /复制摘要/ })[0]);
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("北美 AI 算力"));
  });

  await waitFor(() => {
    expect(calls).toContainEqual({ command: "open_external_url", payload: { url: "https://news.example.com/optical" } });
  });
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([
    { command: "news_market", payload: { market: "CN", limit: 20 } },
    { command: "news_stats", payload: { market: "CN", limit: 20 } },
    { command: "news_hot_topics", payload: { market: "CN", limit: 20 } },
    {
      command: "search_news",
      payload: { payload: { keyword: "光模块", symbols: [], limit: 20, offset: 0, sort: "relevance" } },
    },
    { command: "open_external_url", payload: { url: "https://news.example.com/optical" } },
  ]);
});

test("分析报告详情页面无报告时展示空态并可返回列表", async () => {
  window.location.hash = "#/reports/report-1";
  const writeText = vi.fn(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  const calls: Array<{ command: string; payload?: any }> = [];
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
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "报告详情" })).toBeInTheDocument();
  });

  expect(screen.getByText("暂无报告详情，请从报告历史选择已生成的报告")).toBeInTheDocument();
  expect(screen.queryByText("核心结论")).not.toBeInTheDocument();
  expect(screen.getByText("风险声明")).toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();

  expect(writeText).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: /返回列表/ }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "分析报告历史" })).toBeInTheDocument();
  });

  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([
    { command: "report_list", payload: {} },
    { command: "report_stats", payload: {} },
  ]);
});

test("分析报告详情页面按路由 ID 读取真实报告", async () => {
  window.location.hash = "#/reports/3";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "report_get") {
      return {
        code: 0,
        message: "ok",
        data: {
          id: 3,
          task_id: "task-report-3",
          symbol: "CN:SH:600519",
          title: "贵州茅台 个股综合分析",
          analysis_type: "stock_full",
          model_name: "gpt-4.1-mini",
          content_markdown: "## 核心结论\n\n仅作研究辅助，不构成投资建议。",
          risk_summary: "估值波动风险",
          favorite: false,
          created_at: "2026-06-22T09:00:00Z",
          updated_at: "2026-06-22T10:00:00Z",
        },
      };
    }
    if (command === "report_update") {
      return {
        code: 0,
        message: "ok",
        data: { id: 3, task_id: "task-report-3", symbol: "CN:SH:600519", title: "贵州茅台 个股综合分析", favorite: true },
      };
    }
    if (command === "report_export") {
      return { saved: true, file_path: "/tmp/贵州茅台.md", file_name: "贵州茅台.md" };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "贵州茅台 个股综合分析" })).toBeInTheDocument();
  });
  expect(screen.getAllByText(/核心结论/).length).toBeGreaterThan(0);
  expect(screen.getByText(/仅作研究辅助，不构成投资建议/)).toBeInTheDocument();
  expect(screen.getByText(/估值波动风险/)).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "复制输入快照" })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "折叠目录" }));
  expect(screen.queryByRole("button", { name: /1\\. 核心结论/ })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "展开目录" })).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "收藏报告" }));
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "report_update", payload: { id: 3, favorite: true } });
  });
  fireEvent.click(screen.getByRole("button", { name: /导出 Markdown/ }));
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "report_export", payload: { id: 3 } });
  });
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([
    { command: "report_get", payload: { id: 3 } },
    { command: "report_update", payload: { id: 3, favorite: true } },
    { command: "report_export", payload: { id: 3 } },
  ]);
});

test("分析报告详情页面复制 Markdown 失败时展示错误", async () => {
  window.location.hash = "#/reports/3";
  const writeText = vi.fn(() => Promise.reject(new Error("clipboard denied")));
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  mockIPC((command) => {
    if (command === "report_get") {
      return {
        code: 0,
        message: "ok",
        data: {
          id: 3,
          task_id: "task-report-3",
          symbol: "CN:SH:600519",
          title: "贵州茅台 个股综合分析",
          analysis_type: "stock_full",
          model_name: "gpt-4.1-mini",
          content_markdown: "## 核心结论\n\n仅供研究参考。",
          risk_summary: "估值波动风险",
          created_at: "2026-06-23T10:00:00Z",
          updated_at: "2026-06-23T10:00:00Z",
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "贵州茅台 个股综合分析" })).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /复制 Markdown/ }));

  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith("## 核心结论\n\n仅供研究参考。");
    expect(screen.getByText("Markdown 复制失败")).toBeInTheDocument();
  });
});

test("分析报告详情页面重新分析使用报告来源创建新任务", async () => {
  window.location.hash = "#/reports/3";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "report_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 3,
            task_id: "task-report-3",
            symbol: "CN:SH:600519",
            title: "贵州茅台 技术面分析",
            analysis_type: "technical",
            model_name: "DeepSeek-V3",
            prompt_template_id: 7,
            content_markdown: "## 技术面结论\n\n仅作研究辅助，不构成投资建议。",
            risk_summary: "技术形态波动",
            created_at: "2026-06-22T09:00:00Z",
            updated_at: "2026-06-22T10:00:00Z",
          },
        };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
      case "analysis_task_create":
        return { code: 0, message: "ok", data: { task_id: "reanalyze-1", status: "PENDING" } };
      case "task_get":
        return { code: 0, message: "ok", data: { id: "reanalyze-1", type: "ANALYSIS", status: "PENDING", title: "重新分析", progress: 0 } };
      case "task_events":
        return { code: 0, message: "ok", data: { items: [] } };
      case "analysis_task_subscribe":
        return { code: 0, message: "ok", data: { emitted: 0, last_event_id: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "贵州茅台 技术面分析" })).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /重新分析/ }));

  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "analysis_task_create",
      payload: {
        payload: {
          symbol: "CN:SH:600519",
          analysis_type: "technical",
          ai_config_id: 1,
          api_key_ref: "local-vault://ai-config/deepseek-1",
          prompt_template_id: 7,
          user_position: null,
        },
      },
    });
  });
  expect(window.location.hash).toBe("#/analysis/running?taskId=reanalyze-1");
});

test("分析报告详情页面不会把不支持的报告类型重新分析为综合分析", async () => {
  window.location.hash = "#/reports/3";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "report_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 3,
            task_id: "task-report-3",
            symbol: "CN:SH:600519",
            title: "贵州茅台 自定义分析",
            analysis_type: "custom",
            model_name: "DeepSeek-V3",
            prompt_template_id: 7,
            content_markdown: "## 自定义结论\n\n仅作研究辅助，不构成投资建议。",
            risk_summary: "自定义模板风险",
            created_at: "2026-06-22T09:00:00Z",
            updated_at: "2026-06-22T10:00:00Z",
          },
        };
      case "ai_config_list":
        return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
      case "analysis_task_create":
        return { code: 0, message: "ok", data: { task_id: "reanalyze-custom", status: "PENDING" } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "贵州茅台 自定义分析" })).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /重新分析/ }));
  await act(async () => {
    await Promise.resolve();
  });

  expect(calls.map((call) => call.command)).not.toContain("ai_config_list");
  expect(calls.map((call) => call.command)).not.toContain("analysis_task_create");
  expect(window.location.hash).toBe("#/reports/3");
});

test("分析报告详情页面删除报告后返回真实报告列表", async () => {
  window.location.hash = "#/reports/3";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    switch (command) {
      case "report_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 3,
            task_id: "task-report-3",
            symbol: "CN:SH:600519",
            title: "贵州茅台 个股综合分析",
            analysis_type: "stock_full",
            model_name: "DeepSeek-V3",
            prompt_template_id: 7,
            content_markdown: "## 核心结论\n\n仅作研究辅助，不构成投资建议。",
            risk_summary: "估值波动风险",
            created_at: "2026-06-22T09:00:00Z",
            updated_at: "2026-06-22T10:00:00Z",
          },
        };
      case "report_delete":
        return { code: 0, message: "ok", data: { id: 3 } };
      case "report_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "report_stats":
        return { code: 0, message: "ok", data: { total: 0, unique_symbols: 0, analysis_types: [], top_models: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "贵州茅台 个股综合分析" })).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /删除/ }));

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "分析报告历史" })).toBeInTheDocument();
  });
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([
    { command: "report_get", payload: { id: 3 } },
    { command: "report_delete", payload: { id: 3 } },
    { command: "report_list", payload: {} },
    { command: "report_stats", payload: {} },
  ]);
});

test("任务历史页面读取真实任务列表、事件详情并支持取消", async () => {
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
            items: [
              {
                id: "analysis-600000",
                type: "ANALYSIS",
                status: "RUNNING",
                title: "浦发银行 个股综合分析",
                progress: 35,
                started_at: "2026-06-24T12:00:00Z",
                created_at: "2026-06-24T12:00:00Z",
              },
            ],
          },
        };
      case "task_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: "analysis-600000",
            type: "ANALYSIS",
            status: "RUNNING",
            title: "浦发银行 个股综合分析",
            progress: 45,
            started_at: "2026-06-24T12:00:00Z",
            created_at: "2026-06-24T12:00:00Z",
          },
        };
      case "task_events":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                event_type: "TASK_STARTED",
                payload: "{\"message\":\"任务已开始\"}",
                created_at: "2026-06-24T12:00:01Z",
              },
              {
                id: 2,
                event_type: "TASK_PROGRESS",
                payload: "{\"message\":\"正在读取行情上下文\"}",
                created_at: "2026-06-24T12:00:02Z",
              },
            ],
          },
        };
      case "analysis_task_cancel":
        return { code: 0, message: "ok", data: { task_id: "analysis-600000", status: "CANCELLED" } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务历史" })).toBeInTheDocument();
  });
  expect(screen.getByText("跟踪 AI 分析、行情刷新、资讯同步、缓存清理和数据重建任务")).toBeInTheDocument();
  expect(screen.getByText("运行中任务")).toBeInTheDocument();
  expect(screen.getByText("今日成功")).toBeInTheDocument();
  expect(screen.getByText("失败任务")).toBeInTheDocument();
  await waitFor(() => {
    expect(screen.getByText("analysis-600000")).toBeInTheDocument();
  });
  expect(screen.getAllByText("浦发银行 个股综合分析").length).toBeGreaterThan(0);
  fireEvent.click(screen.getAllByText("浦发银行 个股综合分析")[0]);
  await waitFor(() => {
    expect(screen.getByText("事件流")).toBeInTheDocument();
    expect(screen.getByText("任务已开始")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /取消/ }));
  await waitFor(() => {
    expect(screen.getAllByText("已取消").length).toBeGreaterThan(0);
  });
  expect(screen.queryByText("task_20250520_152834_abcd1234")).not.toBeInTheDocument();
  expect(screen.queryByText("生益科技 个股综合分析")).not.toBeInTheDocument();
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([
    { command: "task_list", payload: { limit: 100 } },
    { command: "task_get", payload: { taskId: "analysis-600000" } },
    { command: "task_events", payload: { taskId: "analysis-600000", afterEventId: 0 } },
    { command: "analysis_task_cancel", payload: { taskId: "analysis-600000" } },
  ]);
});

test("任务历史页面时间范围筛选使用后端任务日期而不是展示文本", async () => {
  window.location.hash = "#/tasks";
  mockIPC((command) => {
    if (command === "task_list") {
      return {
        code: 0,
        message: "ok",
        data: {
          items: [
            {
              id: "analysis-600519",
              type: "ANALYSIS",
              status: "SUCCESS",
              title: "贵州茅台 个股综合分析",
              progress: 100,
              started_at: "2026-06-23T09:00:00Z",
              finished_at: "2026-06-23T09:03:00Z",
              created_at: "2026-06-23T09:00:00Z",
            },
            {
              id: "analysis-600000",
              type: "ANALYSIS",
              status: "SUCCESS",
              title: "浦发银行 个股综合分析",
              progress: 100,
              started_at: "2026-06-24T12:00:00Z",
              finished_at: "2026-06-24T12:04:00Z",
              created_at: "2026-06-24T12:00:00Z",
            },
          ],
        },
      };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("贵州茅台 个股综合分析")).toBeInTheDocument();
    expect(screen.getByText("浦发银行 个股综合分析")).toBeInTheDocument();
  });

  fireEvent.input(screen.getByPlaceholderText("开始日期"), { target: { value: "2026-06-24" } });
  fireEvent.input(screen.getByPlaceholderText("结束日期"), { target: { value: "2026-06-24" } });
  fireEvent.click(screen.getByRole("button", { name: /查\s*询/ }));

  expect(screen.queryByText("贵州茅台 个股综合分析")).not.toBeInTheDocument();
  expect(screen.getByText("浦发银行 个股综合分析")).toBeInTheDocument();
});

test("任务历史成功分析任务可跳转已生成报告", async () => {
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
            items: [
              {
                id: "analysis-success",
                type: "ANALYSIS",
                status: "SUCCESS",
                title: "贵州茅台 技术面分析",
                progress: 100,
                report_id: 88,
                started_at: "2026-06-24T12:00:00Z",
                finished_at: "2026-06-24T12:03:00Z",
                created_at: "2026-06-24T12:00:00Z",
              },
            ],
          },
        };
      case "report_get":
        return {
          code: 0,
          message: "ok",
          data: {
            id: 88,
            task_id: "analysis-success",
            symbol: "CN:SH:600519",
            title: "贵州茅台 技术面分析",
            analysis_type: "technical",
            model_name: "DeepSeek-V3",
            content_markdown: "## 技术结论\n\n仅作研究辅助，不构成投资建议。",
            risk_summary: "技术形态波动",
            created_at: "2026-06-24T12:03:00Z",
            updated_at: "2026-06-24T12:03:00Z",
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("analysis-success")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /报告/ }));
  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "贵州茅台 技术面分析" })).toBeInTheDocument();
  });
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([
    { command: "task_list", payload: { limit: 100 } },
    { command: "report_get", payload: { id: 88 } },
  ]);
});

test("任务历史搜索索引重建任务展示为数据重建且不显示报告入口", async () => {
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
            items: [
              {
                id: "search-rebuild-1782221382580223000",
                type: "SEARCH_REBUILD",
                status: "SUCCESS",
                title: "搜索索引重建",
                progress: 100,
                started_at: "2026-06-23T21:29:42Z",
                finished_at: "2026-06-23T21:29:45Z",
                created_at: "2026-06-23T21:29:42Z",
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
    expect(screen.getByText("search-rebuild-1782221382580223000")).toBeInTheDocument();
  });
  const taskTable = screen.getByText("任务列表").closest("section");
  expect(taskTable).not.toBeNull();
  const tableScope = within(taskTable as HTMLElement);
  expect(tableScope.getByText("数据重建")).toBeInTheDocument();
  expect(tableScope.queryByText("AI 分析")).not.toBeInTheDocument();
  expect(tableScope.getByRole("button", { name: /日志/ })).toBeInTheDocument();
  expect(tableScope.queryByRole("button", { name: /报告/ })).not.toBeInTheDocument();
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([{ command: "task_list", payload: { limit: 100 } }]);
});

test("任务历史失败分析任务可按原创建事件生成重试任务", async () => {
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
            items: [
              {
                id: "analysis-failed",
                type: "ANALYSIS",
                status: "FAILED",
                title: "浦发银行 个股综合分析",
                progress: 80,
                error_message: "provider error",
                started_at: "2026-06-24T12:00:00Z",
                finished_at: "2026-06-24T12:01:00Z",
                created_at: "2026-06-24T12:00:00Z",
              },
            ],
          },
        };
      case "task_events":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                event_type: "TASK_CREATED",
                payload: JSON.stringify({
                  symbol: "CN:SH:600000",
                  analysis_type: "stock_full",
                  ai_config_id: 7,
                  prompt_template_id: 9,
                  has_user_position: false,
                }),
                created_at: "2026-06-24T12:00:00Z",
              },
            ],
          },
        };
      case "ai_config_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [{
              id: 7,
              name: "DeepSeek",
              provider: "openai-compatible",
              base_url: "https://api.example.com/v1",
              api_key_ref: "local-vault://ai-config/deepseek-7",
              masked_api_key: "sk-***",
              has_api_key: true,
              model_name: "deepseek-chat",
              temperature: 0.2,
              max_tokens: 4096,
              timeout_seconds: 60,
              stream_enabled: true,
              is_default: true,
            }],
          },
        };
      case "analysis_task_create":
        return { code: 0, message: "ok", data: { task_id: "analysis-retry", status: "PENDING" } };
      case "task_get":
        return { code: 0, message: "ok", data: { id: "analysis-retry", type: "ANALYSIS", status: "PENDING", title: "重试任务", progress: 0 } };
      case "analysis_task_subscribe":
        return { code: 0, message: "ok", data: { emitted: 0, last_event_id: 0 } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("analysis-failed")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: /重试/ }));
  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "analysis_task_create",
      payload: {
        payload: {
          symbol: "CN:SH:600000",
          analysis_type: "stock_full",
          ai_config_id: 7,
          api_key_ref: "local-vault://ai-config/deepseek-7",
          prompt_template_id: 9,
          retry_of_task_id: "analysis-failed",
          user_position: null,
        },
      },
    });
  });
});

test("TopBar 运行期新增未读通知时按设置触发系统通知", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  notificationIsPermissionGrantedMock.mockResolvedValue(true);
  let unreadCountCalls = 0;
  mockIPC((command, payload) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: emptyDashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change: 0, change_percent: 0 } };
      case "notifications_unread_count":
        unreadCountCalls += 1;
        return { code: 0, message: "ok", data: { count: unreadCountCalls === 1 ? 0 : 1 } };
      case "notifications_list":
        expect(payload).toEqual({ payload: { unread_only: true, limit: 5, offset: 0 } });
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 11,
                type: "task_success",
                level: "success",
                title: "任务完成",
                content: "浦发银行分析已完成",
                route: "/reports/3",
                is_read: false,
                created_at: "2026-06-23T09:00:00Z",
              },
            ],
            total: 1,
            limit: 5,
            offset: 0,
          },
        };
      case "settings_get":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { key: "notifications.system_enabled", value: "true" },
              { key: "notifications.task_success", value: "true" },
              { key: "notifications.task_failed", value: "true" },
              { key: "notifications.provider_error", value: "true" },
            ],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  await waitFor(() => {
    expect(unreadCountCalls).toBe(1);
  });
  expect(notificationSendMock).not.toHaveBeenCalled();

  await vi.advanceTimersByTimeAsync(30_000);

  await waitFor(() => {
    expect(notificationSendMock).toHaveBeenCalledWith({
      title: "任务完成",
      body: "浦发银行分析已完成",
    });
  });
});

test("TopBar 系统通知关闭时不触发系统通知", async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  notificationIsPermissionGrantedMock.mockResolvedValue(true);
  let unreadCountCalls = 0;
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "dashboard_summary":
        return { code: 0, message: "ok", data: emptyDashboardFixture };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 0, message: "ok", data: { symbol: "000001.SH", price: 0, change: 0, change_percent: 0 } };
      case "notifications_unread_count":
        unreadCountCalls += 1;
        return { code: 0, message: "ok", data: { count: unreadCountCalls === 1 ? 0 : 1 } };
      case "notifications_list":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 12,
                type: "provider_error",
                level: "warning",
                title: "数据源异常",
                content: "行情 Provider 不可用",
                route: "/settings",
                is_read: false,
                created_at: "2026-06-23T09:00:00Z",
              },
            ],
            total: 1,
            limit: 5,
            offset: 0,
          },
        };
      case "settings_get":
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              { key: "notifications.system_enabled", value: "false" },
              { key: "notifications.task_success", value: "true" },
              { key: "notifications.task_failed", value: "true" },
              { key: "notifications.provider_error", value: "true" },
            ],
          },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  render(<App />);
  await waitFor(() => {
    expect(unreadCountCalls).toBe(1);
  });

  await vi.advanceTimersByTimeAsync(30_000);

  await waitFor(() => {
    expect(unreadCountCalls).toBeGreaterThan(1);
  });
  expect(notificationSendMock).not.toHaveBeenCalled();
  expect(notificationIsPermissionGrantedMock).not.toHaveBeenCalled();
});

test("任务历史页面筛选和重置保持空态", async () => {
  window.location.hash = "#/tasks";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "task_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByRole("heading", { name: "任务历史" })).toBeInTheDocument();
  });

  fireEvent.change(screen.getByPlaceholderText("搜索任务标题 / 任务 ID / 股票代码"), {
    target: { value: "雅克科技" },
  });
  fireEvent.click(screen.getByRole("button", { name: /查\s*询/ }));
  expect(screen.queryByText("雅克科技 技术面分析")).not.toBeInTheDocument();

  expect(appDatePickerLocale.lang.shortWeekDays).toEqual(["日", "一", "二", "三", "四", "五", "六"]);
  expect(appDatePickerLocale.lang.shortMonths).toContain("6月");

  fireEvent.click(screen.getByRole("button", { name: /重\s*置/ }));
  expect(screen.queryByText("生益科技 个股综合分析")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /取消/ })).not.toBeInTheDocument();
  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual([{ command: "task_list", payload: { limit: 100 } }]);
});

test("设置中心基础设置页展示真实空态并支持基础交互", async () => {
  window.location.hash = "#/settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  const writeText = vi.fn<(text: string) => Promise<void>>(() => Promise.resolve());
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  dialogOpenMock.mockResolvedValueOnce(null).mockResolvedValueOnce("/tmp/invest-compass-logs");
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "core_health") {
      return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
    }
    if (command === "providers_status") {
      return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
    }
    if (command === "settings_get") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
    }
    if (command === "prompt_templates_list") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "prompt_templates_create") {
      return {
        code: 0,
        message: "ok",
        data: {
          id: 11,
          ...(payload as any).payload,
          variables: ["stock_code"],
          is_builtin: false,
        },
      };
    }
    if (command === "cache_stats") {
      return {
        code: 0,
        message: "ok",
        data: {
          total_bytes: 1_572_864,
          items: [
            { target: "quote", bytes: 1_048_576, label: "行情缓存", cleanable: true },
            { target: "task_logs", bytes: 524_288, label: "任务日志", cleanable: true },
          ],
        },
      };
    }
    if (command === "scheduler_status") {
      return {
        code: 0,
        message: "ok",
        data: { jobs_total: 5, jobs_enabled: 3, queued_runs: 1, running_runs: 0, failed_runs: 0 },
      };
    }
    if (command === "cache_clean") {
      const cleanPayload = payload as { payload?: { targets?: string[] } } | undefined;
      return { code: 0, message: "ok", data: { cleaned_targets: cleanPayload?.payload?.targets ?? [] } };
    }
    if (command === "proxy_connection_test") {
      return {
        code: 0,
        message: "ok",
        data: {
          result: {
            ok: true,
            target: "baidu",
            status_code: 200,
            duration_ms: 128,
            checked_at: "2026-06-24T12:00:00Z",
            message: "ok",
          },
        },
      };
    }
    if (command === "search_status") {
      return {
        code: 0,
        message: "ok",
        data: {
          fts5_status: "available",
          gse_status: "fallback",
          search_status: "ready",
          active_stock_batch_id: "stock-ready-1",
          active_document_batch_id: "doc-ready-1",
          running_rebuild_task_id: "",
          stock_index_count: 12,
          report_index_count: 3,
          news_index_count: 4,
          watchlist_note_index_count: 5,
          last_rebuild_at: "2026-06-22T13:30:00Z",
          tokenizer_name: "simple",
          tokenizer_version: "1",
          dictionary_hash: "builtin",
        },
      };
    }
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
    if (command === "export_logs") {
      return {
        file_path: "/tmp/invest-compass-logs/invest-compass-logs-20260624-120000.txt",
        file_name: "invest-compass-logs-20260624-120000.txt",
      };
    }
    const dataSourceCredentialResponse = dataSourceCredentialCommandResponse(command, payload);
    if (dataSourceCredentialResponse) {
      return dataSourceCredentialResponse;
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("应用基础设置")).toBeInTheDocument();
  });
  expect(screen.getByRole("tab", { name: "基础设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tab", { name: "通知设置" })).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "工作区设置" })).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "缓存管理" })).toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "开机自启" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "检查更新" })).not.toBeInTheDocument();
  expect(screen.getByText("配置应用的基本行为与偏好设置")).toBeInTheDocument();
  expect(screen.getByText("默认 AI 模型")).toBeInTheDocument();
  expect(screen.getByText("DeepSeek / DeepSeek-V3")).toBeInTheDocument();
  expect(screen.getByText("行情刷新频率")).toBeInTheDocument();
  expect(screen.getByText("默认 K 线周期")).toBeInTheDocument();
  expect(screen.getByText("默认复权类型")).toBeInTheDocument();
  expect(screen.queryByText("管理应用的工作区路径与数据存储位置")).not.toBeInTheDocument();
  expect(screen.queryByText("配置任务与系统通知的接收方式")).not.toBeInTheDocument();
  expect(screen.getByText("配置桌面端行为与系统集成能力")).toBeInTheDocument();
  expect(screen.queryByText("管理本地缓存与临时文件")).not.toBeInTheDocument();
  expect(screen.getByText("搜索索引")).toBeInTheDocument();
  expect(screen.getByText("simple@1")).toBeInTheDocument();
  expect(screen.getByText("stock-ready-1")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /重建报告索引/ })).toBeInTheDocument();
  expect(screen.getByText("系统代理")).toBeInTheDocument();
  expect(screen.queryByText("其他设置")).not.toBeInTheDocument();
  expect(screen.queryByText("帮助我们改进产品（不会收集个人信息）")).not.toBeInTheDocument();
  expect(screen.getByText("敏感信息会脱敏保存；日志导出前将自动清理 API Key 与代理密码。")).toBeInTheDocument();

  expect(getSwitchByTitle("开机自启动")).not.toBeChecked();
  expect(getSwitchByTitle("关闭后最小化到托盘")).toBeChecked();

  fireEvent.click(screen.getByRole("tab", { name: "通知设置" }));
  expect(screen.getByRole("tab", { name: "通知设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("配置任务与系统通知的接收方式")).toBeInTheDocument();
  expect(screen.queryByText("应用基础设置")).not.toBeInTheDocument();
  expect(getSwitchByTitle("应用内通知")).toBeChecked();
  expect(getSwitchByTitle("系统级通知")).toBeChecked();
  expect(getSwitchByTitle("任务成功通知")).toBeChecked();
  expect(getSwitchByTitle("任务失败通知")).toBeChecked();
  expect(getSwitchByTitle("Provider 异常通知")).toBeChecked();

  fireEvent.click(screen.getByRole("tab", { name: "工作区设置" }));
  expect(screen.getByRole("tab", { name: "工作区设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("管理应用的工作区路径与数据存储位置")).toBeInTheDocument();
  expect(screen.queryByText("配置任务与系统通知的接收方式")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "选择目录" }));
  fireEvent.click(screen.getByRole("button", { name: "打开目录" }));

  fireEvent.click(screen.getByRole("tab", { name: "缓存管理" }));
  expect(screen.getByRole("tab", { name: "缓存管理" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("管理本地缓存与临时文件")).toBeInTheDocument();
  expect(screen.queryByText("管理应用的工作区路径与数据存储位置")).not.toBeInTheDocument();
  await waitFor(() => {
    expect(screen.getAllByText("1.5 MB")).toHaveLength(2);
  });
  expect(screen.getByText("行情缓存")).toBeInTheDocument();
  expect(screen.getByText("任务日志")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));
  await waitFor(() => {
    expect(screen.getByText("确认清理缓存")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("button", { name: "确认清理" }));
  await waitFor(() => {
    expect(calls.filter((call) => call.command === "cache_clean")).toHaveLength(1);
  });

  fireEvent.click(screen.getByRole("tab", { name: "基础设置" }));
  expect(screen.getByRole("tab", { name: "基础设置" })).toHaveAttribute("aria-selected", "true");

  fireEvent.click(screen.getByRole("button", { name: /编辑代理设置/ }));
  fireEvent.click(screen.getByRole("tab", { name: "模型设置" }));
  expect(screen.getByRole("tab", { name: "模型设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("Provider 列表")).toBeInTheDocument();
  expect(screen.getByText("模型配置列表")).toBeInTheDocument();
  expect(screen.queryByText("默认 OpenAI")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("tab", { name: "Prompt 配置" }));
  expect(screen.getByRole("tab", { name: "Prompt 配置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tab", { name: "关于应用" })).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Prompt 模板" })).toBeInTheDocument();
  expect(screen.getByText("管理系统提示词与投研分析模板")).toBeInTheDocument();
  expect(screen.getByText("模板分类")).toBeInTheDocument();
  expect(screen.queryByText("默认个股分析模板")).not.toBeInTheDocument();
  expect(screen.getByText("变量说明")).toBeInTheDocument();
  expect(screen.getByText("{{stock_name}}")).toBeInTheDocument();
  expect(screen.getByText("{{analysis_language}}")).toBeInTheDocument();
  expect(screen.getByText("输出预览")).toBeInTheDocument();
  expect(screen.getAllByText("暂无预览内容").length).toBeGreaterThan(0);
  fireEvent.click(screen.getByRole("button", { name: "新增分类" }));
  expect(screen.queryByText("新建分类")).not.toBeInTheDocument();
  expect(screen.queryByText("策略模板")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /新建模板/ }));
  fireEvent.change(screen.getByLabelText("模板名称"), { target: { value: "技术突破模板" } });
  fireEvent.change(screen.getByLabelText("Prompt 内容"), { target: { value: "# 测试模板\n{{stock_code}}" } });
  fireEvent.click(screen.getByRole("button", { name: /保存/ }));
  await waitFor(() => {
    expect(calls.some((call) => call.command === "prompt_templates_create")).toBe(true);
  });
  expect(calls).toContainEqual({
    command: "prompt_templates_create",
    payload: {
      payload: {
        name: "技术突破模板",
        type: "stock_full",
        description: "",
        content: "# 测试模板\n{{stock_code}}",
      },
    },
  });
  await waitFor(() => {
    expect(screen.getAllByDisplayValue("技术突破模板").length).toBeGreaterThan(0);
  });
  fireEvent.click(screen.getByRole("button", { name: /复制/ }));
  await waitFor(() => {
    expect(writeText).toHaveBeenCalledWith("# 测试模板\n{{stock_code}}");
  });
  fireEvent.click(screen.getByRole("button", { name: "格式化" }));
  fireEvent.click(screen.getByRole("button", { name: "全屏编辑" }));
  fireEvent.mouseDown(screen.getByText("Markdown"));
  fireEvent.click(screen.getAllByRole("option", { name: "纯文本" })[0]);
  expect(screen.queryByText("输入激活码")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("tab", { name: "数据源设置" }));
  expect(screen.getByRole("tab", { name: "数据源设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tab", { name: "数据源概览" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("数据源基础设置")).toBeInTheDocument();
  expect(screen.getByText("默认行情源")).toBeInTheDocument();
  expect(screen.getByText("自动降级")).toBeInTheDocument();
  expect(screen.getByText("选择自动降级时，会按已启用数据源顺序尝试；前一数据源无数据或数据不完整时使用下一个数据源备份。")).toBeInTheDocument();
  const marketSourceField = screen.getByText("默认行情源").closest(".settings-data-source-field") as HTMLElement;
  const marketSourceSelector = marketSourceField.querySelector(".ant-select-content") as HTMLElement;
  fireEvent.mouseDown(marketSourceSelector);
  expect(screen.queryByTitle("AkShare / EastMoney")).not.toBeInTheDocument();
  expect(screen.queryByTitle("Custom Provider")).not.toBeInTheDocument();
  fireEvent.click(screen.getByTitle("通达信（K线）"));
  await waitFor(() => {
    expect(calls).toContainEqual({
      command: "settings_set",
      payload: { payload: { items: [{ key: "data_source.default_market_source", value: "tdx" }] } },
    });
  });
  expect(screen.getByText("新闻同步频率")).toBeInTheDocument();
  expect(screen.queryByText("启动时自动同步")).not.toBeInTheDocument();
  expect(screen.getByText("行情数据源")).toBeInTheDocument();
  expect(screen.getByText("资讯与新闻源")).toBeInTheDocument();
  expect(screen.getByText("同步任务策略")).toBeInTheDocument();
  expect(screen.getByText("本地缓存与快照")).toBeInTheDocument();
  expect(screen.getByText("数据源状态摘要")).toBeInTheDocument();
  expect(screen.getByText("数据合规与说明")).toBeInTheDocument();
  await waitFor(() => {
    expect(screen.getAllByText("market").length).toBeGreaterThan(0);
  });
  expect(screen.getAllByText("sina").length).toBeGreaterThan(0);
  expect(screen.getAllByText("1 MB").length).toBeGreaterThan(0);
  expect(screen.getByText("后端未提供")).toBeInTheDocument();
  expect(screen.getByText("已启用任务")).toBeInTheDocument();
  expect(screen.getByText("3 / 5")).toBeInTheDocument();
  expect(screen.getByText("数据仅用于本地研究与分析展示")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
  fireEvent.click(screen.getByRole("button", { name: /编辑配置/ }));
  expect(screen.getByRole("tab", { name: "凭据管理" })).toHaveAttribute("aria-selected", "true");
  fireEvent.click(screen.getByRole("tab", { name: "数据源概览" }));
  expect(screen.getByRole("button", { name: /立即同步/ })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: /清理缓存/ }));
  fireEvent.click(screen.getByRole("button", { name: /重新检测/ }));
  fireEvent.click(screen.getByRole("button", { name: /查看数据说明/ }));

  fireEvent.click(screen.getByRole("tab", { name: "代理设置" }));
  expect(screen.getByRole("tab", { name: "代理设置" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("代理模式")).toBeInTheDocument();
  expect(screen.getByText("系统代理（推荐）")).toBeInTheDocument();
  expect(screen.getByText("不使用代理")).toBeInTheDocument();
  expect(screen.getByText("手动代理")).toBeInTheDocument();
  expect(screen.getByText("当前使用")).toBeInTheDocument();
  expect(screen.queryByText("HTTP 代理")).not.toBeInTheDocument();
  expect(screen.queryByText("SOCKS5 代理")).not.toBeInTheDocument();
  expect(screen.getByText("代理配置")).toBeInTheDocument();
  expect(screen.getByText("当前使用系统代理设置，无需手动配置")).toBeInTheDocument();
  expect(screen.getByText(/系统代理信息由操作系统管理/)).toBeInTheDocument();
  expect(screen.getByText("代理来源")).toBeInTheDocument();
  expect(screen.getByText("操作系统")).toBeInTheDocument();
  expect(screen.getByText("PAC 模式")).toBeInTheDocument();
  expect(screen.getByText("自动检测")).toBeInTheDocument();
  expect(screen.getByText("最后检查时间")).toBeInTheDocument();
  expect(screen.getAllByText(/settings/).length).toBeGreaterThan(0);
  expect(screen.getByText("连接测试")).toBeInTheDocument();
  expect(screen.getByText("测试目标")).toBeInTheDocument();
  expect(screen.getByText("未测试")).toBeInTheDocument();
  expect(screen.getByText("点击测试连接后将使用当前代理设置访问所选目标")).toBeInTheDocument();
  expect(screen.queryByText("响应时间：128 ms")).not.toBeInTheDocument();
  expect(screen.getByText("绕过代理设置（可选）")).toBeInTheDocument();
  expect(screen.getByPlaceholderText("例如：localhost;127.0.0.1;*.local")).toBeInTheDocument();
  expect(screen.getByText("代理配置仅影响应用访问外部网络的行为，不会修改系统或其他应用的网络设置。")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /手动代理/ }));
  expect(screen.getByText("以下设置仅对当前应用生效，不会修改系统代理设置")).toBeInTheDocument();
  expect(screen.getByText("代理协议")).toBeInTheDocument();
  expect(screen.getByText("代理地址")).toBeInTheDocument();
  expect(screen.getByDisplayValue("127.0.0.1")).toBeInTheDocument();
  expect(screen.getByText("端口")).toBeInTheDocument();
  expect(screen.getByDisplayValue("7890")).toBeInTheDocument();
  expect(screen.getByText("协议类型")).toBeInTheDocument();
  expect(screen.getByText("HTTP / HTTPS")).toBeInTheDocument();
  expect(screen.getByText("首版手动代理仅支持无认证代理，已保存的代理凭据不会用于运行时请求。")).toBeInTheDocument();
  expect(screen.queryByText("身份认证")).not.toBeInTheDocument();
  expect(screen.queryByText("用户名")).not.toBeInTheDocument();
  expect(screen.queryByDisplayValue("invest_user")).not.toBeInTheDocument();
  expect(screen.queryByText("密码")).not.toBeInTheDocument();
  expect(screen.getByText("连接超时")).toBeInTheDocument();
  expect(screen.getByDisplayValue("10")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /保存代理配置/ }));
  fireEvent.click(screen.getByRole("button", { name: /清空配置/ }));
  fireEvent.click(screen.getByRole("button", { name: /不使用代理/ }));
  expect(screen.getByText("当前应用的外部数据请求不会使用系统代理或手动代理。")).toBeInTheDocument();
  fireEvent.click(screen.getByText("系统代理（推荐）"));
  fireEvent.click(screen.getByRole("button", { name: /刷新代理状态/ }));
  fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
  await waitFor(() => {
    expect(screen.getByText("响应时间：128 ms")).toBeInTheDocument();
  });
  fireEvent.change(screen.getByPlaceholderText("例如：localhost;127.0.0.1;*.local"), { target: { value: "localhost;127.0.0.1;*.local" } });
  fireEvent.click(screen.getByRole("button", { name: /保存绕过规则/ }));

  fireEvent.click(screen.getByRole("tab", { name: "关于应用" }));
  expect(screen.getByRole("tab", { name: "关于应用" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText(/投研罗盘\s+Invest Compass/)).toBeInTheDocument();
  expect(screen.getAllByText("v0.1.0").length).toBeGreaterThan(0);
  expect(screen.getByText("本地优先的 AI 投研桌面工作台")).toBeInTheDocument();
  expect(screen.getByText(/投研罗盘是一款面向个人投资者和研究者的 AI 投研助手/)).toBeInTheDocument();
  expect(screen.getByText("应用信息")).toBeInTheDocument();
  expect(screen.getByText("桌面端框架")).toBeInTheDocument();
  expect(screen.getByText("Tauri v2")).toBeInTheDocument();
  expect(screen.getByText("React + TypeScript")).toBeInTheDocument();
  expect(screen.getByText("Go Core 已连接")).toBeInTheDocument();
  expect(screen.getByText("SQLite 正常")).toBeInTheDocument();
  expect(screen.getByText("C:\\Users\\InvestCompass\\Documents\\InvestCompass")).toBeInTheDocument();
  expect(screen.getByText("C:\\Users\\InvestCompass\\AppData\\Local\\InvestCompass\\logs")).toBeInTheDocument();
  expect(screen.getAllByText("检查更新").length).toBeGreaterThan(0);
  expect(screen.getByText("未检查")).toBeInTheDocument();
  expect(screen.queryByText("当前已是最新版本")).not.toBeInTheDocument();
  expect(screen.queryByText("2025-05-18")).not.toBeInTheDocument();
  expect(screen.getByText("授权状态")).toBeInTheDocument();
  expect(screen.getByText("FREE")).toBeInTheDocument();
  expect(screen.getByText("首版仅展示授权状态，不提供激活流程与功能限制。")).toBeInTheDocument();
  expect(screen.getByText("个人非商用")).toBeInTheDocument();
  expect(screen.getByText("开源许可证")).toBeInTheDocument();
  expect(screen.getByText("用户手册")).toBeInTheDocument();
  expect(screen.getByText("日志与诊断")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "检查更新" }));
  await waitFor(() => {
    expect(screen.getByText("发现新版本")).toBeInTheDocument();
  });
  expect(screen.getByText("0.1.1")).toBeInTheDocument();
  expect(screen.getByText("发布说明可用")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /查看发布说明/ }));
  expect(screen.getByRole("dialog", { name: "内置发布说明" })).toBeInTheDocument();
  expect(screen.getByText("v0.1.0 内测版")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "关闭" }));
  fireEvent.click(screen.getByRole("button", { name: /查看 LICENSE/ }));
  expect(screen.getByRole("dialog", { name: "内置 LICENSE" })).toBeInTheDocument();
  expect(screen.getByText("GNU General Public License v3.0")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "关闭" }));
  fireEvent.click(screen.getByRole("button", { name: /打开用户手册/ }));
  expect(screen.getByRole("dialog", { name: "内置用户手册" })).toBeInTheDocument();
  expect(screen.getByText("首版使用流程")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "关闭" }));
  fireEvent.click(screen.getByRole("button", { name: /导出日志/ }));
  await waitFor(() => {
    expect(calls).toContainEqual({ command: "export_logs", payload: { targetDir: "/tmp/invest-compass-logs" } });
  });
  expect(screen.queryByText("输入激活码")).not.toBeInTheDocument();
  expect(screen.queryByText("购买专业版")).not.toBeInTheDocument();
  expect(screen.queryByText("升级 Pro")).not.toBeInTheDocument();
  expect(screen.queryByText("立即更新")).not.toBeInTheDocument();

  const settingCalls = withoutGlobalNotificationUnreadCalls(calls);
  expect(settingCalls).toEqual(expect.arrayContaining([
    { command: "settings_get", payload: basicSettingsGetPayload },
    { command: "settings_get", payload: notificationSettingsGetPayload },
    { command: "workspace_open", payload: {} },
    { command: "cache_clean", payload: { payload: { targets: ["quote", "task_logs"] } } },
    { command: "settings_set", payload: { payload: { items: [{ key: "data_source.default_market_source", value: "tdx" }] } } },
    { command: "cache_clean", payload: { payload: { targets: ["quote", "kline", "news", "search"] } } },
    {
      command: "settings_set",
      payload: {
        payload: {
          items: [
            { key: "proxy.mode", value: "custom" },
            { key: "proxy.username", value: "" },
          ],
          clear_proxy_credential: true,
        },
      },
    },
    {
      command: "settings_set",
      payload: {
        payload: {
          items: [
            { key: "proxy.mode", value: "custom" },
            { key: "proxy.http_url", value: "http://127.0.0.1:7890" },
            { key: "proxy.socks5_url", value: "" },
            { key: "proxy.no_proxy", value: "" },
            { key: "proxy.username", value: "" },
          ],
          clear_proxy_credential: true,
        },
      },
    },
    {
      command: "settings_set",
      payload: {
        payload: {
          items: [
            { key: "proxy.mode", value: "none" },
            { key: "proxy.http_url", value: "" },
            { key: "proxy.socks5_url", value: "" },
            { key: "proxy.username", value: "" },
          ],
          clear_proxy_credential: true,
        },
      },
    },
    { command: "proxy_connection_test", payload: { payload: { target: "baidu" } } },
    { command: "settings_set", payload: { payload: { items: [{ key: "proxy.no_proxy", value: "localhost;127.0.0.1;*.local" }] } } },
    { command: "check_update", payload: {} },
    { command: "export_logs", payload: { targetDir: "/tmp/invest-compass-logs" } },
  ]));
}, 10_000);

test("数据源设置凭据管理页展示脱敏凭据并仅使用本地交互", async () => {
  window.location.hash = "#/settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "core_health") {
      return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
    }
    if (command === "providers_status") {
      return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
    }
    if (command === "settings_get") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
    }
    if (command === "cache_stats") {
      return { code: 0, message: "ok", data: { total_bytes: 0, items: [] } };
    }
    if (command === "scheduler_status") {
      return { code: 0, message: "ok", data: { jobs_total: 0, jobs_enabled: 0, queued_runs: 0, running_runs: 0, failed_runs: 0 } };
    }
    if (command === "search_status") {
      return {
        code: 0,
        message: "ok",
        data: {
          fts5_status: "available",
          gse_status: "fallback",
          search_status: "ready",
          active_stock_batch_id: "",
          active_document_batch_id: "",
          running_rebuild_task_id: "",
          stock_index_count: 0,
          report_index_count: 0,
          news_index_count: 0,
          watchlist_note_index_count: 0,
          last_rebuild_at: "",
          tokenizer_name: "simple",
          tokenizer_version: "1",
          dictionary_hash: "builtin",
        },
      };
    }
    const dataSourceCredentialResponse = dataSourceCredentialCommandResponse(command, payload);
    if (dataSourceCredentialResponse) {
      return dataSourceCredentialResponse;
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("应用基础设置")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("tab", { name: "数据源设置" }));

  expect(screen.getByRole("tab", { name: "数据源概览" })).toHaveAttribute("aria-selected", "true");
  fireEvent.click(screen.getByRole("tab", { name: "凭据管理" }));
  expect(screen.getByRole("tab", { name: "凭据管理" })).toHaveAttribute("aria-selected", "true");
  await waitFor(() => {
    expect(screen.getByText("Provider 列表")).toBeInTheDocument();
  });
  expect(screen.getByText("凭据配置")).toBeInTheDocument();
  expect(screen.getByText("连接测试")).toBeInTheDocument();
  expect(screen.getByText("安全与存储说明")).toBeInTheDocument();
  expect(screen.getByText("已配置凭据概览")).toBeInTheDocument();
  expect(screen.getByText("调用限制与健康状态")).toBeInTheDocument();
  expect(screen.getByText("凭据操作日志")).toBeInTheDocument();

  ["EastMoney", "新浪财经", "腾讯财经", "AkShare", "Alpha Vantage", "财联社", "雪球"].forEach((name) => {
    expect(screen.getAllByText(name).length).toBeGreaterThan(0);
  });
  expect(screen.getAllByText("基础证券列表可访问 / K线受限").length).toBeGreaterThan(0);
  expect(screen.getAllByText("受限").length).toBeGreaterThan(0);
  fireEvent.click(screen.getByText("EastMoney"));
  expect(screen.getByText("基础证券列表（东财）")).toBeInTheDocument();
  expect(screen.queryByText("K线接口（东财）")).not.toBeInTheDocument();
  fireEvent.click(screen.getByText("财联社"));
  expect(screen.queryByText("Custom HTTP")).not.toBeInTheDocument();
  expect(screen.getByDisplayValue("财联社")).toBeInTheDocument();
  expect(screen.getByText("快讯 / 行业事件 / 日历")).toBeInTheDocument();
  expect(screen.getByDisplayValue("https://www.cls.cn")).toBeInTheDocument();
  expect(screen.getByPlaceholderText("2025-06-30 23:59")).toBeInTheDocument();
  expect(screen.getByDisplayValue("uid=****; token=****; session=****")).toBeInTheDocument();
  expect(screen.queryByText("Authorization")).not.toBeInTheDocument();
  expect(screen.queryByText("Proxy-Authorization")).not.toBeInTheDocument();
  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("授权激活")).not.toBeInTheDocument();

  fireEvent.click(screen.getByText("Alpha Vantage"));
  expect(screen.getByDisplayValue("Alpha Vantage")).toBeInTheDocument();
  expect(screen.getAllByText("海外行情").length).toBeGreaterThan(0);
  fireEvent.mouseDown(screen.getAllByRole("combobox")[0]);
  fireEvent.click(screen.getByRole("option", { name: "Cookie" }));
  expect(screen.getAllByText("Cookie").length).toBeGreaterThan(0);
  fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "https://api.example.test" } });
  expect(screen.getByDisplayValue("https://api.example.test")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "保存凭据" }));
  fireEvent.click(screen.getAllByRole("button", { name: /测试连接/ })[0]);
  fireEvent.click(screen.getByRole("button", { name: /重新测试/ }));
  fireEvent.click(screen.getByRole("button", { name: "清除凭据" }));
  expect(screen.getAllByText("确认清除 Alpha Vantage 的凭据？").length).toBeGreaterThan(0);
  fireEvent.click(screen.getByRole("button", { name: "确认清除" }));
  await waitFor(() => {
    expect(screen.getAllByText("未配置").length).toBeGreaterThan(0);
  });

  fireEvent.click(screen.getByRole("tab", { name: "数据源概览" }));
  expect(screen.getByRole("tab", { name: "数据源概览" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("数据源基础设置")).toBeInTheDocument();
  expect(screen.getByText("默认行情源")).toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "Provider 配置" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "同步策略" })).not.toBeInTheDocument();

  expect(withoutGlobalNotificationUnreadCalls(calls).map((call) => call.command)).toEqual(expect.arrayContaining([
    "settings_get",
    "ai_config_list",
    "workspace_get",
    "cache_stats",
    "search_status",
    "autostart_get",
    "scheduler_status",
    "data_source_credentials_list",
    "data_source_credentials_save",
    "data_source_credentials_test",
    "data_source_credentials_clear",
  ]));
  expect(calls).toContainEqual({ command: "settings_get", payload: basicSettingsGetPayload });
  expect(calls).toContainEqual({ command: "settings_get", payload: notificationSettingsGetPayload });
  expect(calls).toContainEqual({ command: "settings_get", payload: dataSourceSettingsGetPayload });
  expect(calls).toContainEqual({ command: "data_source_credentials_clear", payload: { payload: { providerId: "alpha-vantage" } } });
}, 10_000);

test("数据源设置数据说明页展示说明模块并仅使用本地交互", async () => {
  window.location.hash = "#/settings";
  const calls: Array<{ command: string; payload?: any }> = [];
  mockIPC((command, payload) => {
    calls.push({ command, payload });
    if (command === "core_health") {
      return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
    }
    if (command === "providers_status") {
      return { code: 0, message: "ok", data: [{ name: "market", source: "sina", available: true, last_error: "" }] };
    }
    if (command === "settings_get") {
      return { code: 0, message: "ok", data: { items: [] } };
    }
    if (command === "ai_config_list") {
      return { code: 0, message: "ok", data: { items: [defaultAIConfig] } };
    }
    if (command === "cache_stats") {
      return { code: 0, message: "ok", data: { total_bytes: 0, items: [] } };
    }
    if (command === "scheduler_status") {
      return { code: 0, message: "ok", data: { jobs_total: 0, jobs_enabled: 0, queued_runs: 0, running_runs: 0, failed_runs: 0 } };
    }
    if (command === "search_status") {
      return {
        code: 0,
        message: "ok",
        data: {
          fts5_status: "available",
          gse_status: "fallback",
          search_status: "ready",
          active_stock_batch_id: "",
          active_document_batch_id: "",
          running_rebuild_task_id: "",
          stock_index_count: 0,
          report_index_count: 0,
          news_index_count: 0,
          watchlist_note_index_count: 0,
          last_rebuild_at: "",
          tokenizer_name: "simple",
          tokenizer_version: "1",
          dictionary_hash: "builtin",
        },
      };
    }
    const dataSourceCredentialResponse = dataSourceCredentialCommandResponse(command, payload);
    if (dataSourceCredentialResponse) {
      return dataSourceCredentialResponse;
    }
    throw new Error(`unexpected command ${command}`);
  });

  render(<App />);

  await waitFor(() => {
    expect(screen.getByText("应用基础设置")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByRole("tab", { name: "数据源设置" }));

  expect(screen.getByRole("tab", { name: "数据源概览" })).toHaveAttribute("aria-selected", "true");
  fireEvent.click(screen.getByRole("tab", { name: "数据说明" }));
  expect(screen.getByRole("tab", { name: "数据说明" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("数据使用与来源说明")).toBeInTheDocument();
  expect(screen.getByText("说明行情、资讯、缓存与 AI 上下文使用边界")).toBeInTheDocument();
  expect(screen.getByText("适用范围：")).toBeInTheDocument();
  expect(screen.getByText("总览 / 自选股 / 个股详情 / 资讯中心 / AI 分析")).toBeInTheDocument();
  expect(screen.getByText("默认市场：")).toBeInTheDocument();
  expect(screen.getByText("A股")).toBeInTheDocument();
  expect(screen.getByText("行情来源：")).toBeInTheDocument();
  expect(screen.getAllByText("AkShare / EastMoney").length).toBeGreaterThan(0);
  expect(screen.getByText("资讯来源：")).toBeInTheDocument();
  expect(screen.getByText("聚合新闻源 / 个股相关新闻")).toBeInTheDocument();
  expect(screen.getByText("K线数据范围：")).toBeInTheDocument();
  expect(screen.getAllByText("近 5 年").length).toBeGreaterThan(0);
  expect(screen.getByText("数据用途：")).toBeInTheDocument();
  expect(screen.getByText("本地研究展示 / 上下文构建 / 历史快照")).toBeInTheDocument();
  expect(screen.getByText("明确说明：")).toBeInTheDocument();
  expect(screen.getByText("不用于交易执行")).toBeInTheDocument();

  ["A. 数据来源说明", "B. 更新时效说明", "C. AI 上下文说明", "D. 数据合规与边界", "E. 字段说明", "F. 常见问题"].forEach((title) => {
    expect(screen.getByText(title)).toBeInTheDocument();
  });
  expect(screen.getAllByText("行情数据").length).toBeGreaterThan(0);
  expect(screen.getByText("实时行情与分时/盘口数据")).toBeInTheDocument();
  fireEvent.mouseEnter(screen.getByText("实时行情与分时/盘口数据"));
  await waitFor(() => {
    expect(screen.getByRole("tooltip")).toHaveTextContent("实时行情与分时/盘口数据");
  });
  expect(screen.getByText("扩展海外源")).toBeInTheDocument();
  expect(screen.getByText("Alpha Vantage")).toBeInTheDocument();
  expect(screen.getByText("受限")).toBeInTheDocument();
  expect(screen.getByText("行情轮询频率")).toBeInTheDocument();
  expect(screen.getByText("2 秒（盘中）/ 10 秒（非交易时段）")).toBeInTheDocument();
  expect(screen.getByText("AI 分析会基于以下数据构建上下文，以生成研究结论与解读。")).toBeInTheDocument();
  expect(screen.getByText("事实：")).toBeInTheDocument();
  expect(screen.getByText("来自原始数据或公开信息，可直接验证")).toBeInTheDocument();
  expect(screen.getByText("推断：")).toBeInTheDocument();
  expect(screen.getByText("基于数据逻辑与模型推导，存在不确定性")).toBeInTheDocument();
  expect(screen.getByText("观点：")).toBeInTheDocument();
  expect(screen.getByText("模型综合判断与建议，不构成投资建议")).toBeInTheDocument();
  expect(screen.getByText("现价")).toBeInTheDocument();
  expect(screen.getByText("最新成交价格")).toBeInTheDocument();
  expect(screen.getByText("换手率")).toBeInTheDocument();
  expect(screen.getByText("当日成交量 / 流通股本")).toBeInTheDocument();
  expect(screen.getByText("为什么不同页面时间不完全一致？")).toBeInTheDocument();
  expect(screen.getByText("为什么 AI 报告与页面最新行情略有差异？")).toBeInTheDocument();
  expect(screen.getByText("为什么部分资讯需要凭据？")).toBeInTheDocument();
  expect(screen.getByText("敏感凭据仅保存在本地安全存储；日志导出前将自动清理 API Key 与代理密码。")).toBeInTheDocument();
  expect(screen.getByText("仅供研究，不构成投资建议。")).toBeInTheDocument();

  expect(screen.queryByText("买入")).not.toBeInTheDocument();
  expect(screen.queryByText("卖出")).not.toBeInTheDocument();
  expect(screen.queryByText("下单")).not.toBeInTheDocument();
  expect(screen.queryByText("券商账户")).not.toBeInTheDocument();
  expect(screen.queryByText("自动交易")).not.toBeInTheDocument();
  expect(screen.queryByText("收益承诺")).not.toBeInTheDocument();
  expect(screen.queryByText("授权激活")).not.toBeInTheDocument();
  expect(screen.queryByText("真实 API Key")).not.toBeInTheDocument();
  expect(screen.queryByText("真实 Cookie")).not.toBeInTheDocument();
  expect(screen.queryByText("真实 Token")).not.toBeInTheDocument();
  expect(screen.queryByText("Authorization")).not.toBeInTheDocument();
  expect(screen.queryByText("Proxy-Authorization")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "查看数据源" }));
  expect(screen.getByRole("tab", { name: "数据源概览" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("数据源基础设置")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("tab", { name: "数据说明" }));
  fireEvent.click(screen.getByRole("button", { name: "查看数据源概览" }));
  expect(screen.getByRole("tab", { name: "数据源概览" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByText("数据源基础设置")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("tab", { name: "数据说明" }));
  fireEvent.click(screen.getByRole("button", { name: "查看凭据管理 >" }));
  expect(screen.getByRole("tab", { name: "凭据管理" })).toHaveAttribute("aria-selected", "true");
  await waitFor(() => {
    expect(screen.getByText("Provider 列表")).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("tab", { name: "数据说明" }));
  expect(screen.queryByRole("button", { name: "查看更多字段说明 >" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "为什么部分资讯需要凭据？ right" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "查看更多 FAQ >" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "Provider 配置" })).not.toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "同步策略" })).not.toBeInTheDocument();
  expect(screen.getByRole("tab", { name: "数据说明" })).toHaveAttribute("aria-selected", "true");

  expect(withoutGlobalNotificationUnreadCalls(calls)).toEqual(expect.arrayContaining([
    { command: "settings_get", payload: basicSettingsGetPayload },
    { command: "settings_get", payload: notificationSettingsGetPayload },
    { command: "settings_get", payload: dataSourceSettingsGetPayload },
    { command: "cache_stats", payload: {} },
    { command: "scheduler_status", payload: {} },
    { command: "data_source_credentials_list", payload: {} },
  ]));
}, 10_000);

test("任务调度页面读取真实调度接口并支持立即执行", async () => {
  window.location.hash = "#/scheduler";
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
  window.location.hash = "#/scheduler";
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
  window.location.hash = "#/scheduler";
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
  window.location.hash = "#/scheduler";
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
  window.location.hash = "#/scheduler";
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

function ChartSourceProbe() {
  const location = useLocation();
  return <div>{`source:${location.pathname}`}</div>;
}

function getSwitchByTitle(title: string): HTMLElement {
  const row = screen.getByText(title).closest(".settings-basic-switch-row");
  const switchElement = row?.querySelector("[role='switch']");
  if (!(switchElement instanceof HTMLElement)) {
    throw new Error(`missing switch for ${title}`);
  }
  return switchElement;
}

function dataSourceCredentialCommandResponse(command: string, payload?: unknown): unknown {
  if (command === "data_source_credentials_list") {
    return { code: 0, message: "ok", data: dataSourceCredentialListFixture };
  }
  if (command === "data_source_credentials_save") {
    const config = (payload as { payload?: { config?: Record<string, unknown> } } | undefined)?.payload?.config;
    return {
      code: 0,
      message: "ok",
      data: {
        config: {
          ...config,
          credentialStatus: "normal",
          maskedCredential: "sk-****",
        },
      },
    };
  }
  if (command === "data_source_credentials_clear") {
    const providerId = (payload as { payload?: { providerId?: string } } | undefined)?.payload?.providerId ?? "alpha-vantage";
    const config = dataSourceCredentialListFixture.configs[providerId as keyof typeof dataSourceCredentialListFixture.configs];
    return {
      code: 0,
      message: "ok",
      data: {
        config: {
          ...config,
          credentialStatus: "not_configured",
          maskedCredential: "",
        },
      },
    };
  }
  if (command === "data_source_credentials_test") {
    return {
      code: 0,
      message: "ok",
      data: {
        result: {
          status: "success",
          responseTimeMs: 168,
          testedAt: "2025-05-20 15:30:00",
          messages: ["本地凭据预检通过"],
        },
      },
    };
  }
  return undefined;
}
