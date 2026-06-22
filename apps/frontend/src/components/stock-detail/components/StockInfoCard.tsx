import { Empty, Tag } from "antd";
import type { BasicInfoItem } from "../types";

type StockInfoCardProps = {
  items: BasicInfoItem[];
  concepts: string[];
};

export function StockInfoCard(props: StockInfoCardProps) {
  return (
    <section className="stock-detail-card px-5 py-4">
      <h2 className="m-0 mb-3 text-[16px] font-semibold leading-6 text-[#111827]">基础信息</h2>
      {props.items.length > 0 ? (
        <>
          <div className="grid grid-cols-2 gap-x-6">
            {props.items.map((item) => (
              <div key={item.label} className="flex h-[34px] items-center justify-between gap-3 border-b border-[#edf1f7] text-[13px]">
                <span className="text-[#64748b]">{item.label}</span>
                <span className="app-number truncate font-medium text-[#1f2937]">{item.value}</span>
              </div>
            ))}
          </div>
          <div className="mt-3 flex items-center gap-3 text-[13px]">
            <span className="shrink-0 text-[#64748b]">概念板块</span>
            <div className="flex flex-wrap gap-2">
              {props.concepts.map((concept) => (
                <Tag key={concept} className="m-0 rounded-md border-0 bg-[#eaf3ff] px-2 py-0.5 text-[12px] text-[#1677ff]">
                  {concept}
                </Tag>
              ))}
            </div>
          </div>
        </>
      ) : (
        <div className="py-6">
          <Empty description="暂无基础信息" />
        </div>
      )}
    </section>
  );
}
