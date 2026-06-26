import { Button } from "antd";
import { DatabaseFilled } from "@ant-design/icons";
import type { DataSourceStatus } from "../types";

type DataSourceStatusCardProps = {
  statuses: DataSourceStatus[];
  updatedAt?: string;
};

export function DataSourceStatusCard({ statuses, updatedAt }: DataSourceStatusCardProps) {
  return (
    <section className="news-side-card">
      <header className="news-side-header">
        <div>
          <DatabaseFilled className="news-source-status-icon" />
          <h3>数据源状态</h3>
        </div>
        <span>{updatedAt ? `${updatedAt} 更新` : "暂无更新时间"}</span>
      </header>
      <div className="news-source-status-table">
        <div className="news-source-status-head">
          <span>数据源</span>
          <span>状态</span>
          <span>最近更新时间</span>
          <span>缓存状态</span>
        </div>
        {statuses.length > 0 ? (
          statuses.map((item) => (
            <div key={item.name} className="news-source-status-row">
              <span>{item.name}</span>
              <StatusTag text={item.status === "normal" ? "正常" : item.status === "failed" ? "异常" : "停用"} />
              <span>{item.lastUpdatedAt}</span>
              <StatusTag text={item.cacheStatus === "good" ? "良好" : item.cacheStatus === "warning" ? "一般" : "异常"} />
            </div>
          ))
        ) : (
          <div className="news-source-status-row">
            <span>暂无数据源运行状态</span>
            <span>-</span>
            <span>-</span>
            <span>-</span>
          </div>
        )}
      </div>
      <footer className="news-source-status-footer">
        <span>缓存统计待接入</span>
        <Button type="primary" disabled>
          清理缓存
        </Button>
      </footer>
    </section>
  );
}

function StatusTag({ text }: { text: string }) {
  return <span className="news-source-status-tag">{text}</span>;
}
