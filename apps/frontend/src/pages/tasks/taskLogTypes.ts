import type { TaskItem, TaskStatus } from "./types";

export type TaskLogDrawerProps = {
  open: boolean;
  task: TaskItem | null;
  onClose: () => void;
};

export type TaskLogLevel = "INFO" | "WARN" | "ERROR";

export type TaskLogRecord = {
  id: string;
  time: string;
  level: TaskLogLevel;
  module: string;
  stage: string;
  message: string;
  raw?: Record<string, unknown>;
};

export type TaskLogTab = "events" | "logs" | "diagnosis" | "context";

export type TaskLogSummary = {
  title: string;
  taskId: string;
  taskType: string;
  stock?: string;
  model?: string;
  startedAt: string;
  duration: string;
  requestId: string;
  traceId: string;
  status: TaskStatus;
};

export type TaskLogEvent = {
  id: string;
  time: string;
  eventType:
    | "TASK_CREATED"
    | "TASK_STARTED"
    | "TASK_PROGRESS"
    | "TASK_CHUNK"
    | "TASK_SUCCESS"
    | "TASK_FAILED"
    | "TASK_CANCELLED";
  description: string;
};
