import { DownOutlined } from "@ant-design/icons";
import { Button, Tag } from "antd";
import type { TaskLogItem } from "../types";

type TaskLogPanelProps = {
  logs: TaskLogItem[];
  onClear: () => void;
};

export function TaskLogPanel({ logs, onClear }: TaskLogPanelProps) {
  return (
    <section className="analysis-running-card analysis-running-log-card">
      <div className="analysis-running-card-header">
        <h2 className="analysis-running-card-title">任务日志</h2>
        <Button size="small" className="analysis-running-small-button" onClick={onClear}>
          清空日志
        </Button>
      </div>
      <div className="analysis-running-log-list">
        {logs.map((log, index) => (
          <div key={log.id} className={`analysis-running-log-item analysis-running-log-${log.color}`}>
            <time>{log.time}</time>
            <div className="analysis-running-log-rail">
              <span className="analysis-running-log-dot" />
              {index < logs.length - 1 ? <span className="analysis-running-log-line" /> : null}
            </div>
            <div className="analysis-running-log-content">
              <div className="analysis-running-log-head">
                <Tag className={`analysis-running-log-tag analysis-running-log-tag-${log.color}`}>{log.eventType}</Tag>
                {log.extra ? <span className="analysis-running-log-extra">{log.extra}</span> : null}
                {log.expandable ? <DownOutlined className="analysis-running-log-expand" /> : null}
              </div>
              <p>{log.description}</p>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
