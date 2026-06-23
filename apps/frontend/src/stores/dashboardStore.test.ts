/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import { afterEach, expect, test } from "vitest";
import { useDashboardStore } from "./dashboardStore";

afterEach(() => {
  clearMocks();
  useDashboardStore.setState({ state: null, loading: false, error: null, lastLoadedAt: null });
});

test("Dashboard store 不把自选股读取失败伪装成空列表", async () => {
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "dashboard_summary":
        return {
          code: 0,
          message: "ok",
          data: {
            watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
            recent_reports: [],
            recent_tasks: [],
            market_news: [],
            risk_tips: [],
            provider_statuses: [],
          },
        };
      case "watchlist_list":
        return { code: 50000, message: "watchlist db unavailable", data: null };
      case "market_quote":
        return { code: 50000, message: "quote unavailable", data: null };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  await useDashboardStore.getState().load();

  expect(useDashboardStore.getState().state).toBeNull();
  expect(useDashboardStore.getState().error).toBe("watchlist db unavailable");
});

test("Dashboard store 指数走势失败时保留核心总览数据并降级为空走势", async () => {
  mockIPC((command, payload) => {
    const args = payload as { symbol?: string };
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "dashboard_summary":
        return {
          code: 0,
          message: "ok",
          data: {
            watchlist: { up_count: 1, down_count: 0, flat_count: 0 },
            recent_reports: [],
            recent_tasks: [],
            market_news: [],
            risk_tips: [],
            provider_statuses: [],
          },
        };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: { symbol: args.symbol, price: 3000, change_percent: 0.2, quote_time: "2026-06-19T15:00:00+08:00" },
        };
      case "market_kline":
        return { code: 50000, message: "kline unavailable", data: null };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  await useDashboardStore.getState().load();

  const state = useDashboardStore.getState().state;
  expect(state?.indexQuotes).toHaveLength(4);
  expect(state?.indexTrends["000001.SH"]).toEqual([]);
  expect(useDashboardStore.getState().error).toBeNull();
});

test("Dashboard store 保留后端摘要并使用 K 线接口加载指数走势", async () => {
  const klinePayloads: unknown[] = [];
  mockIPC((command, payload) => {
    const args = payload as { symbol?: string };
    switch (command) {
      case "core_health":
        return { code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } };
      case "dashboard_summary":
        return {
          code: 0,
          message: "ok",
          data: {
            watchlist: { up_count: 1, down_count: 1, flat_count: 0 },
            recent_reports: [{ id: 1, title: "后端报告", symbol: "000001.SZ", created_at: "2026-06-23T10:00:00+08:00" }],
            recent_tasks: [{ id: "task-1", title: "后端任务", status: "SUCCESS", progress: 100 }],
            market_news: [{ title: "后端新闻", source: "新闻源" }],
            risk_tips: ["后端风险提示"],
            provider_statuses: [{ name: "EastMoney", source: "行情 / K线", available: true }],
          },
        };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [{ id: 1, symbol: "000001.SZ", name: "平安银行" }] } };
      case "market_quote":
        return {
          code: 0,
          message: "ok",
          data: { symbol: args.symbol, price: 10, change_percent: 0.8, quote_time: "2026-06-23T15:00:00+08:00" },
        };
      case "market_kline":
        klinePayloads.push(payload);
        return {
          code: 0,
          message: "ok",
          data: { items: [{ symbol: args.symbol, period: "day", adjust: "qfq", trade_date: "2026-06-23", close: 3000 }] },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  await useDashboardStore.getState().load();

  const state = useDashboardStore.getState().state;
  expect(state?.summary.recent_reports[0]?.title).toBe("后端报告");
  expect(state?.summary.recent_tasks[0]?.title).toBe("后端任务");
  expect(state?.summary.market_news[0]?.title).toBe("后端新闻");
  expect(state?.summary.provider_statuses[0]?.name).toBe("EastMoney");
  expect(state?.watchlistRows[0]?.quote?.symbol).toBe("000001.SZ");
  expect(klinePayloads).toEqual([
    { symbol: "000001.SH", period: "day", adjust: "qfq", limit: 40 },
    { symbol: "399001.SZ", period: "day", adjust: "qfq", limit: 40 },
    { symbol: "399006.SZ", period: "day", adjust: "qfq", limit: 40 },
    { symbol: "000300.SH", period: "day", adjust: "qfq", limit: 40 },
  ]);
});
