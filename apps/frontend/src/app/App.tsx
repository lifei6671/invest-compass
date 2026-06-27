import { Alert, App as AntApp, Button, ConfigProvider, Spin, Tag, Typography } from "antd";
import { Component, lazy, Suspense, useEffect, useMemo, useRef, useState, type ErrorInfo, type FormEvent, type ReactNode } from "react";
import { HashRouter, Link, Route, Routes, useParams } from "react-router-dom";
import { KlineChart } from "../components/market/KlineChart";
import { SchedulerBackfillDialog } from "../components/scheduler/SchedulerBackfillDialog";
import { SchedulerJobEditor, type SchedulerJobFormState } from "../components/scheduler/SchedulerJobEditor";
import { SchedulerJobTable } from "../components/scheduler/SchedulerJobTable";
import { SchedulerRunList } from "../components/scheduler/SchedulerRunList";
import {
  aiConfigList,
  appBootStatus,
  marketIndicators,
  marketKline,
  marketQuote,
  newsList,
  openExternalURL,
  providersStatus,
  settingsGet,
  stockProfile,
  stockSearch,
  watchlistCreate,
  watchlistDelete,
  watchlistList,
  watchlistUpdate,
  type AppBootStatus,
  type MarketIndicatorsResult,
  type MarketKlineItem,
  type MarketQuote,
  type NewsItem,
  type NewsListResult,
  type ProviderStatusItem,
  type StockProfile,
  type StockSearchResult,
  type WatchlistItem,
  type WatchlistList,
} from "../services/coreClient";
import type { BasicInfoItem, KlineItem, StockDetail, StockNewsItem, TechnicalIndicator } from "../components/stock-detail/types";
import {
  schedulerJobsBackfill,
  schedulerJobsDelete,
  schedulerJobsList,
  schedulerJobsRunNow,
  schedulerJobsSave,
  schedulerJobsSetEnabled,
  schedulerJobTypes,
  schedulerRefreshSymbol,
  schedulerRunsGet,
  schedulerRunsList,
  schedulerStatus,
  type SchedulerJob,
  type SchedulerJobPayload,
  type SchedulerJobType,
  type SchedulerRun,
  type SchedulerStatus,
} from "../services/scheduler";
import { APP_FONT } from "../styles/fonts";
import { appAntdLocale } from "../lib/antdLocale";
import { AppShell } from "./AppShell";
import { AppInitializationPage } from "../pages/initialization/AppInitializationPage";
import { useAutoRefresh } from "../hooks/useAutoRefresh";

const DashboardPage = lazy(() =>
  import("../pages/dashboard/DashboardPage").then((module) => ({
    default: module.DashboardPage,
  })),
);
const WatchlistPage = lazy(() =>
  import("../components/watchlist/WatchlistPage").then((module) => ({
    default: module.WatchlistPage,
  })),
);
const StockDetailPage = lazy(() =>
  import("../components/stock-detail/StockDetailPage").then((module) => ({
    default: module.StockDetailPage,
  })),
);
const AnalysisPage = lazy(() =>
  import("../pages/analysis/AnalysisPage").then((module) => ({
    default: module.AnalysisPage,
  })),
);
const AnalysisRunningPage = lazy(() =>
  import("../pages/analysis-running/AnalysisRunningPage").then((module) => ({
    default: module.AnalysisRunningPage,
  })),
);
const ReportHistoryPage = lazy(() =>
  import("../pages/reports/ReportHistoryPage").then((module) => ({
    default: module.ReportHistoryPage,
  })),
);
const ReportDetailPage = lazy(() =>
  import("../pages/reports/detail/ReportDetailPage").then((module) => ({
    default: module.ReportDetailPage,
  })),
);
const TaskHistoryPage = lazy(() =>
  import("../pages/tasks/TaskHistoryPage").then((module) => ({
    default: module.TaskHistoryPage,
  })),
);
const NewsCenterPage = lazy(() =>
  import("../pages/news/NewsCenterPage").then((module) => ({
    default: module.NewsCenterPage,
  })),
);
const SettingsPage = lazy(() =>
  import("../pages/settings/SettingsPage").then((module) => ({
    default: module.SettingsPage,
  })),
);
const FullscreenKlinePage = lazy(() =>
  import("../pages/chart/FullscreenKlinePage").then((module) => ({
    default: module.FullscreenKlinePage,
  })),
);

type AppErrorBoundaryProps = {
  children: ReactNode;
};

type AppErrorBoundaryState = {
  hasError: boolean;
  message: string;
  componentStack: string;
};

export class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = { hasError: false, message: "", componentStack: "" };

  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { hasError: true, message: "", componentStack: "" };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // 渲染异常只进入本地边界，不把错误对象或可能包含的上下文透出到页面。
    this.setState({
      message: error.message,
      componentStack: info.componentStack ?? "",
    });
  }

  render() {
    if (this.state.hasError) {
      const isDevMode = Boolean(
        (import.meta as unknown as { env?: { DEV?: boolean } }).env?.DEV,
      );
      if (isDevMode && this.state.message) {
        return (
          <Alert
            title="界面渲染失败"
            description={
              <pre className="whitespace-pre-wrap break-words text-xs">
                {this.state.message}
                {this.state.componentStack ? `\n${this.state.componentStack}` : ""}
              </pre>
            }
            type="error"
            showIcon
          />
        );
      }
      return <Alert title="界面渲染失败" type="error" showIcon />;
    }

    return this.props.children;
  }
}

const APP_ROUTES = {
  home: "/",
  watchlist: "/watchlist",
  stockDetail: "/stocks/:symbol",
  news: "/news",
  scheduler: "/scheduler",
  analysis: "/analysis",
  analysisRunning: "/analysis/running",
  reports: "/reports",
  reportDetail: "/reports/:reportId",
  tasks: "/tasks",
  settings: "/settings",
  aiSettings: "/ai-settings",
  chartKline: "/chart/kline",
} as const;

export const APP_NAV_ITEMS = [
  { path: APP_ROUTES.home, label: "总览" },
  { path: APP_ROUTES.watchlist, label: "自选股" },
  { path: APP_ROUTES.analysis, label: "AI 分析" },
  { path: APP_ROUTES.reports, label: "报告历史" },
  { path: APP_ROUTES.news, label: "资讯中心" },
  { path: APP_ROUTES.tasks, label: "任务历史" },
  { path: APP_ROUTES.settings, label: "设置" },
] as const;

export const APP_ROUTE_PATHS = Object.values(APP_ROUTES);

export type AppBootState = "initializing" | "ready" | "failed";

type AppProps = {
  initialBootState?: AppBootState;
  bootStatusPollIntervalMs?: number;
  minimumInitializationVisibleMs?: number;
};

const defaultBootStatusPollIntervalMs = 300;
const defaultMinimumInitializationVisibleMs = 3000;
const minimumReadyVisibleMs = 800;
let bootReadyInCurrentRendererSession = false;

export function App(props: AppProps = {}) {
  const [bootState, setBootState] = useState<AppBootState>(() => resolveInitialBootState(props.initialBootState));
  const [bootStatus, setBootStatus] = useState<AppBootStatus | null>(null);
  const bootStatusPollIntervalMs = props.bootStatusPollIntervalMs ?? defaultBootStatusPollIntervalMs;
  const minimumInitializationVisibleMs = props.minimumInitializationVisibleMs ?? defaultMinimumInitializationVisibleMs;

  useEffect(() => {
    if (bootState !== "initializing") {
      return;
    }
    let active = true;
    let timer: number | undefined;
    const startedAt = Date.now();
    const pollBootStatus = async () => {
      try {
        const nextStatus = await appBootStatus();
        if (!active) {
          return;
        }
        setBootStatus(nextStatus);
        if (nextStatus.ready) {
          rememberBootReadyForCurrentSession();
          const remainingVisibleMs = Math.max(minimumReadyVisibleMs, minimumInitializationVisibleMs - (Date.now() - startedAt));
          timer = window.setTimeout(() => {
            if (active) {
              setBootState("ready");
            }
          }, remainingVisibleMs);
          return;
        }
      } catch {
        if (!active) {
          return;
        }
        // 启动状态 command 自身失败时，停留在初始化页并展示脱敏错误；下一轮轮询仍可恢复。
        setBootStatus(buildBootStatusReadFailedState());
      }
      if (active) {
        timer = window.setTimeout(pollBootStatus, bootStatusPollIntervalMs);
      }
    };
    void pollBootStatus();
    return () => {
      active = false;
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [bootState, bootStatusPollIntervalMs, minimumInitializationVisibleMs]);

  return (
    <ConfigProvider
      locale={appAntdLocale}
      theme={{
        token: {
          borderRadius: 4,
          colorPrimary: "#1677ff",
          colorTextSecondary: "#6b7280",
          colorBorder: "#e5e7eb",
          fontFamily: APP_FONT,
          fontSize: 13,
          boxShadowSecondary: "0 8px 24px rgba(15, 23, 42, 0.06)",
        },
      }}
    >
      <AntApp>
        <AppErrorBoundary>
          <HashRouter>
            {bootState === "initializing" ? (
              <AppInitializationPage state={bootStatus ?? undefined} />
            ) : (
              <AppShell routes={APP_ROUTES} navItems={APP_NAV_ITEMS}>
                <Suspense
                  fallback={
                    <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-sm text-blue-700">
                      正在连接本地核心服务 <Spin className="ml-2" size="small" />
                    </div>
                  }
                >
                  <Routes>
                    <Route path={APP_ROUTES.home} element={<DashboardPage />} />
                    <Route path={APP_ROUTES.watchlist} element={<WatchlistPage />} />
                    <Route path={APP_ROUTES.stockDetail} element={<StockDetailRoute />} />
                    <Route path={APP_ROUTES.news} element={<NewsCenterPage />} />
                    <Route path={APP_ROUTES.scheduler} element={<SchedulerRoute />} />
                    <Route path={APP_ROUTES.analysis} element={<AnalysisPage />} />
                    <Route path={APP_ROUTES.analysisRunning} element={<AnalysisRunningPage />} />
                    <Route path={APP_ROUTES.reports} element={<ReportHistoryPage />} />
                    <Route path={APP_ROUTES.reportDetail} element={<ReportDetailPage />} />
                    <Route path={APP_ROUTES.tasks} element={<TaskHistoryPage />} />
                    <Route path={APP_ROUTES.settings} element={<SettingsPage />} />
                    <Route path={APP_ROUTES.aiSettings} element={<SettingsPage initialActiveTab="model-config" />} />
                    <Route path={APP_ROUTES.chartKline} element={<FullscreenKlinePage />} />
                    <Route path="*" element={<Alert title="页面不存在" type="warning" showIcon />} />
                  </Routes>
                </Suspense>
              </AppShell>
            )}
          </HashRouter>
        </AppErrorBoundary>
      </AntApp>
    </ConfigProvider>
  );
}

function resolveInitialBootState(initialBootState?: AppBootState): AppBootState {
  const requestedState = initialBootState ?? defaultBootState();
  if (requestedState !== "initializing") {
    return requestedState;
  }
  return hasBootReadyInCurrentSession() ? "ready" : requestedState;
}

function defaultBootState(): AppBootState {
  const env = (import.meta as unknown as { env?: { MODE?: string } }).env;
  return env?.MODE === "test" ? "ready" : "initializing";
}

function rememberBootReadyForCurrentSession() {
  bootReadyInCurrentRendererSession = true;
}

function hasBootReadyInCurrentSession(): boolean {
  return bootReadyInCurrentRendererSession;
}

export function resetBootReadyForTest() {
  bootReadyInCurrentRendererSession = false;
}

function buildBootStatusReadFailedState(): AppBootStatus {
  return {
    ready: false,
    progress: 10,
    currentStepId: "sidecar",
    steps: [
      { id: "sidecar", index: 1, title: "启动 Go Core Sidecar", status: "failed", badgeText: "失败" },
      { id: "workspace", index: 2, title: "检查本地工作区与目录权限", status: "pending", badgeText: "等待中" },
      { id: "sqlite_migration", index: 3, title: "SQLite 数据库迁移 / 重建索引", status: "pending", badgeText: "等待中" },
      { id: "tokenizer", index: 4, title: "加载分词器（GSE / 全文检索词典）", status: "pending", badgeText: "等待中" },
      { id: "data_source", index: 5, title: "初始化数据源配置", status: "pending", badgeText: "等待中" },
      { id: "market_cache", index: 6, title: "同步基础行情快照与资讯缓存", status: "pending", badgeText: "等待中" },
      { id: "ready", index: 7, title: "完成基础检查并进入工作台", status: "pending", badgeText: "等待中" },
    ],
    taskDetail: {
      taskId: "boot-status-read",
      elapsed: "计算中",
      currentStage: "启动状态读取失败",
      remaining: "等待重试",
    },
    logs: [
      {
        id: "boot-status-read-failed",
        time: "--:--:--",
        status: "error",
        message: "启动状态读取失败，请稍后重试",
      },
    ],
  };
}

type SchedulerViewState = {
  status: SchedulerStatus;
  jobs: SchedulerJob[];
  runs: SchedulerRun[];
  types: SchedulerJobType[];
  providers: ProviderStatusItem[];
};

type WatchlistViewState = {
  items: WatchlistItem[];
  quotes: Record<string, MarketQuote | { error: string }>;
};

type StockDetailViewState = {
  quote: MarketQuote;
  profile: StockProfile;
  kline: MarketKlineItem[];
  indicators: MarketIndicatorsResult;
  news: NewsItem[];
  watchlistItem?: WatchlistItem;
};

type StockDetailPeriod = "day" | "week" | "month";
type StockDetailAdjust = "none" | "qfq" | "hfq";
type StockDetailRefreshRequest = {
  version: number;
};

type StockDetailSettings = {
  period: StockDetailPeriod;
  adjust: StockDetailAdjust;
};

const stockDetailSettingsKeys = ["kline.default_period", "kline.default_adjust"];
const stockDetailIndicatorKeys = ["ma", "rsi", "macd", "kdj", "boll"];

function WatchlistRoute() {
  const [state, setState] = useState<WatchlistViewState | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [searchKeyword, setSearchKeyword] = useState("");
  const [searchResults, setSearchResults] = useState<StockSearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [refreshingSymbols, setRefreshingSymbols] = useState<Record<string, boolean>>({});

  const load = () =>
    watchlistList().then(async (list) => ({
      items: list.items,
      quotes: await loadQuotes(list.items),
    }));

  useEffect(() => {
    let active = true;
    load()
      .then((nextState) => {
        if (active) {
          setState(nextState);
          setLoadError(null);
        }
      })
      .catch((error: Error) => {
        if (active) {
          setLoadError(error.message);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  const refresh = () => {
    setActionError(null);
    load()
      .then((nextState) => {
        setState(nextState);
        setActionMessage("自选股已刷新");
      })
      .catch((error: Error) => setActionError(error.message));
  };

  const search = (event: FormEvent) => {
    event.preventDefault();
    setActionError(null);
    setSearching(true);
    stockSearch(searchKeyword)
      .then((results) => setSearchResults(results))
      .catch((error: Error) => setActionError(error.message))
      .finally(() => setSearching(false));
  };

  const add = (symbol: string) => {
    setActionError(null);
    const sortOrder = state ? state.items.length * 10 + 20 : 10;
    watchlistCreate({ symbol, sort_order: sortOrder, tags: [], note: "" })
      .then(async (item) => {
        const quote = await loadQuote(item.symbol);
        setState((current) => ({
          items: [...(current?.items ?? []), item],
          quotes: { ...(current?.quotes ?? {}), [item.symbol]: quote },
        }));
        setActionMessage(`${item.symbol} 已添加`);
      })
      .catch((error: Error) => setActionError(error.message));
  };

  const updateDraft = (id: number, patch: Partial<WatchlistItem>) => {
    setState((current) =>
      current
        ? {
          ...current,
          items: current.items.map((item) => (item.id === id ? { ...item, ...patch } : item)),
        }
        : current,
    );
  };

  const save = (item: WatchlistItem) => {
    setActionError(null);
    watchlistUpdate({
      id: item.id,
      sort_order: Number(item.sort_order) || 0,
      tags: normalizeWatchlistTags(item.tags),
      note: normalizeWatchlistNote(item.note),
    })
      .then((updated) => {
        updateDraft(updated.id, updated);
        setActionMessage(`${updated.symbol} 已保存`);
      })
      .catch((error: Error) => setActionError(error.message));
  };

  const remove = (item: WatchlistItem) => {
    setActionError(null);
    watchlistDelete(item.id)
      .then(() => {
        setState((current) =>
          current
            ? {
              items: current.items.filter((value) => value.id !== item.id),
              quotes: Object.fromEntries(Object.entries(current.quotes).filter(([symbol]) => symbol !== item.symbol)),
            }
            : current,
        );
        setActionMessage(`${item.symbol} 已删除`);
      })
      .catch((error: Error) => setActionError(error.message));
  };

  const refreshSymbol = (item: WatchlistItem) => {
    setActionError(null);
    setActionMessage(null);
    setRefreshingSymbols((current) => ({ ...current, [item.symbol]: true }));
    schedulerRefreshSymbol({ symbol: item.symbol, data_type: "quote" })
      .then(async (result) => {
        const pendingRun = result.items.find((run) => run.status === "queued" || run.status === "running");
        if (pendingRun) {
          setActionMessage(`${item.symbol} 行情刷新任务已提交`);
          return;
        }
        const failedRun = result.items.find((run) => run.status === "failed" || run.status === "cancelled");
        if (failedRun) {
          throw new Error(failedRun.error_message || `${item.symbol} 行情刷新失败`);
        }
        const skippedRun = result.items.find((run) => run.status === "skipped");
        if (skippedRun) {
          setActionMessage(`${item.symbol} 行情刷新已跳过：${skippedRun.skipped_reason || "重复任务"}`);
          return;
        }
        if (result.items.length === 0 || !result.items.every((run) => run.status === "success")) {
          setActionMessage(`${item.symbol} 行情刷新任务已提交`);
          return;
        }
        const quote = await loadQuote(item.symbol);
        setState((current) =>
          current
            ? {
              ...current,
              quotes: { ...current.quotes, [item.symbol]: quote },
            }
            : current,
        );
        setActionMessage(`${item.symbol} 行情已刷新`);
      })
      .catch((error: Error) => setActionError(error.message))
      .finally(() =>
        setRefreshingSymbols((current) => {
          const next = { ...current };
          delete next[item.symbol];
          return next;
        }),
      );
  };

  if (loadError) {
    return <Alert title="自选股读取失败" description={loadError} type="error" showIcon />;
  }
  if (!state) {
    return <Alert title="正在读取自选股" description={<Spin size="small" />} type="info" showIcon />;
  }

  const sortedItems = [...state.items].sort((left, right) => left.sort_order - right.sort_order || left.symbol.localeCompare(right.symbol));

  return (
    <section className="flex flex-col gap-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <Typography.Title level={2} className="m-0">
            自选股
          </Typography.Title>
          <Typography.Text type="secondary">搜索、维护自选股，并读取真实行情快照。</Typography.Text>
        </div>
        <Button aria-label="刷新" onClick={refresh}>刷新</Button>
      </div>

      {actionError ? <Alert title={actionError} type="error" showIcon /> : null}
      {actionMessage ? <Alert title={actionMessage} type="success" showIcon /> : null}

      <form onSubmit={search} className="flex flex-wrap items-end gap-3 rounded-md border border-slate-200 bg-white p-4">
        <label className="flex min-w-60 flex-1 flex-col gap-1 text-sm">
          <span className="text-slate-600">搜索股票</span>
          <input
            aria-label="搜索股票"
            className="rounded border border-slate-300 px-3 py-2"
            value={searchKeyword}
            onChange={(event) => setSearchKeyword(event.target.value)}
          />
        </label>
        <Button aria-label="搜索" htmlType="submit" loading={searching}>搜索</Button>
      </form>

      {searchResults.length > 0 ? (
        <section className="rounded-md border border-slate-200 bg-white">
          <div className="border-b border-slate-200 px-4 py-3 text-sm font-medium text-slate-700">搜索结果</div>
          {searchResults.map((result) => (
            <div key={result.symbol} className="flex items-center justify-between border-t border-slate-100 px-4 py-3 text-sm">
              <div>
                <div className="font-medium text-slate-900">{result.name}</div>
                <div className="mt-1 text-xs text-slate-500">{[result.code, result.market, result.exchange].filter(Boolean).join(" · ")}</div>
              </div>
              <Button aria-label={`添加 ${result.symbol}`} onClick={() => add(result.symbol)}>添加 {result.symbol}</Button>
            </div>
          ))}
        </section>
      ) : null}

      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3 text-sm font-medium text-slate-700">自选列表</div>
        {sortedItems.length === 0 ? (
          <div className="px-4 py-8 text-sm text-slate-500">暂无自选股</div>
        ) : (
          sortedItems.map((item) => (
            <WatchlistRow
              key={item.id}
              item={item}
              quote={state.quotes[item.symbol]}
              refreshing={Boolean(refreshingSymbols[item.symbol])}
              onDraftChange={updateDraft}
              onRefresh={refreshSymbol}
              onSave={save}
              onDelete={remove}
            />
          ))
        )}
      </section>
    </section>
  );
}

function WatchlistRow(props: {
  item: WatchlistItem;
  quote?: MarketQuote | { error: string };
  refreshing: boolean;
  onDraftChange: (id: number, patch: Partial<WatchlistItem>) => void;
  onRefresh: (item: WatchlistItem) => void;
  onSave: (item: WatchlistItem) => void;
  onDelete: (item: WatchlistItem) => void;
}) {
  const quote = props.quote;
  const quoteError = quote && "error" in quote ? quote.error : "";
  const quoteValue = quote && !("error" in quote) ? quote : null;
  return (
    <div className="grid gap-3 border-t border-slate-100 px-4 py-4 text-sm md:grid-cols-[1fr_120px_120px_1.5fr_1.5fr_auto]">
      <div>
        <Link className="font-medium text-slate-900" to={`/stocks/${encodeURIComponent(props.item.symbol)}`}>
          {props.item.symbol}
        </Link>
        <div className="mt-1 text-xs text-slate-500">{quoteValue?.provider ? `来源 ${quoteValue.provider}` : "等待行情"}</div>
      </div>
      <div>
        <div className="text-xs text-slate-500">最新价</div>
        <div className="mt-1 font-medium text-slate-900">{quoteValue ? formatNumber(quoteValue.price) : quoteError || "-"}</div>
      </div>
      <div>
        <div className="text-xs text-slate-500">涨跌幅</div>
        <div className="mt-1 font-medium text-slate-900">{quoteValue?.change_percent === undefined ? "-" : `${formatNumber(quoteValue.change_percent)}%`}</div>
      </div>
      <label className="flex flex-col gap-1">
        <span className="text-xs text-slate-500">排序</span>
        <input
          aria-label={`排序 ${props.item.symbol}`}
          className="rounded border border-slate-300 px-2 py-1"
          value={props.item.sort_order}
          onChange={(event) => props.onDraftChange(props.item.id, { sort_order: Number(event.target.value) || 0 })}
        />
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-xs text-slate-500">标签</span>
        <input
          aria-label={`标签 ${props.item.symbol}`}
          className="rounded border border-slate-300 px-2 py-1"
          value={normalizeWatchlistTags(props.item.tags).join(",")}
          onChange={(event) => props.onDraftChange(props.item.id, { tags: splitTags(event.target.value) })}
        />
      </label>
      <div className="flex flex-col gap-2">
        <label className="flex flex-col gap-1">
          <span className="text-xs text-slate-500">备注</span>
          <input
            aria-label={`备注 ${props.item.symbol}`}
            className="rounded border border-slate-300 px-2 py-1"
            value={normalizeWatchlistNote(props.item.note)}
            onChange={(event) => props.onDraftChange(props.item.id, { note: event.target.value })}
          />
        </label>
        <div className="flex gap-2">
          <Button aria-label={`刷新 ${props.item.symbol}`} loading={props.refreshing} onClick={() => props.onRefresh(props.item)}>
            刷新 {props.item.symbol}
          </Button>
          <Button aria-label={`保存 ${props.item.symbol}`} onClick={() => props.onSave(props.item)}>保存 {props.item.symbol}</Button>
          <Button aria-label={`删除 ${props.item.symbol}`} danger onClick={() => props.onDelete(props.item)}>删除 {props.item.symbol}</Button>
        </div>
      </div>
    </div>
  );
}

async function loadQuotes(items: WatchlistItem[]): Promise<Record<string, MarketQuote | { error: string }>> {
  const entries = await Promise.all(items.map(async (item) => [item.symbol, await loadQuote(item.symbol)] as const));
  return Object.fromEntries(entries);
}

async function loadQuote(symbol: string): Promise<MarketQuote | { error: string }> {
  try {
    return await marketQuote(symbol);
  } catch (error) {
    return { error: error instanceof Error ? error.message : "quote_error" };
  }
}

function splitTags(value: string): string[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function normalizeWatchlistTags(tags: WatchlistItem["tags"]): string[] {
  if (!Array.isArray(tags)) {
    return [];
  }
  return tags.filter((item): item is string => typeof item === "string").map((item) => item.trim()).filter(Boolean);
}

function normalizeWatchlistNote(note: WatchlistItem["note"]): string {
  return typeof note === "string" ? note : "";
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value);
}

function NewsExternalLink(props: { url?: string }) {
  const [error, setError] = useState<string | null>(null);

  if (!props.url) {
    return null;
  }
  const url = props.url;

  const openLink = async () => {
    setError(null);
    try {
      await openExternalURL(url);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "打开新闻外链失败");
    }
  };

  return (
    <div className="mt-2 flex flex-col gap-2">
      <div className="break-all text-xs text-slate-500">{url}</div>
      <Button aria-label="打开新闻外链" size="small" onClick={openLink}>
        打开
      </Button>
      {error ? <span className="text-xs text-red-600">{error}</span> : null}
    </div>
  );
}

function normalizeNewsItems(items: NewsItem[]) {
  return items.filter((item) => isHTTPSURL(item.url));
}

async function loadStockDetailState(normalizedSymbol: string, period: string, adjust: string): Promise<StockDetailViewState> {
  const [quote, profile, kline, indicators, news, watchlist] = await Promise.all([
    marketQuote(normalizedSymbol),
    stockProfile(normalizedSymbol),
    marketKline({ symbol: normalizedSymbol, period, adjust, limit: 120 }),
    marketIndicators({
      symbol: normalizedSymbol,
      period,
      adjust,
      limit: 120,
      indicators: stockDetailIndicatorKeys,
    }),
    newsList({ symbol: normalizedSymbol, limit: 20 }).catch((): NewsListResult => ({ items: [] })),
    watchlistList().catch((): WatchlistList => ({ items: [] })),
  ]);
  return {
    quote,
    profile,
    kline: kline.items,
    indicators,
    news: normalizeNewsItems(news.items),
    watchlistItem: watchlist.items.find((item) => symbolsReferToSameStock(item.symbol, normalizedSymbol)),
  };
}

function symbolsReferToSameStock(left: string, right: string) {
  return normalizeComparableStockSymbol(left) === normalizeComparableStockSymbol(right);
}

function normalizeComparableStockSymbol(symbol: string) {
  const value = symbol.trim().toUpperCase();
  const cnSymbol = /^CN:(SH|SZ|BJ):(\d+)$/.exec(value);
  if (cnSymbol) {
    return `${cnSymbol[2]}.${cnSymbol[1]}`;
  }
  const dotSymbol = /^(\d+)\.(SH|SZ|BJ)$/.exec(value);
  if (dotSymbol) {
    return `${dotSymbol[1]}.${dotSymbol[2]}`;
  }
  return value;
}

export function StockDetailRoute() {
  const { message } = AntApp.useApp();
  const params = useParams();
  const normalizedSymbol = (params.symbol ?? "").trim();
  const [settingsReady, setSettingsReady] = useState(false);
  const [period, setPeriod] = useState<StockDetailPeriod>("day");
  const [adjust, setAdjust] = useState<StockDetailAdjust>("qfq");
  const [state, setState] = useState<StockDetailViewState | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [savingWatchlistNote, setSavingWatchlistNote] = useState(false);
  const [refreshRequest, setRefreshRequest] = useState<StockDetailRefreshRequest>({ version: 0 });
  const mountedRef = useRef(true);

  useEffect(() => () => {
    mountedRef.current = false;
  }, []);

  useEffect(() => {
    let cancelled = false;
    setSettingsReady(false);
    setLoadError(null);
    settingsGet(stockDetailSettingsKeys)
      .then((result) => {
        if (cancelled) {
          return;
        }
        const values = new Map(result.items.map((item) => [item.key, item.value]));
        setPeriod(normalizeStockDetailPeriod(values.get("kline.default_period")));
        setAdjust(normalizeStockDetailAdjust(values.get("kline.default_adjust")));
        setSettingsReady(true);
      })
      .catch((error: Error) => {
        if (!cancelled) {
          setLoadError(`基础设置读取失败：${error.message}`);
          setSettingsReady(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [normalizedSymbol]);

  useEffect(() => {
    if (!settingsReady || !normalizedSymbol) {
      return;
    }
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    loadStockDetailState(normalizedSymbol, period, adjust)
      .then((nextState) => {
        if (!cancelled) {
          setState(nextState);
        }
      })
      .catch((error: Error) => {
        if (!cancelled) {
          setLoadError(error.message);
          setState(null);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [adjust, normalizedSymbol, period, refreshRequest, settingsReady]);
  useAutoRefresh(() => {
    if (settingsReady && normalizedSymbol) {
      setRefreshRequest((current) => ({ version: current.version + 1 }));
    }
  });

  if (!normalizedSymbol) {
    return <StockDetailPage error="未选择股票" />;
  }

  const saveWatchlistNote = async (value: { tags: string[]; note: string }) => {
    if (!state?.watchlistItem) {
      return;
    }
    setSavingWatchlistNote(true);
    try {
      const updated = await watchlistUpdate({
        id: state.watchlistItem.id,
        sort_order: state.watchlistItem.sort_order,
        tags: value.tags,
        note: value.note,
      });
      setState((current) => (current ? { ...current, watchlistItem: updated } : current));
      message.success("自选股备注已更新");
    } catch (cause) {
      message.error(cause instanceof Error ? cause.message : "自选股备注更新失败");
    } finally {
      setSavingWatchlistNote(false);
    }
  };

  const refreshQuoteFromProvider = () => {
    if (!normalizedSymbol) {
      return;
    }
    setLoadError(null);
    void marketQuote(normalizedSymbol, { forceRefresh: true })
      .then((quote) => {
        if (mountedRef.current) {
          setState((current) => (current ? { ...current, quote } : current));
        }
      })
      .catch((cause) => {
        if (mountedRef.current) {
          const reason = cause instanceof Error ? cause.message : "刷新行情失败";
          message.error(reason);
        }
      });
  };

  return (
    <StockDetailPage
      stock={state ? toStockDetail(normalizedSymbol, state.quote, state.profile) : emptyStockDetailForSymbol(normalizedSymbol)}
      basicInfo={state ? toBasicInfo(state.quote, state.profile) : []}
      klineItems={state ? toKlineItems(state.kline) : []}
      technicalIndicators={state ? toTechnicalIndicators(state.indicators.indicators, period) : []}
      newsItems={state ? toStockNewsItems(state.news) : []}
      watchlistNote={state?.watchlistItem ? { tags: normalizeWatchlistTags(state.watchlistItem.tags), note: normalizeWatchlistNote(state.watchlistItem.note), editable: true } : { tags: [], note: "", editable: false }}
      loading={loading || !settingsReady}
      error={loadError}
      period={period}
      adjust={adjust}
      onPeriodChange={setPeriod}
      onAdjustChange={setAdjust}
      onRefresh={refreshQuoteFromProvider}
      onSaveWatchlistNote={state?.watchlistItem ? saveWatchlistNote : undefined}
      savingWatchlistNote={savingWatchlistNote}
    />
  );
}

function normalizeStockDetailPeriod(value: string | undefined): StockDetailPeriod {
  switch (value) {
    case "week":
    case "month":
      return value;
    case "minute":
    case "day":
    default:
      // Go market provider 当前仅支持 day/week/month，分时设置在详情页按日 K 加载。
      return "day";
  }
}

function normalizeStockDetailAdjust(value: string | undefined): StockDetailAdjust {
  switch (value) {
    case "none":
    case "hfq":
      return value;
    case "qfq":
    default:
      return "qfq";
  }
}

function emptyStockDetailForSymbol(symbol: string): StockDetail {
  return {
    name: symbol,
    symbol,
    code: stockCodeFromSymbol(symbol),
    industry: "暂无",
    subIndustry: "暂无",
    concepts: [],
    price: null,
    changeAmount: null,
    changePercent: null,
    open: null,
    high: null,
    low: null,
    previousClose: null,
    turnoverRate: "暂无",
    amount: "暂无",
    volume: "暂无",
    updateTime: "待加载",
  };
}

function toStockDetail(symbol: string, quote: MarketQuote, profile: StockProfile): StockDetail {
  const resolvedSymbol = quote.symbol || profile.symbol || symbol;
  return {
    name: profile.name || resolvedSymbol,
    symbol: resolvedSymbol,
    code: profile.code ? `${profile.code}${profile.exchange ? `.${profile.exchange}` : ""}` : stockCodeFromSymbol(resolvedSymbol),
    industry: profile.industry || "暂无",
    subIndustry: "暂无",
    concepts: profile.concepts ?? [],
    price: quote.price,
    changeAmount: quote.change_amount ?? null,
    changePercent: quote.change_percent ?? null,
    open: quote.open ?? null,
    high: quote.high ?? null,
    low: quote.low ?? null,
    previousClose: quote.pre_close ?? null,
    turnoverRate: typeof quote.turnover_rate === "number" ? `${formatNumber(quote.turnover_rate)}%` : "暂无",
    amount: typeof quote.amount === "number" ? formatNumber(quote.amount) : "暂无",
    volume: typeof quote.volume === "number" ? formatNumber(quote.volume) : "暂无",
    updateTime: quote.quote_time || "后端未提供",
  };
}

function toBasicInfo(quote: MarketQuote, profile: StockProfile): BasicInfoItem[] {
  return [
    { label: "股票代码", value: profile.code ? `${profile.code}${profile.exchange ? `.${profile.exchange}` : ""}` : stockCodeFromSymbol(quote.symbol) || quote.symbol },
    { label: "公司全称", value: profile.full_name || "暂无" },
    { label: "上市日期", value: profile.list_date || "暂无" },
    { label: "状态", value: profile.status || "暂无" },
    { label: "数据源", value: quote.provider || "后端未提供" },
    { label: "成交量", value: typeof quote.volume === "number" ? formatNumber(quote.volume) : "暂无" },
    { label: "成交额", value: typeof quote.amount === "number" ? formatNumber(quote.amount) : "暂无" },
    { label: "市盈率", value: typeof quote.pe === "number" ? formatNumber(quote.pe) : "暂无" },
    { label: "市净率", value: typeof quote.pb === "number" ? formatNumber(quote.pb) : "暂无" },
  ];
}

function toKlineItems(items: MarketKlineItem[]): KlineItem[] {
  return items.map((item) => ({
    date: item.trade_date,
    open: item.open,
    close: item.close,
    low: item.low,
    high: item.high,
    volume: item.volume ?? 0,
  }));
}

function toTechnicalIndicators(indicators: Record<string, unknown>, period: StockDetailPeriod): TechnicalIndicator[] {
  const orderedRows = [
    indicatorRowFromPaths("MA.MA5", indicators, ["ma.ma5", "ma5"]),
    indicatorRowFromPaths("MA.MA10", indicators, ["ma.ma10", "ma10"]),
    indicatorRowFromPaths("MA.MA20", indicators, ["ma.ma20", "ma20"]),
    indicatorRowFromPaths("MA.MA60", indicators, ["ma.ma60", "ma60"]),
    indicatorRowFromPaths("MACD.BAR", indicators, ["macd.bar", "macd_bar"]),
    indicatorRowFromPaths("MACD.DEA", indicators, ["macd.dea", "macd_dea"]),
    indicatorRowFromPaths("MACD.DIF", indicators, ["macd.dif", "macd_dif"]),
    indicatorRowFromPaths("RSI.RSI6", indicators, ["rsi.rsi6", "rsi6"]),
    indicatorRowFromPaths("KDJ.K", indicators, ["kdj.k", "kdj_k"]),
    indicatorRowFromPaths("KDJ.D", indicators, ["kdj.d", "kdj_d"]),
    indicatorRowFromPaths("KDJ.J", indicators, ["kdj.j", "kdj_j"]),
    indicatorRowFromPaths("BOLL.MID", indicators, ["boll.middle", "boll.mid", "boll_middel", "boll_middle"]),
    indicatorRowFromPaths("BOLL.UPPER", indicators, ["boll.upper", "boll_upper"]),
    indicatorRowFromPaths("BOLL.LOWER", indicators, ["boll.lower", "boll_lower"]),
  ].filter((item): item is { name: string; value: string; numericValue: number | null } => Boolean(item));
  const rows = orderedRows.length > 0
    ? orderedRows
    : indicatorRows(indicators).map((item) => ({ ...item, numericValue: numericIndicatorValue(item.value) }));
  return rows.map((item) => ({
    name: item.name.toUpperCase(),
    value: item.value,
    direction: indicatorDirection(item.numericValue),
    desc: `${periodLabel(period)}最新值`,
  }));
}

function toStockNewsItems(items: NewsItem[]): StockNewsItem[] {
  return items.map((item) => ({
    id: String(item.id),
    title: item.title,
    source: item.source || "未知来源",
    publishedAt: item.published_at || "后端未提供",
  }));
}

function stockCodeFromSymbol(symbol: string) {
  const parts = symbol.split(/[.:]/).filter(Boolean);
  return parts.find((part) => /^\d{5,6}$/.test(part)) ?? "";
}

function periodLabel(period: StockDetailPeriod) {
  if (period === "week") {
    return "周K";
  }
  if (period === "month") {
    return "月K";
  }
  return "日K";
}

function KlinePanel(props: { items: MarketKlineItem[] }) {
  return (
    <DashboardList title="K 线图" emptyText="暂无 K 线数据">
      {props.items.length > 0 ? (
        <div className="border-t border-slate-100 px-4 py-4">
          <KlineChart items={props.items} />
          <table className="mt-4 w-full table-fixed text-left text-sm">
            <thead className="text-xs text-slate-500">
              <tr>
                <th className="py-2 font-medium">交易日</th>
                <th className="py-2 font-medium">开盘</th>
                <th className="py-2 font-medium">最高</th>
                <th className="py-2 font-medium">最低</th>
                <th className="py-2 font-medium">收盘</th>
              </tr>
            </thead>
            <tbody>
              {props.items.slice(-5).map((item) => (
                <tr key={`${item.trade_date}-${item.period}-${item.adjust}`} className="border-t border-slate-100">
                  <td className="py-2">{item.trade_date}</td>
                  <td className="py-2">{formatNumber(item.open)}</td>
                  <td className="py-2">{formatNumber(item.high)}</td>
                  <td className="py-2">{formatNumber(item.low)}</td>
                  <td className="py-2">{formatNumber(item.close)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </DashboardList>
  );
}

function IndicatorPanel(props: { indicators: Record<string, unknown> }) {
  const rows = indicatorRows(props.indicators);
  return (
    <DashboardList title="技术指标" emptyText="暂无技术指标">
      {rows.map((row) => (
        <div key={row.name} className="grid grid-cols-[1fr_auto] border-t border-slate-100 px-4 py-3 text-sm">
          <span className="font-medium text-slate-700">{row.name}</span>
          <span className="text-slate-900">{row.value}</span>
        </div>
      ))}
    </DashboardList>
  );
}

function NewsPanel(props: { items: NewsItem[] }) {
  return (
    <DashboardList title="相关新闻" emptyText="暂无相关新闻">
      {props.items.map((item) => (
        <article key={item.id} className="border-t border-slate-100 px-4 py-3 text-sm">
          <div className="font-medium text-slate-900">{item.title}</div>
          <div className="mt-1 text-xs text-slate-500">
            {[item.source, item.published_at].filter(Boolean).join(" · ")}
          </div>
          {item.summary ? <p className="mt-2 text-slate-600">{item.summary}</p> : null}
          <NewsExternalLink url={item.url} />
        </article>
      ))}
    </DashboardList>
  );
}

function isHTTPSURL(value?: string) {
  if (!value) {
    return false;
  }
  try {
    return new URL(value).protocol === "https:";
  } catch {
    return false;
  }
}

function indicatorRows(indicators: Record<string, unknown>) {
  return Object.entries(indicators).flatMap(([key, value]) => flattenIndicatorValue(key, value));
}

function flattenIndicatorValue(name: string, value: unknown): Array<{ name: string; value: string; numericValue: number | null }> {
  if (Array.isArray(value)) {
    const latest = latestPrimitive(value);
    return latest === null ? [] : [{ name, value: formatIndicatorValue(latest), numericValue: typeof latest === "number" ? latest : null }];
  }
  if ((typeof value === "number" && Number.isFinite(value)) || typeof value === "string" || typeof value === "boolean") {
    return [{ name, value: formatIndicatorValue(value), numericValue: typeof value === "number" ? value : numericIndicatorValue(value) }];
  }
  if (value && typeof value === "object") {
    return Object.entries(value).flatMap(([key, child]) => flattenIndicatorValue(`${name}.${key}`, child));
  }
  return [];
}

function indicatorRowFromPaths(name: string, indicators: Record<string, unknown>, paths: string[]) {
  for (const path of paths) {
    const value = latestIndicatorValue(readIndicatorPath(indicators, path));
    if (value !== null) {
      return { name, value: formatIndicatorValue(value), numericValue: typeof value === "number" ? value : numericIndicatorValue(value) };
    }
  }
  return null;
}

function readIndicatorPath(indicators: Record<string, unknown>, path: string): unknown {
  return path.split(".").reduce<unknown>((current, key) => {
    if (current && typeof current === "object" && key in current) {
      return (current as Record<string, unknown>)[key];
    }
    return undefined;
  }, indicators);
}

function latestIndicatorValue(value: unknown): number | string | boolean | null {
  if (Array.isArray(value)) {
    return latestPrimitive(value);
  }
  if ((typeof value === "number" && Number.isFinite(value)) || typeof value === "string" || typeof value === "boolean") {
    return value;
  }
  return null;
}

function latestPrimitive(values: unknown[]) {
  for (let index = values.length - 1; index >= 0; index -= 1) {
    const value = values[index];
    if ((typeof value === "number" && Number.isFinite(value)) || typeof value === "string" || typeof value === "boolean") {
      return value;
    }
  }
  return null;
}

function formatIndicatorValue(value: number | string | boolean) {
  return typeof value === "number" ? formatNumber(value) : String(value);
}

function numericIndicatorValue(value: string | boolean) {
  if (typeof value === "boolean") {
    return null;
  }
  const parsed = Number(value.replace(/,/g, ""));
  return Number.isFinite(parsed) ? parsed : null;
}

function indicatorDirection(value: number | null): TechnicalIndicator["direction"] {
  if (value === null || value === 0) {
    return "flat";
  }
  return value > 0 ? "up" : "down";
}

function SchedulerRoute() {
  const [state, setState] = useState<SchedulerViewState | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [actionPending, setActionPending] = useState<string | null>(null);
  const [selectedRun, setSelectedRun] = useState<SchedulerRun | null>(null);
  const [runFilters, setRunFilters] = useState({
    jobId: "0",
    status: "",
    triggerType: "",
  });
  const today = formatLocalDate(new Date());
  const [backfillForm, setBackfillForm] = useState({
    jobId: "",
    dateFrom: today,
    dateTo: today,
    symbols: "",
  });
  const [refreshForm, setRefreshForm] = useState({
    symbol: "",
    dataType: "all" as "quote" | "kline" | "news" | "all",
  });
  const [jobForm, setJobForm] = useState<SchedulerJobFormState>(() => defaultJobForm());

  const load = (jobId: string) =>
    Promise.all([
      schedulerStatus(),
      schedulerJobsList(),
      schedulerRunsList({ job_id: Number(jobId) || 0, limit: 50 }),
      schedulerJobTypes(),
      providersStatus(),
    ]).then(([status, jobs, runs, types, providers]) => ({
      status,
      jobs: jobs.items,
      runs: runs.items,
      types: types.items,
      providers: providers.items,
    }));

  useEffect(() => {
    let active = true;
    load(runFilters.jobId)
      .then((value) => {
        if (active) {
          setState(value);
          setLoadError(null);
        }
      })
      .catch((cause: unknown) => {
        if (active) {
          setLoadError(cause instanceof Error ? cause.message : "读取调度任务失败");
        }
      });
    return () => {
      active = false;
    };
  }, [runFilters.jobId]);

  const refresh = (jobId = runFilters.jobId) =>
    load(jobId).then((value) => {
      setState(value);
      setLoadError(null);
    });

  const runAction = (key: string, action: () => Promise<string>) => {
    setActionPending(key);
    setActionError(null);
    action()
      .then((message) => {
        setActionMessage(message);
      })
      .catch((cause: unknown) => {
        setActionError(cause instanceof Error ? cause.message : "调度操作失败");
      })
      .finally(() => {
        setActionPending(null);
      });
  };

  const runNow = (job: SchedulerJob) => {
    runAction(`run-${job.id}`, () =>
      schedulerJobsRunNow({
        id: job.id,
        target_date: today,
        ignore_trade_window: true,
      }).then((run) => refresh().then(() => `已入队 ${run.run_key}`)),
    );
  };

  const saveJob = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const validationError = validateJobForm(jobForm);
    if (validationError) {
      setActionError(validationError);
      return;
    }
    const payload = jobPayloadFromForm(jobForm);
    runAction("save-job", () =>
      schedulerJobsSave(payload).then((job) =>
        refresh().then(() => {
          setJobForm(defaultJobForm());
          return `${job.name || `任务 ${job.id}`} 已保存`;
        }),
      ),
    );
  };

  const editJob = (job: SchedulerJob) => {
    setJobForm(jobFormFromJob(job));
    setActionMessage(`正在编辑 ${job.name || `任务 ${job.id}`}`);
    setActionError(null);
  };

  const selectJobType = (cronType: string) => {
    const jobType = state?.types.find((item) => item.cron_type === cronType);
    setJobForm((current) => ({
      ...current,
      cronType,
      cronExpr: jobType?.default_cron_expr || current.cronExpr,
      tradeWindow: jobType?.default_window || current.tradeWindow,
      market: jobType?.market || current.market,
    }));
  };

  const setEnabled = (job: SchedulerJob, enabled: boolean) => {
    runAction(`enabled-${job.id}`, () =>
      schedulerJobsSetEnabled(job.id, enabled).then(() => refresh().then(() => `${job.name || `任务 ${job.id}`} 已${enabled ? "启用" : "停用"}`)),
    );
  };

  const deleteJob = (job: SchedulerJob) => {
    const name = job.name || `任务 ${job.id}`;
    if (!window.confirm(`确认删除 ${name}？`)) {
      return;
    }
    runAction(`delete-${job.id}`, () => schedulerJobsDelete(job.id).then(() => refresh().then(() => `${name} 已删除`)));
  };

  const changeRunJob = (jobId: string) => {
    setRunFilters((current) => ({ ...current, jobId }));
    setSelectedRun(null);
  };

  const viewRunDetail = (run: SchedulerRun) => {
    if (!run.id) {
      setSelectedRun(run);
      return;
    }
    setSelectedRun(null);
    runAction(`run-detail-${run.id}`, () =>
      schedulerRunsGet(run.id as number).then((detail) => {
        setSelectedRun(detail);
        return `已读取 ${detail.run_key}`;
      }),
    );
  };

  const submitBackfill = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const jobId = Number(backfillForm.jobId);
    const symbols = backfillForm.symbols
      .split(/[,\s]+/)
      .map((symbol) => symbol.trim())
      .filter(Boolean);
    const rangeError = validateBackfillRange(backfillForm.dateFrom, backfillForm.dateTo);
    if (!jobId) {
      setActionMessage(null);
      setActionError("请选择需要补偿的调度任务");
      return;
    }
    if (rangeError) {
      setActionMessage(null);
      setActionError(rangeError);
      return;
    }
    if (symbols.length > 30) {
      setActionMessage(null);
      setActionError("单次补偿最多支持 30 个 symbol");
      return;
    }
    runAction("backfill", () =>
      schedulerJobsBackfill({
        id: jobId,
        dateFrom: backfillForm.dateFrom,
        dateTo: backfillForm.dateTo,
        symbols,
      }).then((result) => refresh().then(() => `已生成 ${result.items.length} 条补偿执行记录`)),
    );
  };

  const submitRefreshSymbol = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const symbol = refreshForm.symbol.trim();
    if (!symbol) {
      setActionError("请填写需要刷新的股票代码");
      return;
    }
    runAction("refresh-symbol", () =>
      schedulerRefreshSymbol({
        symbol,
        data_type: refreshForm.dataType,
        target_date: today,
      }).then((result) => refresh().then(() => `已生成 ${result.items.length} 条单股刷新执行记录`)),
    );
  };

  if (loadError && !state) {
    return <Alert title="任务调度读取失败" description={loadError} type="error" showIcon />;
  }
  if (!state) {
    return <Alert title="正在读取任务调度" description={<Spin size="small" />} type="info" showIcon />;
  }

  return (
    <section className="flex flex-col gap-5">
      <div className="flex flex-col gap-1">
        <Typography.Title level={2} className="m-0">
          任务调度
        </Typography.Title>
        <Typography.Text type="secondary">数据刷新、启动补偿和手动补偿的本地执行状态。</Typography.Text>
      </div>
      {actionMessage ? <Alert title={actionMessage} type="success" showIcon /> : null}
      {actionError ? <Alert title={actionError} type="error" showIcon /> : null}
      {loadError ? <Alert title="任务调度刷新失败" description={loadError} type="warning" showIcon /> : null}
      <div className="grid gap-3 md:grid-cols-5">
        <Metric label="任务总数" value={state.status.jobs_total} />
        <Metric label="启用任务" value={state.status.jobs_enabled} />
        <Metric label="排队中" value={state.status.queued_runs} />
        <Metric label="运行中" value={state.status.running_runs} />
        <Metric label="失败记录" value={state.status.failed_runs} />
      </div>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            任务配置
          </Typography.Title>
        </div>
        <SchedulerJobEditor
          value={jobForm}
          types={state.types}
          actionPending={actionPending}
          onChange={setJobForm}
          onSelectType={selectJobType}
          onSubmit={saveJob}
          onReset={() => setJobForm(defaultJobForm())}
        />
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            调度任务
          </Typography.Title>
          <Typography.Text type="secondary">{state.types.length} 类任务类型</Typography.Text>
        </div>
        <SchedulerJobTable
          jobs={state.jobs}
          providers={state.providers}
          actionPending={actionPending}
          onRunNow={runNow}
          onEdit={editJob}
          onSetEnabled={setEnabled}
          onDelete={deleteJob}
        />
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            手动补偿
          </Typography.Title>
        </div>
        <SchedulerBackfillDialog value={backfillForm} jobs={state.jobs} actionPending={actionPending} onChange={setBackfillForm} onSubmit={submitBackfill} />
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            单股刷新
          </Typography.Title>
        </div>
        <form className="grid gap-3 px-4 py-4 md:grid-cols-[1.4fr_1fr_auto]" onSubmit={submitRefreshSymbol}>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">股票代码</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              placeholder="600000.SH"
              value={refreshForm.symbol}
              onChange={(event) => setRefreshForm((current) => ({ ...current, symbol: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">数据类型</span>
            <select
              className="h-8 rounded border border-slate-300 px-2"
              value={refreshForm.dataType}
              onChange={(event) => setRefreshForm((current) => ({ ...current, dataType: event.target.value as typeof current.dataType }))}
            >
              <option value="all">全部</option>
              <option value="quote">行情</option>
              <option value="kline">K 线</option>
              <option value="news">新闻</option>
            </select>
          </label>
          <Button className="self-end" htmlType="submit" loading={actionPending === "refresh-symbol"}>
            刷新单股
          </Button>
        </form>
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <SchedulerRunList
          jobs={state.jobs}
          runs={state.runs}
          filters={runFilters}
          selectedRun={selectedRun}
          actionPending={actionPending}
          onChangeJob={changeRunJob}
          onChangeStatus={(status) => setRunFilters((current) => ({ ...current, status }))}
          onChangeTriggerType={(triggerType) => setRunFilters((current) => ({ ...current, triggerType }))}
          onViewDetail={viewRunDetail}
        />
      </section>
    </section>
  );
}

function Metric(props: { label: string; value: number; suffix?: string }) {
  return (
    <div className="rounded-md border border-slate-200 bg-white px-4 py-3">
      <div className="text-xs text-slate-500">{props.label}</div>
      <div className="mt-1 text-2xl font-semibold text-slate-900">
        {formatNumber(props.value)}
        {props.suffix ?? ""}
      </div>
    </div>
  );
}

function formatLocalDate(value: Date) {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, "0");
  const day = String(value.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function uniqueValues(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort();
}

function validateBackfillRange(dateFrom: string, dateTo: string) {
  if (!dateFrom || !dateTo) {
    return "请选择补偿日期范围";
  }
  const start = new Date(`${dateFrom}T00:00:00`);
  const end = new Date(`${dateTo}T00:00:00`);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
    return "补偿日期格式不正确";
  }
  if (start > end) {
    return "开始日期不能晚于结束日期";
  }
  const oneDayMs = 24 * 60 * 60 * 1000;
  if ((end.getTime() - start.getTime()) / oneDayMs > 29) {
    return "单次补偿最多覆盖 30 个自然日";
  }
  return "";
}

function defaultJobForm(): SchedulerJobFormState {
  return {
    id: 0,
    name: "",
    cronType: "cn_a_share_quote_refresh",
    cronExpr: "30 9 * * 1-5",
    enabled: true,
    market: "CN",
    timezone: "Asia/Shanghai",
    tradeWindow: "09:30-15:00",
    symbols: "",
    paramsJSON: "{}",
    catchupEnabled: true,
    catchupMaxDays: "5",
    timeoutSeconds: "120",
  };
}

function jobFormFromJob(job: SchedulerJob): SchedulerJobFormState {
  return {
    id: job.id,
    name: job.name || "",
    cronType: job.cron_type,
    cronExpr: job.cron_expr || "",
    enabled: Boolean(job.enabled),
    market: job.market || "CN",
    timezone: job.timezone || "Asia/Shanghai",
    tradeWindow: job.trade_window || "",
    symbols: symbolsFromScopeJSON(job.scope_json || ""),
    paramsJSON: job.params_json || "{}",
    catchupEnabled: Boolean(job.catchup_enabled),
    catchupMaxDays: String(job.catchup_max_days ?? 5),
    timeoutSeconds: String(job.timeout_seconds ?? 120),
  };
}

function validateJobForm(form: SchedulerJobFormState) {
  if (!form.name.trim()) {
    return "请填写任务名称";
  }
  if (!form.cronType.trim()) {
    return "请选择任务类型";
  }
  if (!form.cronExpr.trim()) {
    return "请填写 cron 表达式";
  }
  const params = parseJSON(form.paramsJSON);
  if (!params.ok) {
    return "参数 JSON 格式不正确";
  }
  const catchupMaxDays = Number(form.catchupMaxDays);
  if (!Number.isInteger(catchupMaxDays) || catchupMaxDays < 0 || catchupMaxDays > 30) {
    return "补偿天数必须是 0 到 30 的整数";
  }
  const timeoutSeconds = Number(form.timeoutSeconds);
  if (!Number.isInteger(timeoutSeconds) || timeoutSeconds <= 0 || timeoutSeconds > 3600) {
    return "超时秒数必须是 1 到 3600 的整数";
  }
  const symbols = splitSymbols(form.symbols);
  if (symbols.length > 30) {
    return "单个任务最多配置 30 个 symbol";
  }
  return "";
}

function jobPayloadFromForm(form: SchedulerJobFormState): SchedulerJobPayload {
  const symbols = splitSymbols(form.symbols);
  const params = parseJSON(form.paramsJSON);
  return {
    id: form.id,
    name: form.name.trim(),
    cron_type: form.cronType.trim(),
    cron_expr: form.cronExpr.trim(),
    enabled: form.enabled,
    market: form.market.trim() || "CN",
    timezone: form.timezone.trim() || "Asia/Shanghai",
    trade_window: form.tradeWindow.trim(),
    scope_json: JSON.stringify(symbols.length > 0 ? { symbols } : { market: form.market.trim() || "CN" }),
    params_json: JSON.stringify(params.value),
    catchup_enabled: form.catchupEnabled,
    catchup_max_days: Number(form.catchupMaxDays),
    timeout_seconds: Number(form.timeoutSeconds),
  };
}

function splitSymbols(value: string) {
  return value
    .split(/[,\s]+/)
    .map((symbol) => symbol.trim())
    .filter(Boolean);
}

function symbolsFromScopeJSON(scopeJSON: string) {
  const parsed = parseJSON(scopeJSON);
  if (!parsed.ok || typeof parsed.value !== "object" || parsed.value === null || !("symbols" in parsed.value)) {
    return "";
  }
  const symbols = (parsed.value as { symbols?: unknown }).symbols;
  return Array.isArray(symbols) ? symbols.filter((symbol): symbol is string => typeof symbol === "string").join(", ") : "";
}

function parseJSON(value: string): { ok: true; value: unknown } | { ok: false; value: null } {
  try {
    return { ok: true, value: value.trim() ? JSON.parse(value) : {} };
  } catch {
    return { ok: false, value: null };
  }
}

function DashboardList(props: { title: string; emptyText: string; children: ReactNode }) {
  const items = Array.isArray(props.children) ? props.children.filter(Boolean) : props.children ? [props.children] : [];
  return (
    <section className="rounded-md border border-slate-200 bg-white">
      <div className="border-b border-slate-200 px-4 py-3">
        <Typography.Title level={4} className="m-0">
          {props.title}
        </Typography.Title>
      </div>
      {items.length > 0 ? props.children : <div className="px-4 py-8 text-sm text-slate-500">{props.emptyText}</div>}
    </section>
  );
}
