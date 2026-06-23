import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { HotTopicsCard } from "./components/HotTopicsCard";
import { RecentReportsCard } from "./components/RecentReportsCard";
import { RecentTasksCard } from "./components/RecentTasksCard";
import { WatchlistDistributionCard } from "./components/WatchlistDistributionCard";

export function DashboardPage() {
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
