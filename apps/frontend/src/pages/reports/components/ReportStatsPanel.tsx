import { analysisTypeDistribution, reportStats, topModels } from "../mock";
import { AnalysisTypeDistributionCard } from "./AnalysisTypeDistributionCard";
import { ReportStatsCard } from "./ReportStatsCard";
import { TopModelsCard } from "./TopModelsCard";

export function ReportStatsPanel() {
  return (
    <aside className="report-stats-panel hidden xl:block" aria-label="报告统计区域">
      <ReportStatsCard stats={reportStats} />
      <TopModelsCard models={topModels} />
      <AnalysisTypeDistributionCard items={analysisTypeDistribution} />
    </aside>
  );
}
