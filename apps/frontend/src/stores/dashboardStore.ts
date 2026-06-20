import { create } from "zustand";
import {
  coreHealth,
  dashboardSummary,
  marketQuote,
  watchlistList,
  type CoreHealth,
  type DashboardSummary,
  type MarketQuote,
  type WatchlistItem,
} from "../services/coreClient";

export type DashboardQuoteState = {
  symbol: string;
  quote: MarketQuote | null;
  error: string | null;
};

export type DashboardWatchlistRow = WatchlistItem & {
  quote: MarketQuote | null;
  error: string | null;
};

export type DashboardViewState = {
  health: CoreHealth;
  summary: DashboardSummary;
  indexQuotes: DashboardQuoteState[];
  watchlistRows: DashboardWatchlistRow[];
};

type DashboardStoreState = {
  state: DashboardViewState | null;
  loading: boolean;
  error: string | null;
  lastLoadedAt: string | null;
  load: () => Promise<void>;
};

const overviewIndexSymbols = ["000001.SH", "399001.SZ", "399006.SZ", "000300.SH"];

// 首页总览 store 统一编排真实 command，避免组件里散落多份异步状态。
export const useDashboardStore = create<DashboardStoreState>((set) => ({
  state: null,
  loading: false,
  error: null,
  lastLoadedAt: null,
  load: async () => {
    set({ state: null, loading: true, error: null });
    try {
	      const [health, summary, watchlist, indexQuotes] = await Promise.all([
	        coreHealth(),
	        dashboardSummary(),
	        watchlistList(),
	        loadQuoteStates(overviewIndexSymbols),
	      ]);
      const watchlistRows = await loadWatchlistRows(watchlist.items);
      set({
        state: {
          health,
          summary,
          indexQuotes,
          watchlistRows,
        },
        loading: false,
        error: null,
        lastLoadedAt: new Date().toISOString(),
      });
    } catch (cause) {
      set({
        loading: false,
        error: cause instanceof Error ? cause.message : "本地核心服务连接失败",
      });
    }
  },
}));

async function loadQuoteStates(symbols: string[]): Promise<DashboardQuoteState[]> {
  return Promise.all(
    symbols.map(async (symbol) => {
      try {
        return { symbol, quote: await marketQuote(symbol), error: null };
      } catch (cause) {
        return { symbol, quote: null, error: cause instanceof Error ? cause.message : "行情读取失败" };
      }
    }),
  );
}

async function loadWatchlistRows(items: WatchlistItem[]): Promise<DashboardWatchlistRow[]> {
  return Promise.all(
    items.map(async (item) => {
      try {
        return { ...item, quote: await marketQuote(item.symbol), error: null };
      } catch (cause) {
        return { ...item, quote: null, error: cause instanceof Error ? cause.message : "行情读取失败" };
      }
    }),
  );
}
