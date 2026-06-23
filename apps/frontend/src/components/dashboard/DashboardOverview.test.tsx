/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../test/setupDom";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";
import { DashboardOverview } from "./DashboardOverview";
import { useDashboardStore, type DashboardViewState } from "../../stores/dashboardStore";

afterEach(() => {
  useDashboardStore.setState({ state: null, loading: false, error: null, lastLoadedAt: null });
  cleanup();
  vi.restoreAllMocks();
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
