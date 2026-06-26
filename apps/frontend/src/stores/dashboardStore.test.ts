/* @vitest-environment jsdom */

import { clearMocks, mockIPC } from "@tauri-apps/api/mocks";
import { afterEach, expect, test } from "vitest";
import { useDashboardStore, type DashboardViewState } from "./dashboardStore";

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

  await useDashboardStore.getState().load({ forceRefresh: true });

  const state = useDashboardStore.getState().state;
  expect(state?.indexQuotes).toHaveLength(4);
  expect(state?.indexTrends["000001.SH"]).toEqual([]);
  expect(useDashboardStore.getState().error).toBeNull();
});

test("Dashboard store 保留后端摘要并使用 K 线接口加载指数走势", async () => {
  const klinePayloads: unknown[] = [];
  const quotePayloads: unknown[] = [];
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
        quotePayloads.push(payload);
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
          data: { items: [{ symbol: args.symbol, period: "minute", adjust: "none", trade_date: "2026-06-23 09:30", close: 3000 }] },
        };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  await useDashboardStore.getState().load({ forceRefresh: true });

  const state = useDashboardStore.getState().state;
  expect(state?.summary.recent_reports[0]?.title).toBe("后端报告");
  expect(state?.summary.recent_tasks[0]?.title).toBe("后端任务");
  expect(state?.summary.market_news[0]?.title).toBe("后端新闻");
  expect(state?.summary.provider_statuses[0]?.name).toBe("EastMoney");
  expect(state?.watchlistRows[0]?.quote?.symbol).toBe("000001.SZ");
  expect(quotePayloads).toEqual([
    { symbol: "000001.SH", forceRefresh: true },
    { symbol: "399001.SZ", forceRefresh: true },
    { symbol: "399006.SZ", forceRefresh: true },
    { symbol: "000300.SH", forceRefresh: true },
    { symbol: "000001.SZ", forceRefresh: true },
  ]);
  expect(klinePayloads).toEqual([
    { symbol: "000001.SH", period: "minute", adjust: "none", limit: 242 },
    { symbol: "399001.SZ", period: "minute", adjust: "none", limit: 242 },
    { symbol: "399006.SZ", period: "minute", adjust: "none", limit: 242 },
    { symbol: "000300.SH", period: "minute", adjust: "none", limit: 242 },
  ]);
});

test("Dashboard store 非强制刷新时复用自选股本地缓存，避免离开概览后定时刷新逐股打远端", async () => {
  const quotePayloads: unknown[] = [];
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
        return {
          code: 0,
          message: "ok",
          data: {
            items: [
              {
                id: 1,
                symbol: "CN:SH:601138",
                name: "工业富联",
                quote: {
                  symbol: "CN:SH:601138",
                  price: 75.71,
                  change_percent: -1.39,
                  quote_time: "2026-06-25T13:18:00+08:00",
                },
              },
            ],
          },
        };
      case "market_quote":
        quotePayloads.push(payload);
        return {
          code: 0,
          message: "ok",
          data: { symbol: args.symbol, price: 4100, change_percent: 0.2, quote_time: "2026-06-25T13:18:00+08:00" },
        };
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  await useDashboardStore.getState().load();

  expect(useDashboardStore.getState().state?.watchlistRows[0]?.quote?.symbol).toBe("CN:SH:601138");
  expect(quotePayloads).toEqual([
    { symbol: "000001.SH" },
    { symbol: "399001.SZ" },
    { symbol: "399006.SZ" },
    { symbol: "000300.SH" },
  ]);
});

test("Dashboard store 刷新时不清空已有概览，避免菜单切回时误显示连接中", async () => {
  const coreHealthGate = deferredCommand();
  const previousState: DashboardViewState = {
    health: { status: "ok", version: "0.1.0" },
    summary: {
      watchlist: { up_count: 0, down_count: 0, flat_count: 0 },
      recent_reports: [],
      recent_tasks: [],
      market_news: [],
      risk_tips: [],
      provider_statuses: [],
    },
    indexQuotes: [{ symbol: "000001.SH", quote: null, error: null }],
    indexTrends: {},
    watchlistRows: [],
  };
  useDashboardStore.setState({ state: previousState, loading: false, error: null, lastLoadedAt: "2026-06-23T00:00:00Z" });
  mockIPC((command) => {
    switch (command) {
      case "core_health":
        return coreHealthGate.promise;
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
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return { code: 50000, message: "quote unavailable", data: null };
      case "market_kline":
        return { code: 50000, message: "kline unavailable", data: null };
      default:
        throw new Error(`unexpected command ${command}`);
    }
  });

  const loadPromise = useDashboardStore.getState().load();

  expect(useDashboardStore.getState().state).toBe(previousState);

  coreHealthGate.resolve({ code: 0, message: "ok", data: { status: "ok", version: "0.1.0" } });
  await loadPromise;
});

test("Dashboard store 先渲染本地总览数据，不等待远端行情和 K 线请求", async () => {
  const quoteGate = deferredCommand();
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
            watchlist: { up_count: 2, down_count: 1, flat_count: 0 },
            recent_reports: [],
            recent_tasks: [],
            market_news: [{ title: "本地缓存新闻", source: "SQLite" }],
            risk_tips: [],
            provider_statuses: [],
          },
        };
      case "watchlist_list":
        return { code: 0, message: "ok", data: { items: [] } };
      case "market_quote":
        return quoteGate.promise;
      case "market_kline":
        return { code: 50000, message: "kline unavailable", data: null };
      default:
        throw new Error(`unexpected command ${command} ${JSON.stringify(args)}`);
    }
  });

  const loadPromise = useDashboardStore.getState().load();

  await waitForStore(() => useDashboardStore.getState().state !== null);
  expect(useDashboardStore.getState().state?.summary.market_news[0]?.title).toBe("本地缓存新闻");
  expect(useDashboardStore.getState().state?.indexQuotes).toHaveLength(4);
  expect(useDashboardStore.getState().loading).toBe(false);

  quoteGate.resolve({
    code: 0,
    message: "ok",
    data: { symbol: "000001.SH", price: 3000, change_percent: 0.2, quote_time: "2026-06-24T15:00:00+08:00" },
  });
  await loadPromise;
});

test("Dashboard store 忽略旧刷新晚到的远端行情结果", async () => {
  const staleQuoteGates = Array.from({ length: 4 }, () => deferredCommand());
  let quoteCallCount = 0;
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
      case "market_quote": {
        const callIndex = quoteCallCount;
        quoteCallCount += 1;
        if (callIndex < staleQuoteGates.length) {
          return staleQuoteGates[callIndex].promise;
        }
        return {
          code: 0,
          message: "ok",
          data: { symbol: args.symbol, price: 4102.03, change_percent: -0.21, quote_time: "2026-06-25T09:56:49+08:00" },
        };
      }
      case "market_kline":
        return { code: 0, message: "ok", data: { items: [] } };
      default:
        throw new Error(`unexpected command ${command} ${JSON.stringify(args)}`);
    }
  });

  const firstLoad = useDashboardStore.getState().load();
  await waitForStore(() => useDashboardStore.getState().state !== null);

  await useDashboardStore.getState().load();
  expect(useDashboardStore.getState().state?.indexQuotes[0]?.quote?.price).toBe(4102.03);

  for (const gate of staleQuoteGates) {
    gate.resolve({
      code: 0,
      message: "ok",
      data: { symbol: "000001.SH", price: 3000, change_percent: 0.2, quote_time: "2026-06-24T15:00:00+08:00" },
    });
  }
  await firstLoad;

  expect(useDashboardStore.getState().state?.indexQuotes[0]?.quote?.price).toBe(4102.03);
});

function deferredCommand() {
  let resolve!: (value: unknown) => void;
  const promise = new Promise((settle) => {
    resolve = settle;
  });
  return { promise, resolve };
}

async function waitForStore(predicate: () => boolean) {
  for (let index = 0; index < 20; index += 1) {
    if (predicate()) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
  throw new Error("store condition was not reached");
}
