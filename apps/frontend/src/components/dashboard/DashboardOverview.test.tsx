/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../test/setupDom";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";
import { DashboardOverview } from "./DashboardOverview";
import { useDashboardStore, type DashboardViewState } from "../../stores/dashboardStore";

const settingsGetMock = vi.hoisted(() => vi.fn());

vi.mock("../../services/coreClient", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../services/coreClient")>()),
  settingsGet: settingsGetMock,
}));

afterEach(() => {
  useDashboardStore.setState({ state: null, loading: false, error: null, lastLoadedAt: null });
  settingsGetMock.mockReset();
  cleanup();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

test("Dashboard 总览在 A 股交易时段展示交易中状态", () => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-06-25T09:56:50+08:00"));
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
        recent_reports: [],
        recent_tasks: [],
        market_news: [],
        risk_tips: [],
        provider_statuses: [],
      },
      indexQuotes: [
        {
          symbol: "000001.SH",
          quote: { symbol: "000001.SH", price: 4102.03, change_percent: -0.21, quote_time: "2026-06-25T09:56:49+08:00" },
          error: null,
        },
      ],
      indexTrends: {},
      watchlistRows: [],
    },
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-25T09:56:49+08:00",
    load: vi.fn(),
  });

  render(
    <MemoryRouter>
      <DashboardOverview />
    </MemoryRouter>,
  );

  expect(screen.getByText("交易中")).toBeInTheDocument();
  expect(screen.queryByText("已收盘")).not.toBeInTheDocument();
});

test("Dashboard 总览点击指数趋势图进入对应指数全屏 K 线", () => {
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
        recent_reports: [],
        recent_tasks: [],
        market_news: [],
        risk_tips: [],
        provider_statuses: [],
      },
      indexQuotes: [
        {
          symbol: "000001.SH",
          quote: { symbol: "000001.SH", price: 4102.03, change_percent: -0.21, quote_time: "2026-06-25T09:56:49+08:00" },
          error: null,
        },
      ],
      indexTrends: {
        "000001.SH": [
          { symbol: "000001.SH", period: "day", adjust: "qfq", trade_date: "2026-06-24", open: 4100, high: 4120, low: 4090, close: 4102.03 },
          { symbol: "000001.SH", period: "day", adjust: "qfq", trade_date: "2026-06-25", open: 4102, high: 4110, low: 4088, close: 4093.41 },
        ],
      },
      watchlistRows: [],
    },
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-25T09:56:49+08:00",
    load: vi.fn(),
  });

  render(
    <MemoryRouter initialEntries={["/"]}>
      <Routes>
        <Route path="/" element={<DashboardOverview />} />
        <Route path="/chart/kline" element={<DashboardLocationProbe />} />
      </Routes>
    </MemoryRouter>,
  );

  fireEvent.click(screen.getByRole("link", { name: "查看上证指数K线图" }));

  expect(screen.getByText("chart:/chart/kline?symbol=000001.SH&period=day&adjust=qfq")).toBeInTheDocument();
});

test("Dashboard 总览最近报告查看入口进入报告详情路由", () => {
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
        recent_reports: [
          {
            id: 7,
            title: "CN:SH:600522 stock_full AI 分析报告",
            symbol: "CN:SH:600522",
            stock_name: "中天科技",
            analysis_type: "stock_full",
            model_name: "gpt-test",
            created_at: "2026-06-25T15:00:00+08:00",
          },
        ],
        recent_tasks: [],
        market_news: [],
        risk_tips: [],
        provider_statuses: [],
      },
      indexQuotes: [],
      indexTrends: {},
      watchlistRows: [],
    },
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-25T15:00:00+08:00",
    load: vi.fn(),
  });

  render(
    <MemoryRouter initialEntries={["/"]}>
      <Routes>
        <Route path="/" element={<DashboardOverview />} />
        <Route path="/reports/:reportId" element={<DashboardLocationProbe />} />
      </Routes>
    </MemoryRouter>,
  );

  expect(screen.getByText("中天科技（CN:SH:600522）")).toBeInTheDocument();
  expect(screen.getByText("个股综合分析")).toBeInTheDocument();
  expect(screen.queryByText("stock_full")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("link", { name: "查看" }));

  expect(screen.getByText("chart:/reports/7")).toBeInTheDocument();
});

test("Dashboard 总览缺失涨跌分布字段时不渲染 NaN", () => {
  const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 0, down_count: 0 },
        recent_reports: [],
        recent_tasks: [],
        market_news: [],
        risk_tips: [],
        provider_statuses: [],
      },
      indexQuotes: [],
      indexTrends: {},
      watchlistRows: [],
    } as unknown as DashboardViewState,
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-23T00:00:00Z",
    load: vi.fn(),
  });

  render(
    <MemoryRouter>
      <DashboardOverview />
    </MemoryRouter>,
  );

  expect(screen.queryByText("NaN")).not.toBeInTheDocument();
  expect(consoleError).not.toHaveBeenCalledWith(expect.stringContaining("NaN"), expect.anything());
});

test("Dashboard 总览展示后端返回的数据源状态", () => {
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
        recent_reports: [],
        recent_tasks: [],
        market_news: [],
        risk_tips: [],
        provider_statuses: [
          { name: "EastMoney", source: "行情 / K线", available: true },
          { name: "AkShare", source: "基础数据", available: false, last_error: "数据源未配置" },
        ],
      },
      indexQuotes: [],
      indexTrends: {},
      watchlistRows: [],
    },
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-23T00:00:00Z",
    load: vi.fn(),
  });

  render(
    <MemoryRouter>
      <DashboardOverview />
    </MemoryRouter>,
  );

  expect(screen.getByText("数据源状态")).toBeInTheDocument();
  expect(screen.getByText("EastMoney")).toBeInTheDocument();
  expect(screen.getByText("AkShare")).toBeInTheDocument();
  expect(screen.getByText("可用")).toBeInTheDocument();
  expect(screen.getByText("异常")).toBeInTheDocument();
});

test("Dashboard 总览不展示未闭环热点入口或硬编码统计占位", () => {
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 1, down_count: 0, flat_count: 0 },
        recent_reports: [],
        recent_tasks: [],
        market_news: [{ title: "后端市场新闻", source: "新闻源" }],
        risk_tips: [],
        provider_statuses: [],
      },
      indexQuotes: [],
      indexTrends: {},
      watchlistRows: [],
    },
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-23T00:00:00Z",
    load: vi.fn(),
  });

  render(
    <MemoryRouter>
      <DashboardOverview />
    </MemoryRouter>,
  );

  expect(screen.getByText("后端市场新闻")).toBeInTheDocument();
  expect(screen.queryByText("行业热点")).not.toBeInTheDocument();
  expect(screen.queryByText("概念热点")).not.toBeInTheDocument();
  expect(screen.queryByText("重点观察")).not.toBeInTheDocument();
  expect(screen.queryByText("暂未接入")).not.toBeInTheDocument();
});

test("Dashboard 总览首次加载时不误提示正在连接核心服务", () => {
  useDashboardStore.setState({
    state: null,
    loading: true,
    error: null,
    lastLoadedAt: null,
    load: vi.fn(),
  });

  render(
    <MemoryRouter>
      <DashboardOverview />
    </MemoryRouter>,
  );

  expect(screen.getByText(/正在加载概览缓存数据/)).toBeInTheDocument();
  expect(screen.queryByText(/正在连接本地核心服务/)).not.toBeInTheDocument();
});

test("Dashboard 总览涨跌分布和最近任务使用窄屏不重叠的卡片结构", () => {
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 7, down_count: 0, flat_count: 0 },
        recent_reports: [],
        recent_tasks: [
          { id: "task-1", title: "搜索索引重建", task_type: "SEARCH_REBUILD", status: "SUCCESS", progress: 100 },
          { id: "task-2", title: "后端任务", type: "ANALYSIS", status: "RUNNING", progress: 30 },
        ],
        market_news: [],
        risk_tips: [],
        provider_statuses: [],
      },
      indexQuotes: [],
      indexTrends: {},
      watchlistRows: [
        { id: 1, symbol: "000001.SZ", name: "平安银行", quote: { symbol: "000001.SZ", price: 10, change_percent: 1.2 }, error: null },
      ],
    } as unknown as DashboardViewState,
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-23T00:00:00Z",
    load: vi.fn(),
  });

  const { container } = render(
    <MemoryRouter>
      <DashboardOverview />
    </MemoryRouter>,
  );

  expect(container.querySelector(".dashboard-watchlist-distribution-body")).toBeInTheDocument();
  expect(container.querySelector(".dashboard-watchlist-distribution-stats")).toBeInTheDocument();
  expect(container.querySelector(".dashboard-watchlist-distribution-body")?.className).toContain("dashboard-watchlist-distribution-stack");
  expect(container.querySelector(".dashboard-watchlist-distribution-stats")?.className).not.toContain("minmax(220px,1fr)");
  expect(container.querySelector(".dashboard-watchlist-distribution-stats")?.className).toContain("flex-wrap");
  expect(container.querySelector(".dashboard-watchlist-distribution-metrics")?.className).toContain("px-4");
  expect(container.querySelector(".dashboard-summary-table-body")).toBeInTheDocument();
  expect(container.querySelector(".dashboard-card-action")).toBeInTheDocument();
  expect(screen.getByText("索引重建")).toBeInTheDocument();
});

test("Dashboard 总览已有缓存时进入页面不强制刷新，定时刷新才强制刷新真实数据", async () => {
  vi.useFakeTimers();
  settingsGetMock.mockResolvedValue({ items: [{ key: "quote.refresh_interval", value: "15s" }] });
  const load = vi.fn();
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 1, down_count: 0, flat_count: 0 },
        recent_reports: [],
        recent_tasks: [],
        market_news: [],
        risk_tips: [],
        provider_statuses: [],
      },
      indexQuotes: [],
      indexTrends: {},
      watchlistRows: [],
    },
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-23T00:00:00Z",
    load,
  });

  render(
    <MemoryRouter>
      <DashboardOverview />
    </MemoryRouter>,
  );

  await Promise.resolve();
  expect(load).not.toHaveBeenCalled();

  await vi.advanceTimersByTimeAsync(15_000);

  expect(load).toHaveBeenCalledTimes(1);
  expect(load).toHaveBeenNthCalledWith(1, { forceRefresh: true });
});

function DashboardLocationProbe() {
  const location = useLocation();
  return <div>{`chart:${location.pathname}${location.search}`}</div>;
}
