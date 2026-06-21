import { CheckCircleFilled, CloseCircleFilled, PlayCircleFilled } from "@ant-design/icons";
import type { TaskSummary } from "../types";

type TaskSummaryCardsProps = {
  summary: TaskSummary;
};

export function TaskSummaryCards({ summary }: TaskSummaryCardsProps) {
  const items = [
    { label: "运行中任务", value: summary.runningCount, suffix: "个", icon: <PlayCircleFilled />, className: "task-summary-icon-blue" },
    { label: "今日成功", value: summary.successTodayCount, suffix: "个", icon: <CheckCircleFilled />, className: "task-summary-icon-green" },
    { label: "失败任务", value: summary.failedCount, suffix: "个", icon: <CloseCircleFilled />, className: "task-summary-icon-red" },
  ];

  return (
    <div className="task-summary-cards">
      {items.map((item) => (
        <section className="task-summary-card" key={item.label}>
          <span className={`task-summary-icon ${item.className}`}>{item.icon}</span>
          <div>
            <p>{item.label}</p>
            <strong>
              {item.value} <span>{item.suffix}</span>
            </strong>
          </div>
        </section>
      ))}
    </div>
  );
}
