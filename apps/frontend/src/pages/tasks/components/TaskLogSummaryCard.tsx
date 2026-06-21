import { App as AntApp, Tooltip } from "antd";
import { CopyOutlined } from "@ant-design/icons";
import type { TaskLogSummary } from "../taskLogTypes";

type TaskLogSummaryCardProps = {
  summary: TaskLogSummary;
};

const copyableKeys = new Set(["taskId", "requestId", "traceId"]);

const fields: Array<{ key: keyof TaskLogSummary; label: string }> = [
  { key: "title", label: "标题" },
  { key: "taskId", label: "任务 ID" },
  { key: "taskType", label: "任务类型" },
  { key: "stock", label: "关联股票" },
  { key: "model", label: "使用模型" },
  { key: "startedAt", label: "开始时间" },
  { key: "duration", label: "耗时" },
  { key: "requestId", label: "request_id" },
  { key: "traceId", label: "trace_id" },
];

export function TaskLogSummaryCard({ summary }: TaskLogSummaryCardProps) {
  const { message } = AntApp.useApp();

  const handleCopy = async (value: string) => {
    await navigator.clipboard.writeText(value);
    message.success("已复制");
  };

  return (
    <section className="task-log-summary-card">
      {fields.map((field) => {
        const value = String(summary[field.key] ?? "—");
        return (
          <div className="task-log-summary-field" key={field.key}>
            <span>{field.label}</span>
            <strong title={value}>
              {value}
              {copyableKeys.has(field.key) ? (
                <Tooltip title="复制">
                  <button type="button" onClick={() => handleCopy(value)} aria-label={`复制${field.label}`}>
                    <CopyOutlined />
                  </button>
                </Tooltip>
              ) : null}
            </strong>
          </div>
        );
      })}
    </section>
  );
}
