import {
  dispose,
  init,
  type ActionCallback,
  type Chart,
  type Crosshair,
  type DeepPartial,
  type Indicator as KLineChartIndicator,
  type IndicatorFigure,
  type KLineData,
  type Layout,
  type LayoutPaneContentChild,
  type Period,
  type Point,
  type Styles,
  type TooltipLegend,
} from "klinecharts";
import { useEffect, useMemo, useRef, useState } from "react";
import { APP_FONT } from "../../../styles/fonts";
import { goStockCandleIndicators, goStockPaneIndicators, registerGoStockIndicators } from "../goStockIndicators";
import type { ChartPeriod, IndicatorKey, IndicatorSeries, KlineBar, StockChartQuote } from "../types";

type KlineMultiPaneChartProps = {
  bars: KlineBar[];
  indicators: IndicatorSeries;
  quote: StockChartQuote;
  activeIndicators: IndicatorKey[];
  activePeriod: ChartPeriod;
};

const upColor = "#ff4d4f";
const downColor = "#16a34a";
const supportedPaneIndicators = goStockPaneIndicators;
const supportedCandleIndicators = goStockCandleIndicators;
const indicatorPaneHeight = 104;
const xAxisPaneHeight = 28;
const dashedLine = "dashed" as const;
export const klineChartLocale = "zh-CN";

registerGoStockIndicators();

type ChartWithCrosshair = Chart & { getCrosshair?: () => Crosshair };

type IndicatorHoverPoint = {
  x: number;
  containerWidth: number;
  paneId?: string;
  valuesByIndicator: Partial<Record<IndicatorKey, IndicatorHoverValue[]>>;
};

type IndicatorHoverValue = {
  title: string;
  value: string;
  color: string;
};

export function KlineMultiPaneChart(props: KlineMultiPaneChartProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [indicatorHoverPoint, setIndicatorHoverPoint] = useState<IndicatorHoverPoint | null>(null);
  const chartData = useMemo(() => toKLineChartData(props.bars, props.indicators), [props.bars, props.indicators]);
  const period = useMemo(() => toKLineChartPeriod(props.activePeriod), [props.activePeriod]);
  const layout = useMemo(() => buildKLineChartLayout(props.activeIndicators, props.activePeriod), [props.activeIndicators, props.activePeriod]);
  const chartStyles = useMemo(() => buildKLineChartStyles(props.activePeriod), [props.activePeriod]);
  const paneIndicators = useMemo(() => activePaneIndicators(props.activeIndicators), [props.activeIndicators]);

  useEffect(() => {
    if (!containerRef.current || isJSDOM()) {
      return undefined;
    }
    const chart = init(containerRef.current, {
      locale: klineChartLocale,
      timezone: "Asia/Shanghai",
      styles: chartStyles,
      formatter: {
        formatDate: ({ timestamp, type }) => formatChartDate(timestamp, type),
        formatBigNumber: formatBigNumber,
      },
      layout,
    });
    if (!chart) {
      return undefined;
    }
    chart.setSymbol({ ticker: props.quote.symbol || props.quote.code, pricePrecision: 2, volumePrecision: 0 });
    chart.setPeriod(period);
    chart.setDataLoader({
      getBars: ({ callback }) => {
        callback(chartData, false);
      },
    });
    chart.setBarSpace(props.activePeriod === "minute" ? 7 : 9);
    chart.setOffsetRightDistance(12);
    chart.setScrollEnabled(true);
    chart.setZoomEnabled(true);
    chart.resize();
    const handleCrosshairChange: ActionCallback = (payload) => {
      const crosshair = resolveCrosshairSnapshot(chart, payload, chartData.length);
      if (!crosshair || !containerRef.current) {
        setIndicatorHoverPoint(null);
        return;
      }
      const containerWidth = containerRef.current.clientWidth;
      const valuesByIndicator = buildIndicatorHoverTooltipData(chart.getIndicators(), crosshair.dataIndex);
      setIndicatorHoverPoint((current) => {
        if (
          current?.x === crosshair.x &&
          current.containerWidth === containerWidth &&
          current.paneId === crosshair.paneId &&
          current.valuesByIndicator === valuesByIndicator
        ) {
          return current;
        }
        return { x: crosshair.x, containerWidth, paneId: crosshair.paneId, valuesByIndicator };
      });
    };
    chart.subscribeAction("onCrosshairChange", handleCrosshairChange);
    return () => {
      chart.unsubscribeAction("onCrosshairChange", handleCrosshairChange);
      dispose(chart);
    };
  }, [chartData, chartStyles, layout, period, props.activePeriod, props.quote.code, props.quote.symbol]);

  return (
    <section className="mx-4 mt-2 min-h-0 flex-1 rounded-t-lg border border-[#e5eaf3] border-b-0 bg-white px-3 pb-2 pt-2">
      <div
        className="relative h-full min-h-[520px] overflow-hidden rounded-lg bg-white"
        onMouseLeave={() => setIndicatorHoverPoint(null)}
      >
        <IndicatorHoverTooltips indicators={paneIndicators} hoverPoint={indicatorHoverPoint} />
        <div ref={containerRef} aria-label="全屏K线趋势图" className="h-full w-full" />
      </div>
    </section>
  );
}

function IndicatorHoverTooltips(props: { indicators: IndicatorKey[]; hoverPoint: IndicatorHoverPoint | null }) {
  if (!props.hoverPoint) {
    return null;
  }
  const tooltipWidth = 248;
  const left = clamp(props.hoverPoint.x + 12, 16, Math.max(16, props.hoverPoint.containerWidth - tooltipWidth - 64));
  return (
    <>
      {props.indicators.map((indicator, index) => {
        const values = props.hoverPoint?.valuesByIndicator[indicator] ?? [];
        if (values.length === 0) {
          return null;
        }
        const bottom = xAxisPaneHeight + (props.indicators.length - index - 1) * indicatorPaneHeight;
        return (
          <div
            key={indicator}
            className="pointer-events-none absolute z-20 flex max-w-[248px] items-center gap-2 rounded-md border border-[#dbe5f2] bg-white/95 px-2 py-1 text-[12px] leading-5 shadow-[0_8px_20px_rgba(15,23,42,0.08)]"
            style={{ left, bottom: bottom + indicatorPaneHeight - 32 }}
          >
            <span className="font-semibold text-[#374151]">{formatIndicatorDisplayName(indicator)}</span>
            {values.map((item) => (
              <span key={`${indicator}-${item.title}`} className="whitespace-nowrap font-medium" style={{ color: item.color }}>
                {item.title}{item.value}
              </span>
            ))}
          </div>
        );
      })}
    </>
  );
}

export function toKLineChartData(bars: KlineBar[], indicators?: IndicatorSeries): KLineData[] {
  return bars.map((bar, index) => ({
    timestamp: tradeDateTimestamp(bar.date),
    open: bar.open,
    high: bar.high,
    low: bar.low,
    close: bar.close,
    volume: bar.volume,
    turnover: bar.amount,
    ma5Text: formatPriceIndicator(indicators?.ma5[index]),
    ma10Text: formatPriceIndicator(indicators?.ma10[index]),
    ma20Text: formatPriceIndicator(indicators?.ma20[index]),
    ma60Text: formatPriceIndicator(indicators?.ma60[index]),
    bollUpText: formatPriceIndicator(indicators?.bollUp[index]),
    bollMidText: formatPriceIndicator(indicators?.bollMid[index]),
    bollDnText: formatPriceIndicator(indicators?.bollDn[index]),
    volMa5Text: formatVolumeIndicator(indicators?.volMa5[index]),
    volMa10Text: formatVolumeIndicator(indicators?.volMa10[index]),
    macdDifText: formatOscillatorIndicator(indicators?.macdDif[index]),
    macdDeaText: formatOscillatorIndicator(indicators?.macdDea[index]),
    macdHistText: formatOscillatorIndicator(indicators?.macdHist[index]),
    kdjKText: formatOscillatorIndicator(indicators?.kdjK[index]),
    kdjDText: formatOscillatorIndicator(indicators?.kdjD[index]),
    kdjJText: formatOscillatorIndicator(indicators?.kdjJ[index]),
    rsi6Text: formatOscillatorIndicator(indicators?.rsi6[index]),
  }));
}

export function toKLineChartPeriod(period: ChartPeriod): Period {
  switch (period) {
    case "minute":
      return { type: "minute", span: 1 };
    case "5m":
      return { type: "minute", span: 5 };
    case "15m":
      return { type: "minute", span: 15 };
    case "30m":
      return { type: "minute", span: 30 };
    case "60m":
      return { type: "minute", span: 60 };
    case "week":
      return { type: "week", span: 1 };
    case "month":
      return { type: "month", span: 1 };
    case "quarter":
      return { type: "month", span: 3 };
    case "year":
      return { type: "year", span: 1 };
    case "day":
    default:
      return { type: "day", span: 1 };
  }
}

export function buildKLineChartLayout(activeIndicators: IndicatorKey[], activePeriod: ChartPeriod = "day"): Layout {
  const candleContent = activePeriod === "minute"
    ? [{ name: "MA", calcParams: [5, 10, 20] }]
    : activeIndicators
      .filter((indicator) => supportedCandleIndicators.includes(indicator))
      .map((indicator) => toCandleIndicatorContent(indicator));
  const panes: Layout["panes"] = [
    {
      type: "candle",
      content: candleContent,
      options: { id: "candle", minHeight: 320, dragEnabled: false },
    },
    ...activePaneIndicators(activeIndicators).map((indicator) => ({
      type: "indicator" as const,
      content: [toPaneIndicatorContent(indicator)],
      options: { id: `indicator-${indicator.toLowerCase()}`, height: indicatorPaneHeight, minHeight: 86, dragEnabled: false },
    })),
    { type: "xAxis", options: { id: "x-axis", height: xAxisPaneHeight, dragEnabled: false } },
  ];
  return {
    basicParams: {
      yAxisPosition: "right",
      yAxisInside: false,
      barSpaceLimitMin: 3,
      barSpaceLimitMax: 22,
      paneMinHeight: 72,
    },
    panes,
  };
}

function activePaneIndicators(activeIndicators: IndicatorKey[]): IndicatorKey[] {
  return supportedPaneIndicators.filter((indicator) => activeIndicators.includes(indicator));
}

function toCandleIndicatorContent(indicator: IndicatorKey): LayoutPaneContentChild {
  if (indicator === "MA") {
    return { name: "MA", calcParams: [5, 10, 20, 60] };
  }
  if (indicator === "EMA") {
    return { name: "EMA", calcParams: [5, 10, 20, 60] };
  }
  return indicator;
}

function toPaneIndicatorContent(indicator: IndicatorKey): LayoutPaneContentChild {
  if (indicator === "VOL") {
    return { name: "VOL", calcParams: [5, 10] };
  }
  if (indicator === "DMI") {
    return "ADX";
  }
  return indicator;
}

function formatPriceIndicator(value: number | null | undefined) {
  return typeof value === "number" && Number.isFinite(value) ? value.toFixed(2) : "--";
}

function formatVolumeIndicator(value: number | null | undefined) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return "--";
  }
  if (Math.abs(value) >= 100000000) {
    return `${(value / 100000000).toFixed(2)}亿`;
  }
  if (Math.abs(value) >= 10000) {
    return `${(value / 10000).toFixed(2)}万`;
  }
  return value.toFixed(0);
}

function formatOscillatorIndicator(value: number | null | undefined) {
  return typeof value === "number" && Number.isFinite(value) ? value.toFixed(4) : "--";
}

export function buildIndicatorHoverTooltipData(
  indicators: Array<Pick<KLineChartIndicator, "name" | "precision" | "shouldFormatBigNumber" | "figures" | "result">>,
  dataIndex: number,
): Partial<Record<IndicatorKey, IndicatorHoverValue[]>> {
  return indicators.reduce<Partial<Record<IndicatorKey, IndicatorHoverValue[]>>>((result, indicator) => {
    const indicatorKey = toIndicatorKey(indicator.name);
    if (!indicatorKey || !supportedPaneIndicators.includes(indicatorKey)) {
      return result;
    }
    const current = indicator.result[dataIndex];
    if (!isRecord(current)) {
      return result;
    }
    const colors = indicatorHoverColors[indicatorKey] ?? [];
    const values = indicator.figures.flatMap((figure, index) => {
      const title = normalizeIndicatorTitle(figure);
      if (!title) {
        return [];
      }
      const value = current[figure.key];
      return [
        {
          title,
          value: formatPaneTooltipValue(value, indicator.precision, indicator.shouldFormatBigNumber),
          color: colors[index] ?? "#64748b",
        },
      ];
    });
    if (values.length > 0) {
      result[indicatorKey] = values;
    }
    return result;
  }, {});
}

const indicatorHoverColors: Partial<Record<IndicatorKey, string[]>> = {
  VOL: ["#f97316", "#a855f7", "#fb7185"],
  MACD: ["#f97316", "#a855f7", "#fb7185"],
  KDJ: ["#f97316", "#a855f7", "#3b82f6"],
  RSI: ["#3b82f6", "#f97316", "#a855f7"],
  WR: ["#3b82f6", "#f97316", "#a855f7"],
  OBV: ["#10b981", "#3b82f6"],
  CCI: ["#f97316"],
  AO: ["#10b981"],
  TRIX: ["#f97316", "#a855f7"],
  ROC: ["#3b82f6", "#f97316"],
  PVT: ["#10b981"],
  ADX: ["#f97316", "#a855f7", "#3b82f6", "#fb7185"],
  DMI: ["#f97316", "#a855f7", "#3b82f6", "#fb7185"],
};

function toIndicatorKey(name: string): IndicatorKey | null {
  const normalized = name.toUpperCase() === "DMI" ? "ADX" : name.toUpperCase();
  return supportedPaneIndicators.includes(normalized as IndicatorKey) ? (normalized as IndicatorKey) : null;
}

function formatIndicatorDisplayName(indicator: IndicatorKey) {
  if (indicator === "DMI") {
    return "ADX";
  }
  if (indicator === "SIGNALRATIO") {
    return "信号比";
  }
  if (indicator === "AVGAMP") {
    return "均幅";
  }
  return indicator;
}

function normalizeIndicatorTitle(figure: IndicatorFigure) {
  if (typeof figure.title !== "string") {
    return "";
  }
  return figure.title.replace(/[:：]\s*$/, "：").replace(/^VOLUME：$/, "成交量：");
}

function formatPaneTooltipValue(value: unknown, precision: number, shouldFormatBigNumber: boolean) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return "--";
  }
  if (shouldFormatBigNumber) {
    return formatBigNumber(value);
  }
  return value.toFixed(Math.max(0, Math.min(4, precision)));
}

function resolveCrosshairSnapshot(chart: Chart, payload: unknown, dataLength: number) {
  const payloadCrosshair = normalizeCrosshair(payload);
  const internalCrosshair = (chart as ChartWithCrosshair).getCrosshair?.();
  const x = internalCrosshair?.realX ?? internalCrosshair?.x ?? payloadCrosshair?.realX ?? payloadCrosshair?.x;
  if (typeof x !== "number") {
    return null;
  }
  const converted = chart.convertFromPixel([{ x }], { paneId: "candle" });
  const point = Array.isArray(converted) ? normalizePoint(converted[0]) : null;
  const dataIndex = internalCrosshair?.dataIndex ?? payloadCrosshair?.dataIndex ?? point?.dataIndex;
  if (typeof dataIndex !== "number") {
    return null;
  }
  const boundedIndex = clamp(Math.round(dataIndex), 0, Math.max(0, dataLength - 1));
  return {
    x,
    dataIndex: boundedIndex,
    paneId: internalCrosshair?.paneId ?? payloadCrosshair?.paneId,
  };
}

function normalizeCrosshair(value: unknown): Crosshair | null {
  return isRecord(value) ? value : null;
}

function normalizePoint(value: unknown): Partial<Point> | null {
  return isRecord(value) ? value : null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

export function buildCandleTooltipLegendTemplate(): TooltipLegend[] {
  return [
    { title: "时间：", value: "{time}" },
    { title: "开：", value: "{open}" },
    { title: "高：", value: "{high}" },
    { title: "低：", value: "{low}" },
    { title: "收：", value: "{close}" },
    { title: "成交量：", value: "{volume}" },
  ];
}

export function buildKLineChartStyles(activePeriod: ChartPeriod = "day"): DeepPartial<Styles> {
  const isTimeSharing = activePeriod === "minute";
  return {
  grid: {
    horizontal: { show: true, color: "#edf1f7", size: 1, style: dashedLine, dashedValue: [4, 3] },
    vertical: { show: true, color: "#edf1f7", size: 1, style: dashedLine, dashedValue: [4, 3] },
  },
  xAxis: {
    axisLine: { show: true, color: "#dfe7f2", size: 1 },
    tickLine: { show: false },
    tickText: { color: "#64748b", family: APP_FONT, size: 12, weight: 400 },
  },
  yAxis: {
    axisLine: { show: false },
    tickLine: { show: false },
    tickText: { color: "#64748b", family: APP_FONT, size: 12, weight: 400 },
  },
  separator: {
    size: 1,
    color: "#e5eaf3",
    fill: true,
    activeBackgroundColor: "rgba(22,119,255,0.06)",
  },
  candle: {
    type: isTimeSharing ? "area" as const : "candle_solid" as const,
    area: {
      lineSize: 2,
      lineColor: "#1677ff",
      value: "close",
      smooth: false,
      backgroundColor: [
        { offset: 0, color: "rgba(22,119,255,0.16)" },
        { offset: 1, color: "rgba(22,119,255,0.02)" },
      ],
      point: {
        show: false,
        color: "#1677ff",
        radius: 3,
        rippleColor: "rgba(22,119,255,0.2)",
        rippleRadius: 6,
        animation: false,
        animationDuration: 0,
      },
    },
    bar: {
      upColor,
      downColor,
      noChangeColor: "#64748b",
      compareRule: "current_open" as const,
      upBorderColor: upColor,
      downBorderColor: downColor,
      noChangeBorderColor: "#64748b",
      upWickColor: upColor,
      downWickColor: downColor,
      noChangeWickColor: "#64748b",
    },
    tooltip: {
      showRule: "follow_cross" as const,
      showType: "rect" as const,
      title: { show: true, color: "#374151", family: APP_FONT, size: 12, weight: 600, marginLeft: 0, marginTop: 0, marginRight: 0, marginBottom: 8 },
      legend: {
        color: "#64748b",
        family: APP_FONT,
        size: 12,
        weight: 500,
        marginLeft: 0,
        marginTop: 0,
        marginRight: 0,
        marginBottom: 4,
        template: buildCandleTooltipLegendTemplate(),
      },
      rect: {
        position: "pointer" as const,
        paddingLeft: 10,
        paddingRight: 10,
        paddingTop: 8,
        paddingBottom: 8,
        offsetLeft: 12,
        offsetTop: 12,
        offsetRight: 12,
        offsetBottom: 12,
        color: "#ffffff",
        borderColor: "#e5eaf3",
        borderSize: 1,
        borderRadius: 8,
      },
    },
    priceMark: {
      high: { show: true, color: "#64748b", textFamily: APP_FONT, textSize: 12, textWeight: "400" },
      low: { show: true, color: "#64748b", textFamily: APP_FONT, textSize: 12, textWeight: "400" },
      last: {
        show: true,
        upColor,
        downColor,
        noChangeColor: "#64748b",
        line: { show: true, style: dashedLine, size: 1, dashedValue: [4, 3] },
        text: { show: true, color: "#ffffff", family: APP_FONT, size: 12, weight: 700, paddingLeft: 6, paddingRight: 6, paddingTop: 3, paddingBottom: 3 },
      },
    },
  },
  indicator: {
    tooltip: {
      showRule: "follow_cross" as const,
      showType: "standard" as const,
      title: { show: true, showName: true, showParams: true, color: "#374151", family: APP_FONT, size: 12, weight: 600, marginLeft: 0, marginTop: 0, marginRight: 6, marginBottom: 0 },
      legend: { color: "#64748b", family: APP_FONT, size: 12, weight: 500, marginLeft: 0, marginTop: 0, marginRight: 6, marginBottom: 0 },
    },
    lastValueMark: {
      show: false,
      text: { show: true, color: "#334155", family: APP_FONT, size: 12, weight: 400 },
    },
  },
  crosshair: {
    show: true,
    horizontal: {
      show: true,
      line: { show: true, color: "#9aa4b2", size: 1, style: dashedLine, dashedValue: [4, 3] },
      text: { show: true, color: "#ffffff", family: APP_FONT, size: 12, weight: 500, backgroundColor: "#2f333a", borderRadius: 4, paddingLeft: 6, paddingRight: 6, paddingTop: 3, paddingBottom: 3 },
    },
    vertical: {
      show: true,
      line: { show: true, color: "#9aa4b2", size: 1, style: dashedLine, dashedValue: [4, 3] },
      text: { show: true, color: "#ffffff", family: APP_FONT, size: 12, weight: 500, backgroundColor: "#2f333a", borderRadius: 4, paddingLeft: 6, paddingRight: 6, paddingTop: 3, paddingBottom: 3 },
    },
  },
};
}

function tradeDateTimestamp(value: string) {
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
  return Number.isFinite(timestamp) ? timestamp : Date.now();
}

function formatChartDate(timestamp: number, type: string) {
  const date = new Date(timestamp);
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  const hour = `${date.getHours()}`.padStart(2, "0");
  const minute = `${date.getMinutes()}`.padStart(2, "0");
  if (type === "xAxis") {
    return hour === "00" && minute === "00" ? `${year}-${month}` : `${hour}:${minute}`;
  }
  return hour === "00" && minute === "00" ? `${year}-${month}-${day}` : `${year}${month}${day} ${hour}:${minute}`;
}

function formatBigNumber(value: string | number) {
  const numeric = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(numeric)) {
    return String(value);
  }
  if (Math.abs(numeric) >= 100000000) {
    return `${(numeric / 100000000).toFixed(2)}亿`;
  }
  if (Math.abs(numeric) >= 10000) {
    return `${(numeric / 10000).toFixed(2)}万`;
  }
  return Number.isInteger(numeric) ? String(numeric) : numeric.toFixed(2);
}

function isJSDOM() {
  return typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("jsdom");
}
