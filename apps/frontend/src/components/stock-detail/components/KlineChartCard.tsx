import { Button, Empty } from "antd";
import { FullscreenOutlined } from "@ant-design/icons";
import { useMemo } from "react";
import { EChartView } from "../../charts/EChartView";
import { APP_FONT } from "../../../styles/fonts";
import type { KlineItem } from "../types";

type KlineChartCardProps = {
  items: KlineItem[];
  period: PeriodValue;
  adjust: AdjustValue;
  onPeriodChange?: (period: PeriodValue) => void;
  onAdjustChange?: (adjust: AdjustValue) => void;
  onFullscreen?: () => void;
};

type PeriodKey = "日K" | "周K" | "月K";
type PeriodValue = "day" | "week" | "month";
type AdjustValue = "none" | "qfq" | "hfq";

const periods: PeriodKey[] = ["日K", "周K", "月K"];

const periodValues: Record<PeriodKey, PeriodValue> = {
  日K: "day",
  周K: "week",
  月K: "month",
};

const periodLabels: Record<PeriodValue, PeriodKey> = {
  day: "日K",
  week: "周K",
  month: "月K",
};

export function KlineChartCard(props: KlineChartCardProps) {
  const period = periodLabels[props.period];
  const option = useMemo(() => buildKlineOption(props.items), [props.items]);

  return (
    <section className="stock-detail-card h-[360px] px-5 py-4">
      <div className="mb-3 flex items-start justify-between gap-4">
        <div>
          <h2 className="m-0 text-[16px] font-semibold leading-6 text-[#111827]">K线图</h2>
          <div className="mt-3 flex items-center gap-8">
            {periods.map((item) => (
              <button
                key={item}
                type="button"
                className={["relative border-0 bg-transparent px-0 pb-2 text-[14px] font-medium", period === item ? "text-[#1677ff]" : "text-[#64748b]"].join(" ")}
                onClick={() => {
                  if (item !== period) {
                    props.onPeriodChange?.(periodValues[item]);
                  }
                }}
              >
                {item}
                {period === item ? <span className="absolute bottom-0 left-1/2 h-0.5 w-8 -translate-x-1/2 rounded-full bg-[#1677ff]" /> : null}
              </button>
            ))}
          </div>
        </div>
        <Button
          className="h-8 rounded-md border-[#d9e2f1] text-[13px] font-medium text-[#475569]"
          icon={<FullscreenOutlined />}
          onClick={props.onFullscreen}
        >
          全屏
        </Button>
      </div>

      <div className="border-t border-[#edf1f7] pt-3">
        {props.items.length === 0 ? (
          <div className="flex h-[260px] items-center justify-center">
            <Empty description="暂无K线数据" />
          </div>
        ) : isJSDOM() ? (
          <div aria-label="K线图" className="flex h-[260px] items-center justify-center rounded border border-dashed border-[#d9e2f1] text-[13px] text-[#8a94a6]">
            K线图
          </div>
        ) : (
          <EChartView option={option} style={{ height: 260, width: "100%" }} />
        )}
      </div>
    </section>
  );
}

export function buildKlineOption(items: KlineItem[]) {
  const dates = items.map((item) => item.date.slice(0, 7));
  const candleData = items.map((item) => [item.open, item.close, item.low, item.high]);
  const maSeries = {
    MA5: ma(items, 5),
    MA10: ma(items, 10),
    MA20: ma(items, 20),
    MA60: ma(items, 60),
  };
  const priceRange = calculatePriceRange(items);
  const volumeMax = calculateVolumeMax(items);
  const volumeData = items.map((item, index) => ({
    value: item.volume,
    itemStyle: { color: item.close >= item.open ? "#ff4d4f" : "#16a34a" },
    xAxis: index,
  }));
  return {
    animation: false,
    grid: [
      { left: 18, right: 66, top: 4, height: 150, containLabel: true },
      { left: 18, right: 66, top: 178, height: 54, containLabel: true },
    ],
    tooltip: {
      trigger: "axis",
      confine: true,
      axisPointer: { type: "cross" },
      textStyle: { fontFamily: APP_FONT, fontSize: 12 },
      formatter: (params: unknown) => formatKlineTooltip(params, items, maSeries),
    },
    xAxis: [
      { type: "category", data: dates, boundaryGap: true, axisLine: { lineStyle: { color: "#d9e2f1" } }, axisLabel: { color: "#64748b", fontFamily: APP_FONT, fontSize: 12 }, splitLine: { show: false } },
      { type: "category", gridIndex: 1, data: dates, boundaryGap: true, axisLine: { lineStyle: { color: "#d9e2f1" } }, axisLabel: { color: "#64748b", fontFamily: APP_FONT, fontSize: 12 }, splitLine: { show: false } },
    ],
    yAxis: [
      {
        scale: true,
        position: "right",
        min: priceRange.min,
        max: priceRange.max,
        axisLabel: { color: "#475569", fontFamily: APP_FONT, fontSize: 12, margin: 10, formatter: formatAxisNumber },
        splitLine: { lineStyle: { color: "#edf1f7", type: "dashed" } },
      },
      {
        scale: true,
        gridIndex: 1,
        position: "right",
        min: 0,
        max: volumeMax,
        axisLabel: { color: "#475569", fontFamily: APP_FONT, fontSize: 12, margin: 10, formatter: formatAxisNumber },
        splitLine: { lineStyle: { color: "#edf1f7" } },
      },
    ],
    series: [
      {
        type: "candlestick",
        data: candleData,
        itemStyle: { color: "#ff4d4f", color0: "#16a34a", borderColor: "#ff4d4f", borderColor0: "#16a34a" },
        barWidth: "42%",
      },
      lineSeries("MA5", maSeries.MA5, "#f59e0b"),
      lineSeries("MA10", maSeries.MA10, "#1677ff"),
      lineSeries("MA20", maSeries.MA20, "#8b5cf6"),
      lineSeries("MA60", maSeries.MA60, "#16a34a"),
      { type: "bar", xAxisIndex: 1, yAxisIndex: 1, data: volumeData, barWidth: "48%" },
    ],
  };
}

type KlineTooltipPoint = {
  dataIndex?: number;
};

type MovingAverageSeries = {
  MA5: number[];
  MA10: number[];
  MA20: number[];
  MA60: number[];
};

export function formatKlineTooltip(params: unknown, items: KlineItem[], maSeries: MovingAverageSeries) {
  const points = Array.isArray(params) ? params as KlineTooltipPoint[] : [];
  const dataIndex = points.find((point) => typeof point.dataIndex === "number")?.dataIndex ?? 0;
  const item = items[dataIndex];
  if (!item) {
    return "";
  }

  const rows = [
    ["开盘", item.open],
    ["收盘", item.close],
    ["最低", item.low],
    ["最高", item.high],
    ["成交量", formatAxisNumber(item.volume)],
    ["MA5", maSeries.MA5[dataIndex]],
    ["MA10", maSeries.MA10[dataIndex]],
    ["MA20", maSeries.MA20[dataIndex]],
    ["MA60", maSeries.MA60[dataIndex]],
  ];

  return [
    `<div style="min-width:132px;font-family:${APP_FONT};">`,
    `<div style="margin-bottom:6px;color:#64748b;">${escapeHtml(item.date)}</div>`,
    ...rows.map(([label, value]) => `<div style="display:flex;justify-content:space-between;gap:16px;line-height:22px;"><span>${label}</span><strong>${formatTooltipValue(value)}</strong></div>`),
    "</div>",
  ].join("");
}

function formatTooltipValue(value: unknown) {
  if (typeof value === "number") {
    return formatNumber(value);
  }
  if (typeof value === "string") {
    return escapeHtml(value);
  }
  return "-";
}

function formatAxisNumber(value: number) {
  if (!Number.isFinite(value)) {
    return "";
  }
  const abs = Math.abs(value);
  if (abs >= 100000000) {
    return `${formatNumber(value / 100000000)}亿`;
  }
  if (abs >= 10000) {
    return `${formatNumber(value / 10000)}万`;
  }
  return formatNumber(value);
}

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/\.?0+$/, "");
}

function escapeHtml(value: string) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function calculatePriceRange(items: KlineItem[]) {
  const lows = items.map((item) => item.low).filter(Number.isFinite);
  const highs = items.map((item) => item.high).filter(Number.isFinite);
  const min = Math.min(...lows);
  const max = Math.max(...highs);
  if (!Number.isFinite(min) || !Number.isFinite(max)) {
    return { min: undefined, max: undefined };
  }
  const padding = Math.max((max - min) * 0.12, max * 0.015, 0.5);
  return {
    min: Number(Math.max(0, min - padding).toFixed(2)),
    max: Number((max + padding).toFixed(2)),
  };
}

function calculateVolumeMax(items: KlineItem[]) {
  const max = Math.max(...items.map((item) => item.volume).filter(Number.isFinite));
  if (!Number.isFinite(max) || max <= 0) {
    return undefined;
  }
  return Math.ceil(max * 1.18);
}

function ma(items: KlineItem[], size: number) {
  return items.map((_, index) => {
    const start = Math.max(0, index - size + 1);
    const slice = items.slice(start, index + 1);
    return Number((slice.reduce((total, item) => total + item.close, 0) / slice.length).toFixed(2));
  });
}

function lineSeries(name: string, data: number[], color: string) {
  return {
    name,
    type: "line",
    data,
    smooth: true,
    symbol: "none",
    lineStyle: { width: 1.4, color },
  };
}

function isJSDOM() {
  return typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("jsdom");
}
