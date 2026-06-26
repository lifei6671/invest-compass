import { Button, Tag } from "antd";
import { DeleteOutlined, EditOutlined, EyeOutlined, RobotOutlined, StarOutlined } from "@ant-design/icons";
import type React from "react";
import { MiniTrendChart } from "./MiniTrendChart";
import type { WatchlistItem } from "./types";

type WatchlistStockCardProps = {
  item: WatchlistItem;
  onDelete: (item: WatchlistItem) => void;
  onEdit: (item: WatchlistItem) => void;
  onView: (item: WatchlistItem) => void;
  onAnalyze: (item: WatchlistItem) => void;
};

const tagStyleByName: Record<string, React.CSSProperties> = {
  核心: { backgroundColor: "#e9f8ef", borderColor: "#c7edd8", color: "#16a34a" },
  白马: { backgroundColor: "#eff6ff", borderColor: "#cfe3ff", color: "#1677ff" },
  新能源: { backgroundColor: "#eaf2ff", borderColor: "#c8dcff", color: "#2563eb" },
  PCB: { backgroundColor: "#fff2e6", borderColor: "#ffd9b3", color: "#f97316" },
  光模块: { backgroundColor: "#e8f7ff", borderColor: "#bee9ff", color: "#0284c7" },
  半导体: { backgroundColor: "#f0ecff", borderColor: "#d8ceff", color: "#7c3aed" },
  元器件: { backgroundColor: "#f2edff", borderColor: "#ded3ff", color: "#6d5dfc" },
  设备: { backgroundColor: "#f5edff", borderColor: "#e3d0ff", color: "#8b5cf6" },
};

export function WatchlistStockCard(props: WatchlistStockCardProps) {
  const toneClass = props.item.trend === "up" ? "text-[#ff4d4f]" : "text-[#16a34a]";

  return (
    <article className="group relative z-0 flex h-[286px] w-[280px] shrink-0 flex-col overflow-hidden rounded-xl border border-[#e5eaf3] bg-white p-3 shadow-[0_1px_2px_rgba(15,23,42,0.04)] transition duration-150 ease-out hover:z-10 hover:border-[#9fc5ff] hover:shadow-[0_8px_22px_rgba(22,119,255,0.08)]">
      <div className="flex items-start gap-2">
        <StarOutlined className="mt-0.5 text-[14px] text-slate-400" />
        <div className="min-w-0 flex-1 text-left">
          <div className="truncate text-[15px] font-semibold leading-5 text-[#111827]">{props.item.name}</div>
        </div>
      </div>
      <div className="mt-1 flex justify-start gap-4 text-left text-[12px] leading-4 text-[#64748b]">
        <span className="app-number">{props.item.code}</span>
        <span>{props.item.market}</span>
      </div>

      <div className="mt-2 grid grid-cols-[minmax(92px,1fr)_116px] items-center gap-3">
        <div className="min-w-0">
          <div className={["app-number text-[19px] font-bold leading-6", toneClass].join(" ")}>{props.item.price}</div>
          <div className={["app-number mt-0.5 flex items-center gap-2 text-[12px] font-medium", toneClass].join(" ")}>
            <span>{props.item.changeAmount}</span>
            <span>{props.item.changePercent}</span>
          </div>
        </div>
        <MiniTrendChart trend={props.item.trend} points={props.item.trendPoints} />
      </div>

      <div className="mt-2 grid grid-cols-3 gap-3">
        <Metric label="成交额" value={props.item.amount} />
        <Metric label="换手率" value={props.item.turnoverRate} />
        <Metric label="市盈率(PE)" value={props.item.pe} />
      </div>

      <div className="mt-2 flex min-h-[24px] flex-wrap items-center gap-x-2 gap-y-1 text-[12px] text-[#374151]">
        <span className="max-w-[72px] truncate">{props.item.industry}</span>
        {props.item.tags.map((tag) => (
          <Tag key={tag} className="m-0 h-5 rounded px-1.5 py-0 text-[11px] font-medium leading-5" style={tagStyleByName[tag] ?? { backgroundColor: "#f1f5f9", borderColor: "#e2e8f0", color: "#475569" }}>
            {tag}
          </Tag>
        ))}
      </div>

      <div className="mt-2 border-t border-[#edf1f7] pt-1.5 text-[12px] leading-[18px]">
        <div className="flex gap-1">
          <span className="text-[#8a94a6]">我的备注：</span>
          <span className="min-w-0 truncate text-[#374151]">{props.item.note}</span>
        </div>
        <div className="flex gap-1">
          <span className="text-[#8a94a6]">更新：</span>
          <span className="app-number text-[#374151]">{props.item.updatedAt}</span>
        </div>
      </div>

      <div className="mx-1 mt-auto grid h-11 shrink-0 grid-cols-4 items-center gap-1 border-t border-[#edf1f7] pt-2 pb-3">
        <ActionButton icon={<EyeOutlined />} label="详情" className="text-[#475569]" onClick={() => props.onView(props.item)} />
        <ActionButton icon={<RobotOutlined />} label="AI分析" className="text-[#1677ff]" onClick={() => props.onAnalyze(props.item)} />
        <ActionButton icon={<EditOutlined />} label="编辑" className="text-[#475569]" onClick={() => props.onEdit(props.item)} />
        <ActionButton icon={<DeleteOutlined />} label="删除" className="text-[#ff4d4f]" onClick={() => props.onDelete(props.item)} />
      </div>
    </article>
  );
}

function Metric(props: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <div className="truncate text-[11px] leading-4 text-[#8a94a6]">{props.label}</div>
      <div className="app-number truncate text-[12px] font-semibold leading-5 text-[#1f2937]" title={props.value}>
        {props.value}
      </div>
    </div>
  );
}

function ActionButton(props: { icon: React.ReactNode; label: string; className: string; onClick: () => void }) {
  return (
    <Button
      type="text"
      size="small"
      className={["h-6 min-w-0 justify-center whitespace-nowrap rounded-md px-1 text-[11px] leading-6 hover:bg-[#f3f7ff]", props.className].join(" ")}
      style={{ fontSize: 11, height: 24, lineHeight: "24px", minWidth: 0, paddingInline: 4 }}
      onClick={props.onClick}
    >
      <span className="inline-flex min-w-0 items-center justify-center gap-1 whitespace-nowrap">
        <span className="inline-flex shrink-0 items-center text-[12px] leading-none">{props.icon}</span>
        <span className="shrink-0 text-[11px] leading-none">{props.label}</span>
      </span>
    </Button>
  );
}
