import { Button, Popover, Tooltip } from "antd";
import { SettingOutlined } from "@ant-design/icons";
import type { AdjustType, ChartPeriod, IndicatorKey } from "../types";
import { goStockIndicatorGroups, goStockIndicatorTips, quickIndicators } from "../goStockIndicators";

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

export function ChartControlBar(props: ChartControlBarProps) {
  const indicatorPanel = (
    <div className="w-[360px] space-y-3 p-1">
      <div className="flex items-center justify-between border-b border-[#edf1f7] pb-2">
        <div>
          <div className="text-[14px] font-semibold text-[#111827]">指标库</div>
          <div className="mt-0.5 text-[12px] text-[#8a94a6]">已迁入 go-stock 指标分组</div>
        </div>
        <span className="rounded-md bg-[#eaf3ff] px-2 py-1 text-[12px] font-semibold text-[#1677ff]">
          已选 {props.activeIndicators.length}
        </span>
      </div>
      {goStockIndicatorGroups.map((group) => (
        <div key={group.title}>
          <div className="mb-1.5 flex items-center gap-2 text-[12px] font-semibold text-[#374151]">
            <span className="h-3.5 w-1 rounded-full" style={{ backgroundColor: group.tone }} />
            {group.title}
          </div>
          <div className="flex flex-wrap gap-1.5">
            {group.items.map((item) => {
              const active = props.activeIndicators.includes(item.key);
              return (
                <Tooltip key={`${group.title}-${item.label}`} title={<IndicatorTooltipContent indicator={item.key} />} placement="top">
                  <button
                    type="button"
                    className={[
                      "h-7 rounded-md border px-2.5 text-[12px] font-medium transition",
                      active
                        ? "border-[#1677ff] bg-[#1677ff] text-white"
                        : "border-[#e5eaf3] bg-white text-[#374151] hover:border-[#b7d3ff] hover:text-[#1677ff]",
                    ].join(" ")}
                    onClick={() => {
                      props.onIndicatorChange(item.key);
                    }}
                  >
                    {item.label}
                  </button>
                </Tooltip>
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
        {quickIndicators.map((item) => {
          const active = props.activeIndicators.includes(item);
          return (
            <Tooltip key={item} title={<IndicatorTooltipContent indicator={item} />} placement="top">
              <button
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
            </Tooltip>
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

function IndicatorTooltipContent(props: { indicator: IndicatorKey }) {
  const tip = goStockIndicatorTips[props.indicator];
  return (
    <div className="max-w-[260px] space-y-1 text-[12px] leading-5">
      <div className="font-semibold">{props.indicator}</div>
      <div>作用：{tip.effect}</div>
      <div>计算：{tip.formula}</div>
    </div>
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
