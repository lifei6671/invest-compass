import { dispose, init, type KLineData } from "klinecharts";
import { useEffect, useMemo, useRef } from "react";
import type { MarketKlineItem } from "../../services/coreClient";

export function KlineChart(props: { items: MarketKlineItem[] }) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartData = useMemo<KLineData[]>(
    () =>
      props.items.map((item) => ({
        timestamp: new Date(`${item.trade_date}T00:00:00+08:00`).getTime(),
        open: item.open,
        high: item.high,
        low: item.low,
        close: item.close,
        volume: item.volume,
        turnover: item.amount,
      })),
    [props.items],
  );

  useEffect(() => {
    if (!containerRef.current || chartData.length === 0) {
      return undefined;
    }
    if (typeof window.matchMedia !== "function" || isJSDOM()) {
      return undefined;
    }
    try {
      const chart = init(containerRef.current, {
        timezone: "Asia/Shanghai",
        styles: {
          grid: {
            horizontal: { color: "#e2e8f0", size: 1, style: "solid", show: true, dashedValue: [2, 2] },
            vertical: { color: "#e2e8f0", size: 1, style: "solid", show: true, dashedValue: [2, 2] },
          },
          candle: {
            bar: {
              upColor: "#dc2626",
              downColor: "#16a34a",
              noChangeColor: "#64748b",
              upBorderColor: "#dc2626",
              downBorderColor: "#16a34a",
              noChangeBorderColor: "#64748b",
              upWickColor: "#dc2626",
              downWickColor: "#16a34a",
              noChangeWickColor: "#64748b",
            },
          },
        },
        layout: {
          panes: [
            { type: "candle" },
            { type: "indicator", content: ["VOL"], options: { height: 72 } },
            { type: "xAxis" },
          ],
        },
      });
      if (!chart) {
        return undefined;
      }
      chart.setDataLoader({
        getBars: ({ callback }) => {
          callback(chartData, false);
        },
      });
      chart.setSymbol({ ticker: props.items[0]?.symbol ?? "LOCAL", pricePrecision: 2, volumePrecision: 0 });
      chart.setPeriod({ type: "day", span: 1 });
      chart.resize();
      return () => dispose(chart);
    } catch (cause) {
      void cause;
      return undefined;
    }
  }, [chartData, props.items]);

  return <div aria-label="K 线图表" className="h-[260px] w-full" ref={containerRef} />;
}

function isJSDOM() {
  return typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("jsdom");
}
