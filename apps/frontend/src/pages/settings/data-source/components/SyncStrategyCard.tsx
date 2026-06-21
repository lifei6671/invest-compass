import { Button } from "antd";
import { ClockCircleOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { SyncStrategy } from "../types";

type SyncStrategyCardProps = {
  value: SyncStrategy;
  onViewScheduler: () => void;
};

export function SyncStrategyCard(props: SyncStrategyCardProps) {
  return (
    <section className="settings-basic-card settings-basic-mini-card settings-data-source-card">
      <CardHeader title="同步任务策略" description="控制后台定时任务与失败重试行为" />
      <div className="settings-data-source-kv-list">
        <KeyValue label="交易时段行情轮询" value={props.value.quotePolling ? <span className="settings-data-source-tag settings-data-source-tag-ok">已启用</span> : "未启用"} />
        <KeyValue label="资讯增量同步" value={props.value.newsSyncInterval} />
        <KeyValue label="失败自动重试" value={`${props.value.retryTimes} 次`} />
        <KeyValue label="启动后首次预热" value={props.value.startupWarmup ? <span className="settings-data-source-tag settings-data-source-tag-ok">已启用</span> : "未启用"} />
      </div>
      <Button className="settings-basic-outline-button settings-data-source-single-button" icon={<ClockCircleOutlined />} onClick={props.onViewScheduler}>
        查看调度配置
      </Button>
    </section>
  );
}

function CardHeader(props: { title: string; description: string }) {
  return (
    <header className="settings-data-source-card-header">
      <h3>{props.title}</h3>
      <p>{props.description}</p>
    </header>
  );
}

function KeyValue(props: { label: string; value: ReactNode }) {
  return (
    <div className="settings-data-source-kv-row">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
