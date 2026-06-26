export type TaskStatus = "RUNNING" | "SUCCESS" | "FAILED" | "CANCELLED";

export type RunningTaskSummary = {
  title: string;
  stockName: string;
  stockCode: string;
  analysisType: string;
  status: TaskStatus;
  taskId: string;
  elapsed: string;
  model: string;
};

export type TaskStepStatus = "success" | "running" | "pending" | "failed";

export type TaskStep = {
  id: number;
  title: string;
  description: string;
  status: TaskStepStatus;
  time?: string;
  progressText?: string;
};

export type TaskLogEventType =
  | "TASK_STARTED"
  | "TASK_PROGRESS"
  | "TASK_LOG"
  | "TASK_CHUNK"
  | "TASK_SUCCESS"
  | "TASK_FAILED"
  | "TASK_CANCELLED";

export type TaskLogItem = {
  id: string;
  time: string;
  eventType: TaskLogEventType;
  color: "blue" | "green" | "red" | "gray";
  description: string;
  extra?: string;
  expandable?: boolean;
};

export type RunningPageState = {
  autoScroll: boolean;
  generating: boolean;
  streamingMarkdown: string;
  steps: TaskStep[];
  logs: TaskLogItem[];
};
