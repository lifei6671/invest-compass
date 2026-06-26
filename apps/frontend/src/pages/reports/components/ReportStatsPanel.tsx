import type { AnalysisTypeDistributionItem, ReportStats, TopModelItem } from "../types";
import { AnalysisTypeDistributionCard } from "./AnalysisTypeDistributionCard";
import { ReportStatsCard } from "./ReportStatsCard";
import { TopModelsCard } from "./TopModelsCard";

type ReportStatsPanelProps = {
  stats: ReportStats;
  topModels: TopModelItem[];
  analysisTypeDistribution: AnalysisTypeDistributionItem[];
};

export function ReportStatsPanel({ stats, topModels, analysisTypeDistribution }: ReportStatsPanelProps) {
  return (
    <aside className="report-stats-panel hidden xl:block" aria-label="报告统计区域">
      <ReportStatsCard stats={stats} />
      <TopModelsCard models={topModels} />
      <AnalysisTypeDistributionCard items={analysisTypeDistribution} />
    </aside>
  );
}
