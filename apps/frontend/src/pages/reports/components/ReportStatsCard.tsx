import { InfoCircleOutlined } from "@ant-design/icons";
import type { ReportStats } from "../types";

type ReportStatsCardProps = {
  stats: ReportStats;
};

export function ReportStatsCard({ stats }: ReportStatsCardProps) {
  return (
    <section className="report-side-card report-stats-card">
      <div className="report-side-card-title">
        <h3>报告统计</h3>
        <InfoCircleOutlined />
      </div>
      <div className="report-stats-main">
        <div>
          <span className="report-stat-label">报告总数</span>
          <strong className="report-stat-value">{stats.totalCount}</strong>
          <span className="report-stat-sub">已保存报告</span>
        </div>
        <div>
          <span className="report-stat-label">覆盖股票</span>
          <strong className="report-stat-value">{stats.uniqueSymbols}</strong>
          <span className="report-stat-sub">唯一股票数</span>
        </div>
      </div>
      <div className="report-stats-footer">
        <span>
          <span className="report-stat-label">最新报告</span>
          <strong>{stats.latestCreatedAt || "—"}</strong>
        </span>
      </div>
    </section>
  );
}
