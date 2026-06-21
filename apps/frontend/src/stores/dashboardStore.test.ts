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
