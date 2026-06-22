import { ApiOutlined, ClockCircleOutlined, DatabaseOutlined, InfoCircleOutlined } from "@ant-design/icons";
import { Button } from "antd";
import type { ReactNode } from "react";
import type { DataUsageOverview } from "../types";

type DataUsageOverviewCardProps = {
  value: DataUsageOverview;
  onViewOverview: () => void;
};

export function DataUsageOverviewCard({ value, onViewOverview }: DataUsageOverviewCardProps) {
  return (
    <section className="data-description-card data-description-overview-card">
      <header className="data-description-overview-header">
        <div>
          <h2>数据使用与来源说明</h2>
          <p>说明行情、资讯、缓存与 AI 上下文使用边界</p>
        </div>
        <Button className="data-description-outline-button" onClick={onViewOverview}>
          查看数据源概览
        </Button>
      </header>
      <div className="data-description-overview-grid">
        <InfoBlock
          items={[
            { icon: <InfoCircleOutlined />, label: "适用范围：", value: value.scope },
            { icon: <DatabaseOutlined />, label: "默认市场：", value: value.defaultMarket },
            { icon: <ApiOutlined />, label: "行情来源：", value: value.marketSource },
            { icon: <InfoCircleOutlined />, label: "资讯来源：", value: value.newsSource },
          ]}
        />
        <InfoBlock
          items={[
            { icon: <ClockCircleOutlined />, label: "K线数据范围：", value: value.klineRange },
            { icon: <DatabaseOutlined />, label: "数据用途：", value: value.usage },
            { icon: <InfoCircleOutlined />, label: "明确说明：", value: value.disclaimer },
          ]}
        />
      </div>
    </section>
  );
}

function InfoBlock({ items }: { items: Array<{ icon: ReactNode; label: string; value: string }> }) {
  return (
    <div className="data-description-info-block">
      {items.map((item) => (
        <div key={item.label} className="data-description-info-row">
          <span className="data-description-info-icon">{item.icon}</span>
          <strong>{item.label}</strong>
          <span>{item.value}</span>
        </div>
      ))}
    </div>
  );
}
