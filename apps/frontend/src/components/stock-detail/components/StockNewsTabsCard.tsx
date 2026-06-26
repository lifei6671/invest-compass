import { Empty } from "antd";
import type { StockNewsItem } from "../types";

type StockNewsTabsCardProps = {
  items: StockNewsItem[];
  onViewMore?: () => void;
};

export function StockNewsTabsCard(props: StockNewsTabsCardProps) {
  return (
    <section className="stock-detail-card min-h-[210px] px-5 py-3">
      <div className="mb-2 flex items-center gap-8 border-b border-[#edf1f7]">
        <div className="relative pb-2 text-[14px] font-medium text-[#1677ff]">
          新闻资讯
          <span className="absolute bottom-0 left-0 h-0.5 w-full rounded-full bg-[#1677ff]" />
        </div>
      </div>
      <div className="grid grid-cols-[1fr_120px_150px] px-1 py-1 text-[12px] text-[#8a94a6]">
        <span>标题</span>
        <span>来源</span>
        <span>发布日期</span>
      </div>
      {props.items.length > 0 ? (
        <div className="space-y-1">
          {props.items.map((item) => (
            <article key={item.id} className="grid grid-cols-[1fr_120px_150px] items-center px-1 py-1.5 text-[13px]">
              <span className="truncate font-medium text-[#374151]">{item.title}</span>
              <span className="text-[#475569]">{item.source}</span>
              <span className="app-number text-[#64748b]">{item.publishedAt}</span>
            </article>
          ))}
        </div>
      ) : (
        <div className="flex h-[110px] items-center justify-center">
          <Empty description="暂无新闻资讯" />
        </div>
      )}
      <div className="mt-2 text-center">
        <button type="button" className="border-0 bg-transparent text-[13px] font-medium text-[#1677ff]" onClick={props.onViewMore}>
          查看更多资讯 &gt;
        </button>
      </div>
    </section>
  );
}
