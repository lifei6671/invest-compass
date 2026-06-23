import { Button, Tooltip } from "antd";
import type { DataSourceExplanationItem } from "../types";

type DataSourceExplanationCardProps = {
  items: DataSourceExplanationItem[];
  onViewDataSource: () => void;
};

export function DataSourceExplanationCard({ items, onViewDataSource }: DataSourceExplanationCardProps) {
  return (
    <section className="data-description-card data-description-small-card">
      <h2>A. 数据来源说明</h2>
      <div className="data-description-source-table">
        {items.map((item) => (
          <div key={item.name} className="data-description-source-row">
            <span className={`data-description-status-dot data-description-status-${item.status}`} />
            <strong>{item.name}</strong>
            <Tooltip title={item.source}>
              <span className="data-description-source-text">{item.source}</span>
            </Tooltip>
            <Tooltip title={item.description}>
              <span className="data-description-source-detail">{item.description}</span>
            </Tooltip>
            {item.status === "limited" ? <em>受限</em> : null}
          </div>
        ))}
      </div>
      <Button className="data-description-link-button data-description-bottom-button" onClick={onViewDataSource}>
        查看数据源
      </Button>
    </section>
  );
}
