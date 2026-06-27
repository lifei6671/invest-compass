import { Alert, Spin } from "antd";
import { useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import {
  marketIndicators,
  marketKline,
  marketQuote,
  stockSearch,
  stockProfile,
  type MarketIndicatorsResult,
  type MarketKlineItem,
  type MarketQuote,
  type StockSearchResult,
  type StockProfile,
} from "../../services/coreClient";
import { ChartControlBar } from "./components/ChartControlBar";
import { ChartHeaderBar } from "./components/ChartHeaderBar";
import { KlineMultiPaneChart } from "./components/KlineMultiPaneChart";
import { QuoteSummaryStrip } from "./components/QuoteSummaryStrip";
import type { AdjustType, ChartPeriod, IndicatorKey, IndicatorSeries, KlineBar, StockChartQuote } from "./types";
import { useAutoRefresh } from "../../hooks/useAutoRefresh";

const defaultIndicators: IndicatorKey[] = ["MA", "VOL", "MACD", "KDJ"];
const indicatorRequestKeys = ["ma", "rsi", "macd", "kdj", "boll"];
const defaultKlineLimit = 120;
const minuteKlineLimit = 240;
const indexDisplayNames: Record<string, string> = {
  "SH:000001": "上证指数",
  "SH:000016": "上证50",
  "SH:000688": "科创50",
  "SH:000300": "沪深300",
  "SH:000905": "中证500",
  "SH:000852": "中证1000",
  "SZ:399001": "深证成指",
  "SZ:399006": "创业板指",
  "SZ:399005": "中小100",
};

type FullscreenChartState = {
  quote: StockChartQuote;
  bars: KlineBar[];
  indicators: IndicatorSeries;
};

export function FullscreenKlinePage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const location = useLocation();
  const symbol = (searchParams.get("symbol") ?? "").trim();
  const [activePeriod, setActivePeriod] = useState<ChartPeriod>(() => parseChartPeriod(searchParams.get("period")));
  const [activeAdjust, setActiveAdjust] = useState<AdjustType>(() => parseAdjustType(searchParams.get("adjust")));
  const [activeIndicators, setActiveIndicators] = useState<IndicatorKey[]>(defaultIndicators);
  const [state, setState] = useState<FullscreenChartState | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [refreshVersion, setRefreshVersion] = useState(0);
  const currentSymbolRef = useRef<string | null>(null);
  const currentStateRef = useRef<FullscreenChartState | null>(null);
  const marketPeriod = toMarketKlinePeriod(activePeriod);
  const klineLimit = toKlineRequestLimit(activePeriod);

  useEffect(() => {
    setActivePeriod(parseChartPeriod(searchParams.get("period")));
    setActiveAdjust(parseAdjustType(searchParams.get("adjust")));
  }, [searchParams]);

  useEffect(() => {
    if (!symbol) {
      currentSymbolRef.current = null;
      currentStateRef.current = null;
      setState(null);
      setLoadError(null);
      setLoading(false);
      return;
    }
    const symbolChanged = currentSymbolRef.current !== symbol;
    currentSymbolRef.current = symbol;
    if (symbolChanged) {
      currentStateRef.current = null;
      setState(null);
    }
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    loadFullscreenChartState(symbol, activePeriod, marketPeriod, activeAdjust, klineLimit)
      .then((nextState) => {
        if (!cancelled) {
          currentStateRef.current = nextState;
          setState(nextState);
        }
      })
      .catch((error: Error) => {
        if (!cancelled) {
          const hasReusableState = currentStateRef.current !== null && !symbolChanged;
          if (hasReusableState) {
            setLoadError(error.message || "全屏行情数据读取失败");
            return;
          }
          if (symbolChanged) {
            currentStateRef.current = null;
            setState(null);
          }
          setLoadError(error.message || "全屏行情数据读取失败");
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
  }, [activeAdjust, activePeriod, klineLimit, marketPeriod, refreshVersion, symbol]);

  useAutoRefresh(() => {
    if (symbol) {
      setRefreshVersion((version) => version + 1);
    }
  });

  const toggleIndicator = (indicator: IndicatorKey) => {
    setActiveIndicators((current) =>
      current.includes(indicator) ? current.filter((item) => item !== indicator) : [...current, indicator],
    );
  };

  const changePeriod = (period: ChartPeriod) => {
    setActivePeriod(period);
  };

  const routeState = location.state as { from?: string } | null;
  const sourcePath = routeState?.from && routeState.from !== `${location.pathname}${location.search}` ? routeState.from : "/";
  const backToSource = () => {
    navigate(sourcePath);
  };
  const navigateToSearchResult = (stock: StockSearchResult) => {
    navigate(chartKlinePath(stock.symbol, "day", "qfq"), { state: { from: sourcePath } });
  };

  const emptyIndicators = useMemo(() => emptyIndicatorSeries(0), []);

  return (
    <main className="flex h-screen min-w-0 flex-col overflow-hidden bg-[#f6f8fb]" aria-label="全屏K线趋势图页">
      <h1 className="sr-only">全屏 K 线趋势图页</h1>
      {!symbol ? (
        <div className="flex h-full items-center justify-center p-6">
          <Alert
            className="w-full max-w-xl rounded-xl border-[#e5eaf3]"
            title="未选择股票"
            description="请从个股详情页点击全屏，或在地址中携带 symbol 参数后查看全屏行情。"
            type="info"
            showIcon
          />
        </div>
      ) : (
        <>
          {state ? (
            <ChartHeaderBar quote={state.quote} onBack={backToSource} onSearch={stockSearch} onSearchSelect={navigateToSearchResult} />
          ) : (
            <ChartHeaderBar quote={loadingPlaceholderQuote(symbol)} onBack={backToSource} onSearch={stockSearch} onSearchSelect={navigateToSearchResult} />
          )}
          {state ? <QuoteSummaryStrip quote={state.quote} /> : <QuoteSummaryStrip quote={loadingPlaceholderQuote(symbol)} />}
          {loadError ? (
            <Alert className="mx-4 mt-3 shrink-0 rounded-lg border-[#e5eaf3]" title="全屏行情数据读取失败" description={loadError} type="error" showIcon />
          ) : null}
          <ChartControlBar
            activePeriod={activePeriod}
            activeAdjust={activeAdjust}
            activeIndicators={activeIndicators}
            onPeriodChange={changePeriod}
            onAdjustChange={setActiveAdjust}
            onIndicatorChange={toggleIndicator}
          />
          <div className="relative flex min-h-0 flex-1 flex-col">
            {loading ? (
              <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center bg-white/55">
                <Spin description="正在读取行情数据" />
              </div>
            ) : null}
            <KlineMultiPaneChart
              bars={state?.bars ?? []}
              indicators={state?.indicators ?? emptyIndicators}
              quote={state?.quote ?? loadingPlaceholderQuote(symbol)}
              activeIndicators={activeIndicators}
              activePeriod={activePeriod}
            />
          </div>
        </>
      )}
    </main>
  );
}

async function loadFullscreenChartState(
  symbol: string,
  displayPeriod: ChartPeriod,
  sourcePeriod: string,
  adjust: AdjustType,
  limit: number,
): Promise<FullscreenChartState> {
  // 指标接口只增强图表说明，失败时保留真实 K 线并交给 klinecharts 基于 OHLCV 本地计算。
  const indicatorsPromise: Promise<MarketIndicatorsResult | null> = shouldRequestBackendIndicators(displayPeriod)
    ? marketIndicators({
        symbol,
        period: sourcePeriod,
        adjust,
        limit,
        indicators: indicatorRequestKeys,
      }).catch(() => null)
    : Promise.resolve<MarketIndicatorsResult | null>(null);
  const profilePromise = stockProfile(symbol).catch((): StockProfile | null => null);
  const [quote, profile, kline, indicators] = await Promise.all([
    marketQuote(symbol),
    profilePromise,
    marketKline({ symbol, period: sourcePeriod, adjust, limit }),
    indicatorsPromise,
  ]);
  const rawBars = toKlineBars(kline.items);
  const bars = displayPeriod === "minute" ? filterLatestTradingDayBars(rawBars) : rawBars;
  return {
    quote: toStockChartQuote(symbol, quote, profile),
    bars,
    indicators: indicators ? toIndicatorSeries(bars, indicators) : toIndicatorSeries(bars, emptyMarketIndicators(symbol, displayPeriod, adjust)),
  };
}

function emptyMarketIndicators(symbol: string, period: ChartPeriod, adjust: AdjustType): MarketIndicatorsResult {
  return { symbol, period, adjust, indicators: {} };
}

function parseChartPeriod(value: string | null): ChartPeriod {
  switch (value) {
    case "minute":
    case "5m":
    case "15m":
    case "30m":
    case "60m":
    case "week":
    case "month":
    case "quarter":
    case "year":
      return value;
    case "day":
    default:
      return "day";
  }
}

function parseAdjustType(value: string | null): AdjustType {
  switch (value) {
    case "hfq":
    case "none":
      return value;
    case "qfq":
    default:
      return "qfq";
  }
}

function chartKlinePath(symbol: string, period: ChartPeriod, adjust: AdjustType) {
  return `/chart/kline?symbol=${encodeURIComponent(symbol)}&period=${period}&adjust=${adjust}`;
}

function toMarketKlinePeriod(period: ChartPeriod): string {
  switch (period) {
    case "minute":
      return "1m";
    case "5m":
    case "15m":
    case "30m":
    case "60m":
    case "week":
    case "month":
    case "quarter":
    case "year":
      return period;
    case "day":
    default:
      return "day";
  }
}

function toKlineRequestLimit(period: ChartPeriod): number {
  if (period === "minute") {
    return minuteKlineLimit;
  }
  return defaultKlineLimit;
}

function shouldRequestBackendIndicators(period: ChartPeriod) {
  return period !== undefined;
}

function toStockChartQuote(symbol: string, quote: MarketQuote, profile: StockProfile | null): StockChartQuote {
  const resolvedSymbol = quote.symbol || profile?.symbol || symbol;
  const displayName = profile?.name || resolveIndexDisplayName(resolvedSymbol) || resolvedSymbol;
  return {
    name: displayName,
    code: profile?.code || stockCodeFromSymbol(resolvedSymbol) || resolvedSymbol,
    symbol: resolvedSymbol,
    price: quote.price,
    changeAmount: quote.change_amount ?? 0,
    changePercent: quote.change_percent ?? 0,
    open: quote.open ?? quote.price,
    high: quote.high ?? quote.price,
    low: quote.low ?? quote.price,
    previousClose: quote.pre_close ?? quote.price,
    volumeText: typeof quote.volume === "number" ? formatVolumeText(quote.volume) : "暂无",
    amountText: typeof quote.amount === "number" ? formatAmountText(quote.amount) : "暂无",
    turnoverRateText: typeof quote.turnover_rate === "number" ? `${formatNumber(quote.turnover_rate)}%` : "暂无",
    status: profile?.status === "LISTED" || !profile?.status ? "行情数据" : profile.status,
    updateTime: formatQuoteTime(quote.quote_time),
  };
}

function loadingPlaceholderQuote(symbol: string): StockChartQuote {
  return {
    name: resolveIndexDisplayName(symbol) || symbol,
    code: stockCodeFromSymbol(symbol) || symbol,
    symbol,
    price: 0,
    changeAmount: 0,
    changePercent: 0,
    open: 0,
    high: 0,
    low: 0,
    previousClose: 0,
    volumeText: "暂无",
    amountText: "暂无",
    turnoverRateText: "暂无",
    status: "加载中",
    updateTime: "正在读取",
  };
}

function toKlineBars(items: MarketKlineItem[]): KlineBar[] {
  return items.map((item) => ({
    date: item.trade_date,
    open: item.open,
    close: item.close,
    low: item.low,
    high: item.high,
    volume: item.volume ?? 0,
    amount: item.amount,
  }));
}

export function filterLatestTradingDayBars(bars: KlineBar[]): KlineBar[] {
  if (bars.length === 0) {
    return [];
  }
  const latest = bars.reduce((current, next) => (tradeDateTime(next.date) >= tradeDateTime(current.date) ? next : current), bars[0]);
  const latestDay = tradeDateDayKey(latest.date);
  return bars.filter((bar) => tradeDateDayKey(bar.date) === latestDay);
}

function tradeDateDayKey(value: string) {
  const normalized = value.trim();
  const compact = /^(\d{4})(\d{2})(\d{2})/.exec(normalized);
  if (compact) {
    const [, year, month, day] = compact;
    return `${year}-${month}-${day}`;
  }
  const dashed = /^(\d{4}-\d{2}-\d{2})/.exec(normalized);
  if (dashed) {
    return dashed[1];
  }
  const timestamp = tradeDateTime(value);
  if (timestamp > 0) {
    const date = new Date(timestamp);
    const year = date.getFullYear();
    const month = `${date.getMonth() + 1}`.padStart(2, "0");
    const day = `${date.getDate()}`.padStart(2, "0");
    return `${year}-${month}-${day}`;
  }
  return normalized;
}

function tradeDateTime(value: string) {
  const normalized = value.trim();
  const compact = /^(\d{4})(\d{2})(\d{2})(?:\s+(\d{2}):?(\d{2}))?$/.exec(normalized);
  if (compact) {
    const [, year, month, day, hour = "00", minute = "00"] = compact;
    return new Date(`${year}-${month}-${day}T${hour}:${minute}:00+08:00`).getTime();
  }
  if (/^\d{4}-\d{2}-\d{2}$/.test(normalized)) {
    return new Date(`${normalized}T00:00:00+08:00`).getTime();
  }
  if (/^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}/.test(normalized)) {
    return new Date(`${normalized.slice(0, 10)}T${normalized.slice(11, 16)}:00+08:00`).getTime();
  }
  const timestamp = new Date(normalized).getTime();
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function toIndicatorSeries(bars: KlineBar[], result: MarketIndicatorsResult): IndicatorSeries {
  const closes = bars.map((bar) => bar.close);
  const volumes = bars.map((bar) => bar.volume);
  const length = bars.length;
  return {
    ma5: seriesFromIndicators(result.indicators, ["ma.ma5", "ma5"], length, movingAverage(closes, 5)),
    ma10: seriesFromIndicators(result.indicators, ["ma.ma10", "ma10"], length, movingAverage(closes, 10)),
    ma20: seriesFromIndicators(result.indicators, ["ma.ma20", "ma20"], length, movingAverage(closes, 20)),
    ma60: seriesFromIndicators(result.indicators, ["ma.ma60", "ma60"], length, movingAverage(closes, 60)),
    bollUp: seriesFromIndicators(result.indicators, ["boll.upper", "boll.up", "boll_up"], length),
    bollMid: seriesFromIndicators(result.indicators, ["boll.middle", "boll.mid", "boll_mid"], length),
    bollDn: seriesFromIndicators(result.indicators, ["boll.lower", "boll.dn", "boll_lower", "boll_dn"], length),
    volMa5: movingAverage(volumes, 5),
    volMa10: movingAverage(volumes, 10),
    macdDif: seriesFromIndicators(result.indicators, ["macd.dif", "macd_dif"], length),
    macdDea: seriesFromIndicators(result.indicators, ["macd.dea", "macd_dea"], length),
    macdHist: seriesFromIndicators(result.indicators, ["macd.bar", "macd.macd", "macd_bar", "macd_hist"], length),
    kdjK: seriesFromIndicators(result.indicators, ["kdj.k", "kdj_k"], length),
    kdjD: seriesFromIndicators(result.indicators, ["kdj.d", "kdj_d"], length),
    kdjJ: seriesFromIndicators(result.indicators, ["kdj.j", "kdj_j"], length),
    rsi6: seriesFromIndicators(result.indicators, ["rsi.rsi6", "rsi6"], length),
  };
}

function seriesFromIndicators(
  indicators: Record<string, unknown>,
  paths: string[],
  expectedLength: number,
  fallback: Array<number | null> = emptySeries(expectedLength),
): Array<number | null> {
  for (const path of paths) {
    const value = readPath(indicators, path);
    if (Array.isArray(value)) {
      return alignSeries(value.map((item) => numericOrNull(item)), expectedLength);
    }
    const numeric = numericOrNull(value);
    if (numeric !== null) {
      return alignSeries([numeric], expectedLength);
    }
  }
  return fallback;
}

function readPath(source: Record<string, unknown>, path: string): unknown {
  return path.split(".").reduce<unknown>((current, key) => {
    if (!current || typeof current !== "object") {
      return undefined;
    }
    return (current as Record<string, unknown>)[key];
  }, source);
}

function alignSeries(values: Array<number | null>, expectedLength: number): Array<number | null> {
  if (values.length >= expectedLength) {
    return values.slice(values.length - expectedLength);
  }
  return [...emptySeries(expectedLength - values.length), ...values];
}

function movingAverage(values: number[], windowSize: number): Array<number | null> {
  return values.map((_, index) => {
    if (index + 1 < windowSize) {
      return null;
    }
    const windowValues = values.slice(index + 1 - windowSize, index + 1);
    const sum = windowValues.reduce((total, value) => total + value, 0);
    return round2(sum / windowSize);
  });
}

function emptyIndicatorSeries(length: number): IndicatorSeries {
  return {
    ma5: emptySeries(length),
    ma10: emptySeries(length),
    ma20: emptySeries(length),
    ma60: emptySeries(length),
    bollUp: emptySeries(length),
    bollMid: emptySeries(length),
    bollDn: emptySeries(length),
    volMa5: emptySeries(length),
    volMa10: emptySeries(length),
    macdDif: emptySeries(length),
    macdDea: emptySeries(length),
    macdHist: emptySeries(length),
    kdjK: emptySeries(length),
    kdjD: emptySeries(length),
    kdjJ: emptySeries(length),
    rsi6: emptySeries(length),
  };
}

function emptySeries(length: number): Array<number | null> {
  return Array.from({ length }, () => null);
}

function numericOrNull(value: unknown): number | null {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return null;
  }
  return value;
}

function formatVolumeText(value: number) {
  return `${formatNumber(value / 10000)}万手`;
}

function formatAmountText(value: number) {
  return `${formatNumber(value / 100000000)}亿`;
}

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}

function formatQuoteTime(value: string | undefined) {
  if (!value) {
    return "后端未提供";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  const hours = `${date.getHours()}`.padStart(2, "0");
  const minutes = `${date.getMinutes()}`.padStart(2, "0");
  const seconds = `${date.getSeconds()}`.padStart(2, "0");
  return `${month}-${day} ${hours}:${minutes}:${seconds} 北京时间`;
}

function stockCodeFromSymbol(symbol: string) {
  const parts = symbol.split(/[.:]/).filter(Boolean);
  return parts.find((part) => /^\d{5,6}$/.test(part)) ?? "";
}

export function resolveIndexDisplayName(symbol: string) {
  const parts = symbol.toUpperCase().split(/[.:]/).filter(Boolean);
  const code = parts.find((part) => /^\d{5,6}$/.test(part));
  const exchange = parts.find((part) => part === "SH" || part === "SSE" || part === "SS")
    ? "SH"
    : parts.find((part) => part === "SZ" || part === "SZSE" || part === "SHE")
      ? "SZ"
      : "";
  return code && exchange ? indexDisplayNames[`${exchange}:${code}`] ?? "" : "";
}

function round2(value: number) {
  return Number(value.toFixed(2));
}
