import { marketIndexes } from "../mock";
import { MarketIndexCard } from "./MarketIndexCard";

export function MarketIndexGrid() {
  return (
    <div className="dashboard-market-grid">
      {marketIndexes.map((item) => (
        <MarketIndexCard key={item.code} item={item} />
      ))}
    </div>
  );
}
