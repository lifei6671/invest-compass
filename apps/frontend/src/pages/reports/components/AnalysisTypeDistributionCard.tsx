import type { AnalysisTypeDistributionItem } from "../types";

type AnalysisTypeDistributionCardProps = {
  items: AnalysisTypeDistributionItem[];
};

export function AnalysisTypeDistributionCard({ items }: AnalysisTypeDistributionCardProps) {
  return (
    <section className="report-side-card report-distribution-card">
      <div className="report-side-card-title">
        <h3>分析类型分布</h3>
      </div>
      <div className="report-distribution-content">
        <div className="report-donut" aria-label="分析类型分布图" />
        <div className="report-donut-legend">
          {items.map((item) => (
            <div className="report-donut-legend-row" key={item.type}>
              <span className="report-donut-dot" style={{ background: item.color }} />
              <span>{item.type}</span>
              <strong>{item.value}</strong>
              <em>({item.percent.toFixed(1)}%)</em>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
