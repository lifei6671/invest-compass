import { App as AntApp } from "antd";
import { useState } from "react";
import type { StockNewsItem } from "../mock";

type StockNewsTabsCardProps = {
  items: StockNewsItem[];
};

const tabs = ["新闻资讯", "AI分析摘要", "历史报告"];

export function StockNewsTabsCard(props: StockNewsTabsCardProps) {
  const { message } = AntApp.useApp();
  const [active, setActive] = useState("新闻资讯");
  return (
    <section className="stock-detail-card min-h-[210px] px-5 py-3">
      <div className="mb-2 flex items-center gap-8 border-b border-[#edf1f7]">
        {tabs.map((tab) => (
          <button
            key={tab}
            type="button"
            className={["relative border-0 bg-transparent px-0 pb-2 text-[14px] font-medium", active === tab ? "text-[#1677ff]" : "text-[#475569]"].join(" ")}
            onClick={() => {
              setActive(tab);
              if (tab !== "新闻资讯") {
                message.info("内容待接入");
              }
            }}
          >
            {tab}
            {active === tab ? <span className="absolute bottom-0 left-0 h-0.5 w-full rounded-full bg-[#1677ff]" /> : null}
          </button>
        ))}
      </div>
      {active === "新闻资讯" ? (
        <>
          <div className="grid grid-cols-[1fr_120px_150px] px-1 py-1 text-[12px] text-[#8a94a6]">
            <span>标题</span>
            <span>来源</span>
            <span>发布日期</span>
          </div>
          <div className="space-y-1">
            {props.items.map((item) => (
              <article key={item.id} className="grid grid-cols-[1fr_120px_150px] items-center px-1 py-1.5 text-[13px]">
                <span className="truncate font-medium text-[#374151]">{item.title}</span>
                <span className="text-[#475569]">{item.source}</span>
                <span className="app-number text-[#64748b]">{item.publishedAt}</span>
              </article>
            ))}
          </div>
          <div className="mt-2 text-center">
            <button type="button" className="border-0 bg-transparent text-[13px] font-medium text-[#1677ff]" onClick={() => message.info("资讯中心待接入")}>
              查看更多资讯 &gt;
            </button>
          </div>
        </>
      ) : (
        <div className="flex h-[142px] items-center justify-center text-[13px] text-[#8a94a6]">内容待接入</div>
      )}
    </section>
  );
}
