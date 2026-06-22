import { SafetyCertificateOutlined, InfoCircleOutlined } from "@ant-design/icons";
import { useLocation, useNavigate } from "react-router-dom";
import { KlineChartCard } from "./components/KlineChartCard";
import { ResearchEntryCard } from "./components/ResearchEntryCard";
import { StockHeaderCard } from "./components/StockHeaderCard";
import { StockInfoCard } from "./components/StockInfoCard";
import { StockNewsTabsCard } from "./components/StockNewsTabsCard";
import { StockTagsNoteCard } from "./components/StockTagsNoteCard";
import { TechnicalIndicatorCard } from "./components/TechnicalIndicatorCard";
import type { StockDetail } from "./types";

const emptyStockDetail: StockDetail = {
  name: "未选择股票",
  symbol: "暂无",
  code: "",
  industry: "暂无",
  subIndustry: "暂无",
  concepts: [],
  price: null,
  changeAmount: null,
  changePercent: null,
  open: null,
  high: null,
  low: null,
  previousClose: null,
  turnoverRate: "暂无",
  amount: "暂无",
  volume: "暂无",
  updateTime: "待加载",
};

export function StockDetailPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const routeState = location.state as { from?: string } | null;
  const backToSource = () => {
    const from = routeState?.from;
    navigate(from && from !== location.pathname ? from : "/watchlist");
  };

  return (
    <section className="flex min-h-[calc(100vh-120px)] min-w-[1140px] flex-col gap-4">
      <StockHeaderCard stock={emptyStockDetail} onBack={backToSource} />
      <div className="grid min-h-0 grid-cols-[minmax(0,1fr)_360px] gap-4">
        <main className="min-w-0 space-y-4">
          <KlineChartCard items={[]} />
          <TechnicalIndicatorCard items={[]} />
          <StockNewsTabsCard items={[]} />
        </main>
        <aside className="min-w-0 space-y-3">
          <StockInfoCard items={[]} concepts={[]} />
          <StockTagsNoteCard tags={[]} />
          <ResearchEntryCard />
        </aside>
      </div>
      <StockDetailRiskNotice />
    </section>
  );
}

function StockDetailRiskNotice() {
  return (
    <div className="flex min-h-[42px] items-center justify-between rounded-lg border border-[#b9d6ff] bg-[#f2f7ff] px-5 text-[14px] text-[#155ec8]">
      <div className="flex items-center gap-3 font-medium">
        <InfoCircleOutlined className="text-[18px]" />
        仅供研究，不构成投资建议。列表数据仅供研究参考，实际行情请以数据源更新为准。
      </div>
      <div className="flex items-center gap-2 text-slate-500">
        <SafetyCertificateOutlined />
        仅供研究，不构成投资建议。
      </div>
    </div>
  );
}
