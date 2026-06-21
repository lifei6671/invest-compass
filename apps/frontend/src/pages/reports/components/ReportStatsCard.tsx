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
          <span className="report-stat-label">本周生成数量</span>
          <strong className="report-stat-value">{stats.weeklyCount}</strong>
          <span className="report-stat-change report-stat-up">↑ {stats.weeklyChangePercent}%</span>
          <span className="report-stat-sub">较上周</span>
        </div>
        <div>
          <span className="report-stat-label">成功率</span>
          <strong className="report-stat-value">{stats.successRate}%</strong>
          <span className="report-stat-change report-stat-up">↑ {stats.successRateChangePercent}%</span>
          <span className="report-stat-sub">较上周</span>
        </div>
      </div>
      <div className="report-stats-footer">
        <span>
          <span className="report-stat-label">最近失败任务数</span>
          <strong>{stats.recentFailedCount}</strong>
        </span>
        <span>
          较上周 <em>↓ {Math.abs(stats.recentFailedChange)}</em>
        </span>
      </div>
    </section>
  );
}
