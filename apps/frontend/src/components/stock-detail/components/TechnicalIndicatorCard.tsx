import { App as AntApp, Empty } from "antd";
import { useState } from "react";
import type { TechnicalIndicator } from "../types";

type TechnicalIndicatorCardProps = {
  items: TechnicalIndicator[];
};

const tabs = ["MA", "MACD", "RSI", "KDJ", "BOLL"];

export function TechnicalIndicatorCard(props: TechnicalIndicatorCardProps) {
  const { message } = AntApp.useApp();
  const [active, setActive] = useState("MA");

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
              onClick={() => {
                setActive(tab);
                if (tab !== "MA") {
                  message.info("指标详情待接入");
                }
              }}
            >
              {tab}
            </button>
          ))}
        </div>
      </div>
      {props.items.length > 0 ? (
        <div className="mt-2 grid min-w-[720px] grid-cols-[repeat(6,minmax(100px,1fr))]">
          {props.items.map((item, index) => (
            <div key={item.name} className={["min-w-0 px-3", index > 0 ? "border-l border-[#edf1f7]" : ""].join(" ")}>
              <div className="flex items-center gap-2 whitespace-nowrap text-[13px] font-semibold text-[#111827]">
                {item.name}
                <span className="app-number text-[#1f2937]">{item.value}</span>
                <span className="text-[#ff4d4f]">{item.direction === "up" ? "↑" : item.direction === "down" ? "↓" : "-"}</span>
              </div>
              <div className="mt-1 text-[12px] leading-4 text-[#8a94a6]">{item.desc}</div>
            </div>
          ))}
        </div>
      ) : (
        <div className="py-2">
          <Empty description="暂无技术指标" />
        </div>
      )}
    </section>
  );
}
