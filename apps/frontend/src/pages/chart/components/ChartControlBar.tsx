import { Button, Popover } from "antd";
import { SettingOutlined } from "@ant-design/icons";
import type { AdjustType, ChartPeriod, IndicatorKey } from "../types";

type ChartControlBarProps = {
  activePeriod: ChartPeriod;
  activeAdjust: AdjustType;
  activeIndicators: IndicatorKey[];
  onPeriodChange: (period: ChartPeriod) => void;
  onAdjustChange: (adjust: AdjustType) => void;
  onIndicatorChange: (indicator: IndicatorKey) => void;
};

const periods: Array<{ label: string; value: ChartPeriod }> = [
  { label: "分时", value: "minute" },
  { label: "5分", value: "5m" },
  { label: "15分", value: "15m" },
  { label: "30分", value: "30m" },
  { label: "60分", value: "60m" },
  { label: "日K", value: "day" },
  { label: "周K", value: "week" },
  { label: "月K", value: "month" },
  { label: "季K", value: "quarter" },
  { label: "年K", value: "year" },
];

const adjusts: Array<{ label: string; value: AdjustType }> = [
  { label: "前复权", value: "qfq" },
  { label: "后复权", value: "hfq" },
  { label: "不复权", value: "none" },
];

const indicators: IndicatorKey[] = ["MA", "EMA", "BOLL", "MACD", "KDJ", "RSI", "VOL", "WR", "OBV"];

type IndicatorCatalogItem = {
  label: string;
  key?: IndicatorKey;
};

const indicatorGroups: Array<{ title: string; tone: string; items: IndicatorCatalogItem[] }> = [
  {
    title: "趋势",
    tone: "#ef4444",
    items: [
      { label: "MA", key: "MA" },
      { label: "EMA", key: "EMA" },
      { label: "KAMA" },
      { label: "STrend" },
      { label: "SAR", key: "SAR" },
      { label: "Ichi" },
      { label: "Aroon" },
      { label: "DEMA" },
      { label: "SATS" },
      { label: "Gator" },
      { label: "Hull" },
      { label: "TEMA" },
    ],
  },
  {
    title: "波动",
    tone: "#f97316",
    items: [
      { label: "BOLL", key: "BOLL" },
      { label: "Kelt" },
      { label: "Donch" },
      { label: "ATR" },
      { label: "均幅" },
      { label: "TTM" },
      { label: "ZigZag" },
      { label: "Fractal" },
      { label: "Mass" },
      { label: "SMC" },
    ],
  },
  {
    title: "动量",
    tone: "#3b82f6",
    items: [
      { label: "MACD", key: "MACD" },
      { label: "KDJ", key: "KDJ" },
      { label: "RSI", key: "RSI" },
      { label: "CCI", key: "CCI" },
      { label: "W%R", key: "WR" },
      { label: "SRSI" },
      { label: "CMO" },
      { label: "AO", key: "AO" },
      { label: "TRIX", key: "TRIX" },
      { label: "ROC", key: "ROC" },
      { label: "SMI" },
      { label: "Coppck" },
    ],
  },
  {
    title: "量价",
    tone: "#22c55e",
    items: [
      { label: "VOL", key: "VOL" },
      { label: "OBV", key: "OBV" },
      { label: "PVT", key: "PVT" },
      { label: "VWAP" },
      { label: "MFI" },
      { label: "CMF" },
      { label: "FI" },
      { label: "A/D" },
      { label: "ChkOsc" },
      { label: "VWBand" },
    ],
  },
  {
    title: "强度",
    tone: "#a855f7",
    items: [
      { label: "ADX", key: "DMI" },
      { label: "Pivot" },
      { label: "CHOP" },
      { label: "Elder" },
      { label: "Ulcer" },
      { label: "信号比" },
    ],
  },
];

export function ChartControlBar(props: ChartControlBarProps) {
  const indicatorPanel = (
    <div className="w-[360px] space-y-3 p-1">
      <div className="flex items-center justify-between border-b border-[#edf1f7] pb-2">
        <div>
          <div className="text-[14px] font-semibold text-[#111827]">指标库</div>
          <div className="mt-0.5 text-[12px] text-[#8a94a6]">已迁入 go-stock 指标分组，灰色项后续接入</div>
        </div>
        <span className="rounded-md bg-[#eaf3ff] px-2 py-1 text-[12px] font-semibold text-[#1677ff]">
          已选 {props.activeIndicators.length}
        </span>
      </div>
      {indicatorGroups.map((group) => (
        <div key={group.title}>
          <div className="mb-1.5 flex items-center gap-2 text-[12px] font-semibold text-[#374151]">
            <span className="h-3.5 w-1 rounded-full" style={{ backgroundColor: group.tone }} />
            {group.title}
          </div>
          <div className="flex flex-wrap gap-1.5">
            {group.items.map((item) => {
              const active = item.key ? props.activeIndicators.includes(item.key) : false;
              const enabled = Boolean(item.key);
              return (
                <button
                  key={`${group.title}-${item.label}`}
                  type="button"
                  disabled={!enabled}
                  className={[
                    "h-7 rounded-md border px-2.5 text-[12px] font-medium transition",
                    enabled
                      ? active
                        ? "border-[#1677ff] bg-[#1677ff] text-white"
                        : "border-[#e5eaf3] bg-white text-[#374151] hover:border-[#b7d3ff] hover:text-[#1677ff]"
                      : "cursor-not-allowed border-[#edf1f7] bg-[#f8fafc] text-[#a8b1c1]",
                  ].join(" ")}
                  onClick={() => {
                    if (item.key) {
                      props.onIndicatorChange(item.key);
                    }
                  }}
                >
                  {item.label}
                </button>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );

  return (
    <section className="mx-4 mt-3 flex h-[46px] shrink-0 items-center justify-between border-b border-[#e5eaf3] bg-white px-2">
      <div className="flex items-center gap-0.5">
        {periods.map((item) => (
          <ToggleButton
            key={item.value}
            label={item.label}
            active={props.activePeriod === item.value}
            onClick={() => props.onPeriodChange(item.value)}
          />
        ))}
      </div>

      <div className="flex items-center gap-0.5">
        {adjusts.map((item) => (
          <ToggleButton
            key={item.value}
            label={item.label}
            active={props.activeAdjust === item.value}
            compact
            onClick={() => props.onAdjustChange(item.value)}
          />
        ))}
      </div>

      <div className="flex items-center gap-1 border-l border-[#edf1f7] pl-3">
        <span className="mr-1 rounded-md border border-[#e5eaf3] bg-white px-2.5 py-[5px] text-[12px] font-normal text-[#374151]">指标</span>
        {indicators.map((item) => {
          const active = props.activeIndicators.includes(item);
          return (
            <button
              key={item}
              type="button"
              className={[
                "h-[28px] rounded-md border px-2.5 text-[12px] font-normal transition",
                active
                  ? "border-[#1677ff] bg-[#1677ff] text-white shadow-[0_3px_8px_rgba(22,119,255,0.16)]"
                  : "border-[#e5eaf3] bg-[#f8fafc] text-[#374151] hover:border-[#b7d3ff] hover:text-[#1677ff]",
              ].join(" ")}
              onClick={() => {
                props.onIndicatorChange(item);
              }}
            >
              {item}
            </button>
          );
        })}
        <Popover content={indicatorPanel} placement="bottomRight" trigger="click" arrow={false}>
          <Button
            aria-label="指标设置"
            className="h-[28px] w-[28px] rounded-md border-[#e5eaf3] bg-[#f8fafc] p-0 text-[#64748b]"
            icon={<SettingOutlined />}
          />
        </Popover>
      </div>
    </section>
  );
}

function ToggleButton(props: { label: string; active: boolean; compact?: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      className={[
        "h-[28px] rounded-md border text-[12px] font-normal transition",
        props.compact ? "px-2.5" : "px-3",
        props.active
          ? "border-[#b7d3ff] bg-[#eaf3ff] text-[#1677ff] shadow-[0_2px_6px_rgba(22,119,255,0.08)]"
          : "border-transparent bg-[#f8fafc] text-[#374151] hover:border-[#dbe7f7]",
      ].join(" ")}
      onClick={props.onClick}
    >
      {props.label}
    </button>
  );
}
