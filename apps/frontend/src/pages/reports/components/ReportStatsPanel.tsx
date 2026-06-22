import type { AnalysisTypeDistributionItem, ReportStats, TopModelItem } from "../types";
import { AnalysisTypeDistributionCard } from "./AnalysisTypeDistributionCard";
import { ReportStatsCard } from "./ReportStatsCard";
import { TopModelsCard } from "./TopModelsCard";

const reportStats: ReportStats = {
  weeklyCount: 0,
  weeklyChangePercent: 0,
  successRate: 0,
  successRateChangePercent: 0,
  recentFailedCount: 0,
  recentFailedChange: 0,
};
const topModels: TopModelItem[] = [];
const analysisTypeDistribution: AnalysisTypeDistributionItem[] = [];

export function ReportStatsPanel() {
  return (
    <aside className="report-stats-panel hidden xl:block" aria-label="报告统计区域">
      <ReportStatsCard stats={reportStats} />
      <TopModelsCard models={topModels} />
      <AnalysisTypeDistributionCard items={analysisTypeDistribution} />
    </aside>
  );
}
