import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { useEffect } from "react";
import { useDashboardStore, type DashboardViewState } from "../../stores/dashboardStore";
import { HotTopicsCard } from "./components/HotTopicsCard";
import { RecentReportsCard } from "./components/RecentReportsCard";
import { RecentTasksCard } from "./components/RecentTasksCard";
import { WatchlistDistributionCard } from "./components/WatchlistDistributionCard";

export function DashboardPage() {
  useEffect(() => {
    useDashboardStore.setState({
      state: dashboardTopBarState,
      loading: false,
      error: null,
      lastLoadedAt: "",
    });
  }, []);

  return (
    <section className="dashboard-page">
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
      up_count: 0,
      down_count: 0,
      flat_count: 0,
    },
    recent_reports: [],
    recent_tasks: [],
    market_news: [],
    risk_tips: ["仅供研究，不构成投资建议。"],
    provider_statuses: [],
  },
  indexQuotes: [],
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
