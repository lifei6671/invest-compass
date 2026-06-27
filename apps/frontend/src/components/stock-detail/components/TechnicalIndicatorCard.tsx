import { Empty } from "antd";
import { useMemo, useState } from "react";
import type { TechnicalIndicator } from "../types";

type TechnicalIndicatorCardProps = {
  items: TechnicalIndicator[];
};

const tabs = ["MA", "MACD", "RSI", "KDJ", "BOLL"] as const;
export type IndicatorTab = typeof tabs[number];

const indicatorGroups: Record<IndicatorTab, { title: string; fields: Array<{ name: string; label: string }> }> = {
  MA: {
    title: "MA(5,10,20,60)",
    fields: [
      { name: "MA.MA5", label: "MA5" },
      { name: "MA.MA10", label: "MA10" },
      { name: "MA.MA20", label: "MA20" },
      { name: "MA.MA60", label: "MA60" },
    ],
  },
  MACD: {
    title: "MACD(12,26,9)",
    fields: [
      { name: "MACD.DIF", label: "DIF" },
      { name: "MACD.DEA", label: "DEA" },
      { name: "MACD.BAR", label: "BAR" },
    ],
  },
  RSI: {
    title: "RSI(6,12,24)",
    fields: [
      { name: "RSI.RSI6", label: "RSI6" },
      { name: "RSI.RSI12", label: "RSI12" },
      { name: "RSI.RSI24", label: "RSI24" },
    ],
  },
  KDJ: {
    title: "KDJ(9,3,3)",
    fields: [
      { name: "KDJ.K", label: "K" },
      { name: "KDJ.D", label: "D" },
      { name: "KDJ.J", label: "J" },
    ],
  },
  BOLL: {
    title: "BOLL(20)",
    fields: [
      { name: "BOLL.MID", label: "MID" },
      { name: "BOLL.UPPER", label: "UPPER" },
      { name: "BOLL.LOWER", label: "LOWER" },
    ],
  },
};

type IndicatorSummary = {
  title: string;
  values: Array<{ label: string; value: string; direction: TechnicalIndicator["direction"] }>;
  desc: string;
};

export function TechnicalIndicatorCard(props: TechnicalIndicatorCardProps) {
  const [active, setActive] = useState<IndicatorTab>("MA");
  const summary = useMemo(() => buildIndicatorSummary(active, props.items), [active, props.items]);

  return (
    <section className="stock-detail-card min-h-[82px] px-5 py-3">
      <div className="flex items-center justify-between gap-5">
        <div className="flex min-w-[230px] items-baseline gap-2">
          <h2 className="m-0 text-[16px] font-semibold leading-6 text-[#111827]">技术指标</h2>
          <span className="text-[12px] text-[#8a94a6]">（以最新收盘价计算）</span>
        </div>
        <div className="flex shrink-0 items-center gap-1 rounded-md bg-[#f8fafc] p-0.5">
          {tabs.map((tab) => (
            <button
              key={tab}
              type="button"
              className={["h-7 min-w-[52px] rounded border-0 px-3 text-[13px]", active === tab ? "bg-[#eaf3ff] font-medium text-[#1677ff]" : "bg-transparent text-[#64748b]"].join(" ")}
              onClick={() => setActive(tab)}
            >
              {tab}
            </button>
          ))}
        </div>
      </div>
      {summary ? (
        <div className="mt-3 min-w-0 rounded-md border border-[#edf1f7] bg-[#fbfdff] px-3 py-2">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span className="text-[13px] font-semibold text-[#111827]">{summary.title}</span>
            <span className="text-[12px] text-[#8a94a6]">{summary.desc}</span>
          </div>
          <div className="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[13px] leading-5 text-[#111827]">
            {summary.values.map((item, index) => (
              <span key={item.label} className="min-w-0 whitespace-nowrap">
                <span className="text-[#64748b]">{item.label}</span>
                <span className="app-number ml-1 font-semibold text-[#1f2937]">{item.value}</span>
                <span className={["ml-1", item.direction === "down" ? "text-[#16a34a]" : item.direction === "up" ? "text-[#ff4d4f]" : "text-[#8a94a6]"].join(" ")}>
                  {item.direction === "up" ? "↑" : item.direction === "down" ? "↓" : "-"}
                </span>
                {index < summary.values.length - 1 ? <span className="ml-2 text-[#cbd5e1]">/</span> : null}
              </span>
            ))}
          </div>
        </div>
      ) : (
        <div className="py-2">
          <Empty description={`暂无${active}指标`} />
        </div>
      )}
    </section>
  );
}

export function buildIndicatorSummary(tab: IndicatorTab, items: TechnicalIndicator[]): IndicatorSummary | null {
  const group = indicatorGroups[tab];
  const itemMap = new Map(items.map((item) => [item.name.toUpperCase(), item]));
  const values = group.fields
    .map((field) => {
      const item = itemMap.get(field.name);
      return item ? { label: field.label, value: item.value, direction: item.direction } : null;
    })
    .filter((item): item is IndicatorSummary["values"][number] => Boolean(item));
  if (values.length === 0) {
    return null;
  }
  const firstMatchedItem = itemMap.get(group.fields.find((field) => itemMap.has(field.name))?.name ?? "");
  return {
    title: group.title,
    values,
    desc: firstMatchedItem?.desc ?? "最新值",
  };
}
