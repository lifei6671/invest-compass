import { CloseOutlined } from "@ant-design/icons";
import { Button, Progress, Tag } from "antd";
import { taskStatusLabels, type TaskEvent, type TaskItem } from "../types";
import { TaskEventTimeline } from "./TaskEventTimeline";

type TaskDetailPanelProps = {
  task: TaskItem;
  events: TaskEvent[];
  onClose: () => void;
  onFullLog: () => void;
};

const statusColor: Record<TaskItem["status"], string> = {
  RUNNING: "blue",
  SUCCESS: "green",
  FAILED: "red",
  CANCELLED: "default",
};

const progressStatus: Record<TaskItem["status"], "active" | "success" | "exception" | "normal"> = {
  RUNNING: "active",
  SUCCESS: "success",
  FAILED: "exception",
  CANCELLED: "normal",
};

export function TaskDetailPanel({ task, events, onClose, onFullLog }: TaskDetailPanelProps) {
  return (
    <aside className="task-detail-panel">
      <header className="task-detail-header">
        <h2>任务详情</h2>
        <button type="button" aria-label="关闭详情" onClick={onClose}>
          <CloseOutlined />
        </button>
      </header>

      <div className="task-detail-summary">
        <span className="task-detail-ai-icon">AI</span>
        <strong>{task.title}</strong>
        <Tag color={statusColor[task.status]}>{taskStatusLabels[task.status]}</Tag>
      </div>

      <dl className="task-detail-fields">
        <div>
          <dt>任务 ID</dt>
          <dd>{task.id}</dd>
        </div>
        <div>
          <dt>任务类型</dt>
          <dd>{task.taskType}</dd>
        </div>
        <div>
          <dt>关联股票</dt>
          <dd>{task.stockCode && task.stockName ? `${task.stockCode} ${task.stockName}` : "—"}</dd>
        </div>
        <div>
          <dt>使用模型</dt>
          <dd>{task.model ?? "—"}</dd>
        </div>
        <div>
          <dt>开始时间</dt>
          <dd>2025-05-20 {task.startedAt}</dd>
        </div>
        <div>
          <dt>预计耗时</dt>
          <dd>—</dd>
        </div>
        <div className="task-detail-progress-row">
          <dt>进度</dt>
          <dd>
            <Progress percent={task.progress} size="small" status={progressStatus[task.status]} />
          </dd>
        </div>
      </dl>

      <section className="task-detail-events">
        <h3>事件流</h3>
        <TaskEventTimeline events={events} />
      </section>

      <Button className="task-full-log-button" onClick={onFullLog}>
        查看完整日志
      </Button>
    </aside>
  );
}
