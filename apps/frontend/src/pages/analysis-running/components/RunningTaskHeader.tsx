import { BarChartOutlined, LeftOutlined } from "@ant-design/icons";
import { Button, Tag } from "antd";
import type { ReactNode } from "react";
import type { RunningTaskSummary } from "../types";

type RunningTaskHeaderProps = {
  value: RunningTaskSummary;
  onBack: () => void;
};

export function RunningTaskHeader({ value, onBack }: RunningTaskHeaderProps) {
  return (
    <header className="analysis-running-header">
      <div className="analysis-running-header-left">
        <h1>{value.title}</h1>
        <div className="analysis-running-subtitle">
          <BarChartOutlined className="analysis-running-bars" />
          <span>AI 分析任务进行中，请稍候...</span>
        </div>
      </div>
      <div className="analysis-running-meta" aria-label="任务状态摘要">
        <StatusMeta label="状态" value={<Tag className="analysis-running-status-tag">{value.status}</Tag>} />
        <StatusMeta label="任务 ID" value={value.taskId} />
        <StatusMeta label="已耗时" value={value.elapsed} />
        <StatusMeta label="模型" value={value.model} />
      </div>
      <Button className="analysis-running-back-button" icon={<LeftOutlined />} onClick={onBack}>
        返回
      </Button>
    </header>
  );
}

function StatusMeta(props: { label: string; value: string | ReactNode }) {
  return (
    <div className="analysis-running-meta-item">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
