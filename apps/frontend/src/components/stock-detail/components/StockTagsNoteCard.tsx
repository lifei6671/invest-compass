import { App as AntApp, Button, Tag } from "antd";
import type { ReactNode } from "react";

type StockTagsNoteCardProps = {
  tags: string[];
};

export function StockTagsNoteCard(props: StockTagsNoteCardProps) {
  const { message } = AntApp.useApp();
  return (
    <section className="stock-detail-card px-5 py-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="m-0 text-[16px] font-semibold leading-6 text-[#111827]">我的标签与备注</h2>
        <Button type="link" className="h-7 px-0 text-[13px]" onClick={() => message.info("编辑标签与备注待接入")}>
          编辑
        </Button>
      </div>
      <div className="space-y-3 text-[13px]">
        <InfoRow
          label="标签"
          value={
            <div className="flex flex-wrap gap-2">
              {props.tags.map((tag) => (
                <Tag key={tag} className="m-0 rounded-md border-0 bg-[#f3f6fb] px-2.5 py-0.5 text-[12px] text-[#475569]">
                  {tag}
                </Tag>
              ))}
            </div>
          }
        />
        <InfoRow label="备注" value="关注产品结构升级及上游原材料价格变化。" />
        <InfoRow label="最近查看" value={<span className="app-number">2025-05-20 15:20:35</span>} />
      </div>
    </section>
  );
}

function InfoRow(props: { label: string; value: ReactNode }) {
  return (
    <div className="grid grid-cols-[64px_1fr] gap-3">
      <span className="text-[#64748b]">{props.label}</span>
      <div className="text-[#475569]">{props.value}</div>
    </div>
  );
}
