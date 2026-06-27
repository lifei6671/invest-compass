export type TaskType = "AI 分析" | "资讯同步" | "行情刷新" | "缓存清理" | "数据重建";

export type TaskStatus = "RUNNING" | "SUCCESS" | "FAILED" | "CANCELLED";

export const taskStatusLabels: Record<TaskStatus, string> = {
  RUNNING: "运行中",
  SUCCESS: "成功",
  FAILED: "失败",
  CANCELLED: "已取消",
};

export type TaskItem = {
  id: string;
  taskType: TaskType;
  title: string;
  stockCode?: string;
  stockName?: string;
  status: TaskStatus;
  progress: number;
  startedAt: string;
  startedDate: string;
  endedAt?: string;
  completedDate: string;
  duration: string;
  errorSummary?: string;
  reportId?: number;
  model?: string;
};

export type TaskFilters = {
  taskType: "全部类型" | TaskType;
  status: "全部状态" | TaskStatus;
  dateRangeLabel: string;
  dateRangeStart: string;
  dateRangeEnd: string;
  keyword: string;
};

export type TaskSummary = {
  runningCount: number;
  successTodayCount: number;
  failedCount: number;
};

export type TaskEventType =
  | "TASK_CREATED"
  | "TASK_STARTED"
  | "TASK_PROGRESS"
  | "TASK_LOG"
  | "TASK_CHUNK"
  | "TASK_SUCCESS"
  | "TASK_FAILED"
  | "TASK_CANCELLED";

export type TaskEvent = {
  id: string;
  time: string;
  eventType: TaskEventType;
  description: string;
  pending?: boolean;
};

export type TaskSortMode = "按开始时间倒序" | "按开始时间升序" | "按耗时倒序" | "按状态排序";
