import { Button } from "antd";
import { DatabaseFilled } from "@ant-design/icons";
import type { DataSourceStatus } from "../types";

type DataSourceStatusCardProps = {
  statuses: DataSourceStatus[];
  onCleanCache: () => void;
};

export function DataSourceStatusCard({ statuses, onCleanCache }: DataSourceStatusCardProps) {
  return (
    <section className="news-side-card">
      <header className="news-side-header">
        <div>
          <DatabaseFilled className="news-source-status-icon" />
          <h3>数据源状态</h3>
        </div>
        <span>15:30 更新</span>
      </header>
      <div className="news-source-status-table">
        <div className="news-source-status-head">
          <span>数据源</span>
          <span>状态</span>
          <span>最近更新时间</span>
          <span>缓存状态</span>
        </div>
        {statuses.map((item) => (
          <div key={item.name} className="news-source-status-row">
            <span>{item.name}</span>
            <StatusTag text={item.status === "normal" ? "正常" : item.status === "failed" ? "异常" : "停用"} />
            <span>{item.lastUpdatedAt}</span>
            <StatusTag text={item.cacheStatus === "good" ? "良好" : item.cacheStatus === "warning" ? "一般" : "异常"} />
          </div>
        ))}
      </div>
      <footer className="news-source-status-footer">
        <span>缓存总量：312 MB</span>
        <Button type="primary" onClick={onCleanCache}>
          清理缓存
        </Button>
      </footer>
    </section>
  );
}

function StatusTag({ text }: { text: string }) {
  return <span className="news-source-status-tag">{text}</span>;
}

