import { App as AntApp, Button, Tag } from "antd";
import { ArrowLeftOutlined, CopyOutlined, ReloadOutlined, RobotOutlined, StarOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { StockDetail } from "../types";

type StockHeaderCardProps = {
  stock: StockDetail;
  isWatchlisted: boolean;
  onBack: () => void;
  onRefresh?: () => void;
  onAnalyze: () => void;
};

export function StockHeaderCard(props: StockHeaderCardProps) {
  const { message } = AntApp.useApp();
  const stock = props.stock;

  const copySymbol = async () => {
    try {
      await navigator.clipboard?.writeText(stock.symbol);
    } catch {
      // 剪贴板能力不可用时仍保留本地交互反馈，避免页面中断。
    }
    message.success("股票代码已复制");
  };

  return (
    <section className="stock-detail-card grid min-h-[98px] grid-cols-[minmax(430px,1fr)_640px] items-center gap-5 px-5 py-4">
      <div className="min-w-0">
        <div className="flex items-center justify-between gap-4">
          <div className="flex min-w-0 items-center gap-4">
            <h1 className="m-0 shrink-0 text-[24px] font-bold leading-8 text-[#111827]">{stock.name}</h1>
            <button type="button" className="app-number inline-flex items-center gap-1 border-0 bg-transparent p-0 text-[14px] text-[#64748b]" onClick={copySymbol}>
              {stock.symbol}
              <CopyOutlined className="text-[14px] text-[#8a94a6]" />
            </button>
          </div>
          <div className="flex shrink-0 items-center gap-2 whitespace-nowrap">
            <Button className="h-8 rounded-md px-3 text-[13px]" icon={<ArrowLeftOutlined />} onClick={props.onBack}>
              返回
            </Button>
            <Button className="h-8 rounded-md px-3 text-[13px]" icon={<ReloadOutlined />} onClick={props.onRefresh}>
              刷新行情
            </Button>
          </div>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-2 text-[13px] text-[#64748b]">
          <span className="inline-flex items-center gap-2">
            行业
            <StockTag>{stock.industry} / {stock.subIndustry}</StockTag>
          </span>
          <span className="inline-flex items-center gap-2">
            概念
            {stock.concepts.map((concept) => (
              <StockTag key={concept}>{concept}</StockTag>
            ))}
          </span>
        </div>
      </div>

      <div className="min-w-0 border-l border-[#edf1f7] pl-6">
        <div className="flex items-center justify-between gap-5">
          <div className="app-number flex shrink-0 items-baseline gap-5 text-[16px] font-semibold text-[#64748b]">
            <span className="text-[30px] font-bold leading-9">{formatQuoteNumber(stock.price)}</span>
            <span>{formatQuoteNumber(stock.changeAmount)}</span>
            <span>{formatPercent(stock.changePercent)}</span>
          </div>
          <div className="flex shrink-0 items-center justify-end gap-2 whitespace-nowrap">
            <Button className="h-9 rounded-md px-3.5 text-[#1677ff]" icon={<StarOutlined />} disabled={!props.isWatchlisted}>
              {props.isWatchlisted ? "已在自选" : "未加入自选"}
            </Button>
            <Button type="primary" className="h-9 rounded-md bg-[#1677ff] px-4" icon={<RobotOutlined />} onClick={props.onAnalyze}>
              发起 AI 分析
            </Button>
          </div>
        </div>
        <div className="mt-2 grid grid-cols-[repeat(7,max-content)] items-start gap-x-5 text-[12px]">
          <QuoteMetric label="今开" value={formatQuoteNumber(stock.open)} />
          <QuoteMetric label="最高" value={formatQuoteNumber(stock.high)} />
          <QuoteMetric label="最低" value={formatQuoteNumber(stock.low)} />
          <QuoteMetric label="昨收" value={formatQuoteNumber(stock.previousClose)} />
          <QuoteMetric label="成交额" value={stock.amount} valueClass="text-[#f97316]" />
          <QuoteMetric label="换手率" value={stock.turnoverRate} />
          <QuoteMetric label="数据更新时间" value={stock.updateTime} />
        </div>
      </div>
    </section>
  );
}

function StockTag(props: { children: ReactNode }) {
  return <Tag className="m-0 h-[22px] rounded-md border-0 bg-[#eaf3ff] px-2 text-[12px] leading-[22px] text-[#1677ff]">{props.children}</Tag>;
}

function QuoteMetric(props: { label: string; value: string; valueClass?: string }) {
  return (
    <div className="min-w-0">
      <div className="whitespace-nowrap leading-4 text-[#8a94a6]">{props.label}</div>
      <div className={["app-number mt-1 whitespace-nowrap leading-4 text-[#374151]", props.valueClass ?? ""].join(" ")}>{props.value}</div>
    </div>
  );
}

function formatQuoteNumber(value: number | null) {
  return value === null ? "暂无" : value.toFixed(2);
}

function formatPercent(value: number | null) {
  return value === null ? "暂无" : `${value.toFixed(2)}%`;
}
