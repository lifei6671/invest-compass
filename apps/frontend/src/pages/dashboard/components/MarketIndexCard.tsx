import { MiniSparkline } from "./MiniSparkline";
import type { MarketIndexItem } from "../types";

type MarketIndexCardProps = {
  item: MarketIndexItem;
};

export function MarketIndexCard({ item }: MarketIndexCardProps) {
  const toneClass = item.trend === "up" ? "dashboard-tone-up" : "dashboard-tone-down";

  return (
    <article className="dashboard-surface dashboard-index-card-static">
      <div className="dashboard-index-main">
        <div className="dashboard-index-text">
          <h2>{item.name}</h2>
          <div className="dashboard-index-code">{item.code}</div>
          <div className={`dashboard-index-value ${toneClass}`}>{item.value}</div>
          <div className={`dashboard-index-change ${toneClass}`}>
            <span>{item.change}</span>
            <span>{item.changePercent}</span>
          </div>
        </div>
        <MiniSparkline values={item.sparkline} trend={item.trend} />
      </div>
      <div className="dashboard-index-footer">
        <span>{item.time}</span>
        <span>{item.status}</span>
      </div>
    </article>
  );
}
