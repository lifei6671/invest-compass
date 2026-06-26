import { create } from "zustand";
import {
  coreHealth,
  dashboardSummary,
  marketKline,
  marketQuote,
  watchlistList,
  type CoreHealth,
  type DashboardSummary,
  type MarketKlineItem,
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
  indexTrends: Record<string, MarketKlineItem[]>;
  watchlistRows: DashboardWatchlistRow[];
};

type DashboardStoreState = {
  state: DashboardViewState | null;
  loading: boolean;
  error: string | null;
  lastLoadedAt: string | null;
  load: (options?: DashboardLoadOptions) => Promise<void>;
};

type DashboardLoadOptions = {
  forceRefresh?: boolean;
};

const overviewIndexSymbols = ["000001.SH", "399001.SZ", "399006.SZ", "000300.SH"];
const intradayTrendKlinePayload = { period: "minute", adjust: "none", limit: 242 } as const;
let dashboardLoadSequence = 0;

// 首页总览 store 统一编排真实 command，避免组件里散落多份异步状态。
export const useDashboardStore = create<DashboardStoreState>((set, get) => ({
  state: null,
  loading: false,
  error: null,
  lastLoadedAt: null,
  load: async (options) => {
    dashboardLoadSequence += 1;
    const loadSequence = dashboardLoadSequence;
    set((current) => ({ state: current.state, loading: true, error: null }));
    try {
      const [health, summary, watchlist] = await Promise.all([
        coreHealth(),
        dashboardSummary(),
        watchlistList(),
      ]);
      if (loadSequence !== dashboardLoadSequence) {
        return;
      }

      const previousState = get().state;
      const baseState: DashboardViewState = {
        health,
        summary,
        indexQuotes: previousState?.indexQuotes.length ? normalizeIndexQuotes(previousState.indexQuotes) : emptyIndexQuotes(),
        indexTrends: previousState?.indexTrends ?? {},
        watchlistRows: preserveWatchlistRows(watchlist.items, previousState?.watchlistRows ?? []),
      };
      set({
        state: baseState,
        loading: false,
        error: null,
        lastLoadedAt: new Date().toISOString(),
      });

      const [indexQuotes, indexTrends, watchlistRows] = await Promise.all([
        loadQuoteStates(overviewIndexSymbols, options),
        loadIndexTrends(overviewIndexSymbols),
        loadWatchlistRows(watchlist.items, options),
      ]);
      const latestState = get().state;
      if (!latestState || loadSequence !== dashboardLoadSequence) {
        return;
      }
      set({
        state: {
          ...latestState,
          indexQuotes,
          indexTrends,
          watchlistRows,
        },
        loading: false,
        error: null,
        lastLoadedAt: new Date().toISOString(),
      });
    } catch (cause) {
      if (loadSequence !== dashboardLoadSequence) {
        return;
      }
      set({
        loading: false,
        error: cause instanceof Error ? cause.message : "本地核心服务连接失败",
      });
    }
  },
}));

function emptyIndexQuotes(): DashboardQuoteState[] {
  return overviewIndexSymbols.map((symbol) => ({ symbol, quote: null, error: null }));
}

function normalizeIndexQuotes(items: DashboardQuoteState[]): DashboardQuoteState[] {
  const bySymbol = new Map(items.map((item) => [item.symbol, item]));
  return overviewIndexSymbols.map((symbol) => bySymbol.get(symbol) ?? { symbol, quote: null, error: null });
}

function preserveWatchlistRows(items: WatchlistItem[], previousRows: DashboardWatchlistRow[]): DashboardWatchlistRow[] {
  const bySymbol = new Map(previousRows.map((row) => [row.symbol, row]));
  return items.map((item) => {
    const previous = bySymbol.get(item.symbol);
    return { ...item, quote: item.quote ?? previous?.quote ?? null, error: previous?.error ?? null };
  });
}

async function loadQuoteStates(symbols: string[], options?: DashboardLoadOptions): Promise<DashboardQuoteState[]> {
  return Promise.all(
    symbols.map(async (symbol) => {
      try {
        return { symbol, quote: await marketQuote(symbol, { forceRefresh: Boolean(options?.forceRefresh) }), error: null };
      } catch (cause) {
        return { symbol, quote: null, error: cause instanceof Error ? cause.message : "行情读取失败" };
      }
    }),
  );
}

async function loadIndexTrends(symbols: string[]): Promise<Record<string, MarketKlineItem[]>> {
  const entries = await Promise.all(
    symbols.map(async (symbol) => {
      try {
        const result = await marketKline({ symbol, ...intradayTrendKlinePayload });
        return [symbol, result.items] as const;
      } catch {
        // 迷你走势是总览增强信息，失败时只降级为空态，不阻断核心总览读取。
        return [symbol, []] as const;
      }
    }),
  );
  return Object.fromEntries(entries);
}

async function loadWatchlistRows(items: WatchlistItem[], options?: DashboardLoadOptions): Promise<DashboardWatchlistRow[]> {
  if (!options?.forceRefresh) {
    return items.map((item) => ({ ...item, quote: item.quote ?? null, error: null }));
  }
  return Promise.all(
    items.map(async (item) => {
      try {
        return { ...item, quote: await marketQuote(item.symbol, { forceRefresh: Boolean(options?.forceRefresh) }), error: null };
      } catch (cause) {
        return { ...item, quote: null, error: cause instanceof Error ? cause.message : "行情读取失败" };
      }
    }),
  );
}
