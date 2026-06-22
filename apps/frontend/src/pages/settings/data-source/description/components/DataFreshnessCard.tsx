import { ClockCircleOutlined } from "@ant-design/icons";
import type { DataFreshnessItem } from "../types";

export function DataFreshnessCard({ items }: { items: DataFreshnessItem[] }) {
  return (
    <section className="data-description-card data-description-small-card">
      <h2>B. 更新时效说明</h2>
      <div className="data-description-freshness-list">
        {items.map((item) => (
          <div key={item.label} className="data-description-freshness-row">
            <ClockCircleOutlined />
            <strong>{item.label}</strong>
            <span>{item.value}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
