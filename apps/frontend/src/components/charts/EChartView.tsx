import { BarChart, CandlestickChart, LineChart, PieChart, ScatterChart } from "echarts/charts";
import {
  AxisPointerComponent,
  DataZoomComponent,
  GraphicComponent,
  GridComponent,
  LegendComponent,
  MarkLineComponent,
  MarkPointComponent,
  TooltipComponent,
} from "echarts/components";
import { init, use, type EChartsCoreOption, type EChartsType } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { useEffect, useRef, type CSSProperties } from "react";

use([
  CanvasRenderer,
  PieChart,
  LineChart,
  BarChart,
  CandlestickChart,
  ScatterChart,
  AxisPointerComponent,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  DataZoomComponent,
  GraphicComponent,
  MarkLineComponent,
  MarkPointComponent,
]);

type EChartViewProps = {
  option: EChartsCoreOption;
  style?: CSSProperties;
  className?: string;
  "aria-label"?: string;
  onClick?: (params: unknown) => void;
};

export function EChartView(props: EChartViewProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<EChartsType | null>(null);

  useEffect(() => {
    if (!containerRef.current || isJSDOM()) {
      return undefined;
    }

    const chart = init(containerRef.current);
    chartRef.current = chart;
    chart.setOption(props.option, true);

    const resize = () => chart.resize();
    window.addEventListener("resize", resize);
    const observer = typeof ResizeObserver !== "undefined" ? new ResizeObserver(resize) : null;
    observer?.observe(containerRef.current);

    return () => {
      observer?.disconnect();
      window.removeEventListener("resize", resize);
      chart.dispose();
      chartRef.current = null;
    };
  }, []);

  useEffect(() => {
    chartRef.current?.setOption(props.option, true);
  }, [props.option]);

  useEffect(() => {
    const chart = chartRef.current;
    if (!chart || !props.onClick) {
      return undefined;
    }
    chart.on("click", props.onClick);
    return () => {
      chart.off("click", props.onClick);
    };
  }, [props.onClick]);

  if (isJSDOM()) {
    return <div aria-label={props["aria-label"]} className={props.className} style={props.style} />;
  }

  return <div ref={containerRef} aria-label={props["aria-label"]} className={props.className} style={props.style} />;
}

function isJSDOM() {
  return typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("jsdom");
}
