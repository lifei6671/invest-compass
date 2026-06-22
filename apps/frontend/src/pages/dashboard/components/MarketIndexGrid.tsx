import { Empty } from "antd";
import { MarketIndexCard } from "./MarketIndexCard";
import type { MarketIndexItem } from "../types";

export function MarketIndexGrid() {
  const marketIndexes: MarketIndexItem[] = [];

  return (
    <div className="dashboard-market-grid">
      {marketIndexes.length > 0 ? marketIndexes.map((item) => <MarketIndexCard key={item.code} item={item} />) : <Empty description="暂无市场指数数据" />}
    </div>
  );
}
