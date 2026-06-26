import { SafetyCertificateOutlined, InfoCircleOutlined } from "@ant-design/icons";
import { Alert, Spin } from "antd";
import { useLocation, useNavigate } from "react-router-dom";
import { KlineChartCard } from "./components/KlineChartCard";
import { ResearchEntryCard } from "./components/ResearchEntryCard";
import { StockHeaderCard } from "./components/StockHeaderCard";
import { StockInfoCard } from "./components/StockInfoCard";
import { StockNewsTabsCard } from "./components/StockNewsTabsCard";
import { StockTagsNoteCard } from "./components/StockTagsNoteCard";
import { TechnicalIndicatorCard } from "./components/TechnicalIndicatorCard";
import type { BasicInfoItem, KlineItem, StockDetail, StockNewsItem, TechnicalIndicator } from "./types";

type StockDetailPeriod = "day" | "week" | "month";
type AdjustType = "none" | "qfq" | "hfq";

type StockDetailPageProps = {
  stock?: StockDetail;
  basicInfo?: BasicInfoItem[];
  klineItems?: KlineItem[];
  technicalIndicators?: TechnicalIndicator[];
  newsItems?: StockNewsItem[];
  watchlistNote?: {
    tags: string[];
    note: string;
    editable: boolean;
  };
  loading?: boolean;
  error?: string | null;
  period?: StockDetailPeriod;
  adjust?: AdjustType;
  onPeriodChange?: (period: StockDetailPeriod) => void;
  onAdjustChange?: (adjust: AdjustType) => void;
  onRefresh?: () => void;
  onSaveWatchlistNote?: (value: { tags: string[]; note: string }) => Promise<void> | void;
  savingWatchlistNote?: boolean;
};

const fallbackStockDetail: StockDetail = {
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

export function StockDetailPage(props: StockDetailPageProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const routeState = location.state as { from?: string } | null;
  const backToSource = () => {
    const from = routeState?.from;
    navigate(from && from !== location.pathname ? from : "/watchlist");
  };
  const stock = props.stock ?? fallbackStockDetail;
  const period = props.period ?? "day";
  const adjust = props.adjust ?? "qfq";

  return (
    <section className="flex min-h-[calc(100vh-120px)] min-w-[1140px] flex-col gap-4">
      {props.error ? <Alert title="个股详情读取失败" description={props.error} type="error" showIcon /> : null}
      {props.loading ? <Alert title="正在读取个股详情" description={<Spin size="small" />} type="info" showIcon /> : null}
      <StockHeaderCard
        stock={stock}
        isWatchlisted={props.watchlistNote?.editable ?? false}
        onBack={backToSource}
        onRefresh={props.onRefresh}
        onAnalyze={() => navigate(`/analysis?symbol=${encodeURIComponent(stock.symbol)}`)}
      />
      <div className="grid min-h-0 grid-cols-[minmax(0,1fr)_360px] gap-4">
        <main className="min-w-0 space-y-4">
          <KlineChartCard
            items={props.klineItems ?? []}
            period={period}
            adjust={adjust}
            onPeriodChange={props.onPeriodChange}
            onAdjustChange={props.onAdjustChange}
            onFullscreen={() =>
              navigate(`/chart/kline?symbol=${encodeURIComponent(stock.symbol)}&period=${period}&adjust=${adjust}`, {
                state: { from: `${location.pathname}${location.search}` },
              })
            }
          />
          <TechnicalIndicatorCard items={props.technicalIndicators ?? []} />
          <StockNewsTabsCard items={props.newsItems ?? []} onViewMore={() => navigate("/news")} />
        </main>
        <aside className="min-w-0 space-y-3">
          <StockInfoCard items={props.basicInfo ?? []} concepts={props.stock?.concepts ?? []} />
          <StockTagsNoteCard
            tags={props.watchlistNote?.tags ?? []}
            note={props.watchlistNote?.note ?? ""}
            editable={Boolean(props.watchlistNote?.editable && props.onSaveWatchlistNote)}
            saving={props.savingWatchlistNote}
            onSave={props.onSaveWatchlistNote}
          />
          <ResearchEntryCard
            onStartFull={() => navigate(`/analysis?symbol=${encodeURIComponent(stock.symbol)}&analysisType=stock_full`)}
            onStartTechnical={() => navigate(`/analysis?symbol=${encodeURIComponent(stock.symbol)}&analysisType=technical`)}
          />
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
