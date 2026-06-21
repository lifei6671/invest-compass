import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { useEffect } from "react";
import { useDashboardStore, type DashboardViewState } from "../../stores/dashboardStore";
import { watchlistDistribution } from "./mock";
import { HotTopicsCard } from "./components/HotTopicsCard";
import { MarketIndexGrid } from "./components/MarketIndexGrid";
import { RecentReportsCard } from "./components/RecentReportsCard";
import { RecentTasksCard } from "./components/RecentTasksCard";
import { WatchlistDistributionCard } from "./components/WatchlistDistributionCard";

export function DashboardPage() {
  useEffect(() => {
    useDashboardStore.setState({
      state: dashboardTopBarState,
      loading: false,
      error: null,
      lastLoadedAt: "2025-05-20T15:30:00+08:00",
    });
  }, []);

  return (
    <section className="dashboard-page">
      <MarketIndexGrid />
      <div className="dashboard-two-column">
        <WatchlistDistributionCard />
        <HotTopicsCard />
      </div>
      <div className="dashboard-two-column">
        <RecentReportsCard />
        <RecentTasksCard />
      </div>
      <DashboardRiskNotice />
    </section>
  );
}

const dashboardTopBarState: DashboardViewState = {
  health: { status: "ok", version: "0.1.0" },
  summary: {
    watchlist: {
      up_count: watchlistDistribution.up.count,
      down_count: watchlistDistribution.down.count,
      flat_count: watchlistDistribution.flat.count,
    },
    recent_reports: [],
    recent_tasks: [],
    market_news: [],
    risk_tips: ["仅供研究，不构成投资建议。"],
    provider_statuses: [{ name: "market", source: "mock", available: true }],
  },
  indexQuotes: [
    {
      symbol: "000001.SH",
      quote: {
        symbol: "000001.SH",
        price: 3367.46,
        change_amount: 12.34,
        change_percent: 0.37,
        quote_time: "2025-05-20T15:30:00+08:00",
      },
      error: null,
    },
  ],
  indexTrends: {},
  watchlistRows: [],
};

function DashboardRiskNotice() {
  return (
    <div className="dashboard-risk-notice">
      <div className="dashboard-risk-main">
        <InfoCircleOutlined />
        AI 生成内容仅供研究参考，请结合公开披露信息独立判断。
      </div>
      <div className="dashboard-risk-side">
        <SafetyCertificateOutlined />
        仅供研究，不构成投资建议。
      </div>
    </div>
  );
}
