import { App as AntApp, Button, Segmented } from "antd";
import { CompressOutlined, EllipsisOutlined, SettingOutlined } from "@ant-design/icons";
import { useMemo, useState } from "react";
import { EChartView } from "../../charts/EChartView";
import { APP_FONT } from "../../../styles/fonts";
import type { KlineItem } from "../mock";

type KlineChartCardProps = {
  items: KlineItem[];
};

type PeriodKey = "分时" | "日K" | "周K" | "月K";
type AdjustKey = "不复权" | "前复权" | "后复权";

const periods: PeriodKey[] = ["分时", "日K", "周K", "月K"];
const adjusts: AdjustKey[] = ["不复权", "前复权", "后复权"];

export function KlineChartCard(props: KlineChartCardProps) {
  const { message } = AntApp.useApp();
  const [period, setPeriod] = useState<PeriodKey>("日K");
  const [adjust, setAdjust] = useState<AdjustKey>("前复权");
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
                  setPeriod(item);
                  if (item !== period) {
                    message.info("K线周期切换待接入");
                  }
                }}
              >
                {item}
                {period === item ? <span className="absolute bottom-0 left-1/2 h-0.5 w-8 -translate-x-1/2 rounded-full bg-[#1677ff]" /> : null}
              </button>
            ))}
          </div>
        </div>
        <div className="flex items-center gap-4">
          <Segmented
            className="rounded-md border border-[#d9e2f1] bg-white p-0.5"
            value={adjust}
            onChange={(value) => {
              setAdjust(value as AdjustKey);
              message.info("复权设置待接入");
            }}
            options={adjusts.map((item) => ({
              value: item,
              label: <span className={["inline-flex h-7 min-w-[58px] items-center justify-center rounded px-2 text-[13px]", adjust === item ? "bg-[#eaf3ff] text-[#1677ff]" : "text-[#64748b]"].join(" ")}>{item}</span>,
            }))}
          />
          <div className="flex items-center gap-3 text-[#475569]">
            <Button type="text" className="h-8 w-8 p-0" icon={<SettingOutlined />} onClick={() => message.info("功能待接入")} />
            <Button type="text" className="h-8 w-8 p-0" icon={<CompressOutlined />} onClick={() => message.info("功能待接入")} />
            <Button type="text" className="h-8 w-8 p-0" icon={<EllipsisOutlined />} onClick={() => message.info("功能待接入")} />
          </div>
        </div>
      </div>

      <div className="border-t border-[#edf1f7] pt-3">
        <div className="mb-2 flex items-center gap-6 text-[12px]">
          <span className="text-[#64748b]">MA</span>
          <Legend color="#f59e0b" label="MA5: 25.28" />
          <Legend color="#1677ff" label="MA10: 24.92" />
          <Legend color="#8b5cf6" label="MA20: 24.18" />
          <Legend color="#16a34a" label="MA60: 23.45" />
        </div>
        {isJSDOM() ? (
          <div aria-label="K线图" className="flex h-[260px] items-center justify-center rounded border border-dashed border-[#d9e2f1] text-[13px] text-[#8a94a6]">
            K线图
          </div>
        ) : (
          <EChartView option={option} style={{ height: 260, width: "100%" }} />
        )}
        <div className="-mt-7 flex items-center gap-6 text-[12px]">
          <span className="text-[#64748b]">VOL</span>
          <span className="text-[#64748b]">成交量: 34.72万手</span>
          <Legend color="#f59e0b" label="MA5: 28.11万手" />
          <Legend color="#1677ff" label="MA10: 26.33万手" />
        </div>
      </div>
    </section>
  );
}

function buildKlineOption(items: KlineItem[]) {
  const dates = items.map((item) => item.date.slice(0, 7));
  const candleData = items.map((item) => [item.open, item.close, item.low, item.high]);
  const volumeData = items.map((item, index) => ({
    value: item.volume,
    itemStyle: { color: item.close >= item.open ? "#ff4d4f" : "#16a34a" },
    xAxis: index,
  }));
  return {
    animation: false,
    grid: [
      { left: 18, right: 38, top: 4, height: 150 },
      { left: 18, right: 38, top: 178, height: 54 },
    ],
    tooltip: { trigger: "axis", axisPointer: { type: "cross" }, textStyle: { fontFamily: APP_FONT, fontSize: 12 } },
    xAxis: [
      { type: "category", data: dates, boundaryGap: true, axisLine: { lineStyle: { color: "#d9e2f1" } }, axisLabel: { color: "#64748b", fontFamily: APP_FONT, fontSize: 12 }, splitLine: { show: false } },
      { type: "category", gridIndex: 1, data: dates, boundaryGap: true, axisLine: { lineStyle: { color: "#d9e2f1" } }, axisLabel: { color: "#64748b", fontFamily: APP_FONT, fontSize: 12 }, splitLine: { show: false } },
    ],
    yAxis: [
      { scale: true, position: "right", min: 19, max: 27, axisLabel: { color: "#475569", fontFamily: APP_FONT, fontSize: 12 }, splitLine: { lineStyle: { color: "#edf1f7", type: "dashed" } } },
      { scale: true, gridIndex: 1, position: "right", axisLabel: { color: "#475569", fontFamily: APP_FONT, fontSize: 12 }, splitLine: { lineStyle: { color: "#edf1f7" } } },
    ],
    series: [
      {
        type: "candlestick",
        data: candleData,
        itemStyle: { color: "#ff4d4f", color0: "#16a34a", borderColor: "#ff4d4f", borderColor0: "#16a34a" },
        barWidth: "42%",
      },
      lineSeries("MA5", ma(items, 5), "#f59e0b"),
      lineSeries("MA10", ma(items, 10), "#1677ff"),
      lineSeries("MA20", ma(items, 20), "#8b5cf6"),
      lineSeries("MA60", ma(items, 60), "#16a34a"),
      { type: "bar", xAxisIndex: 1, yAxisIndex: 1, data: volumeData, barWidth: "48%" },
    ],
  };
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

function Legend(props: { color: string; label: string }) {
  return <span style={{ color: props.color }}>{props.label}</span>;
}

function isJSDOM() {
  return typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("jsdom");
}
