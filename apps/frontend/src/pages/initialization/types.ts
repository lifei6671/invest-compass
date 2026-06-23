export type InitializationStepStatus = "completed" | "running" | "pending" | "failed";

export type InitializationStep = {
  id: string;
  index: number;
  title: string;
  status: InitializationStepStatus;
  badgeText: string;
};

export type InitializationTaskDetail = {
  taskId: string;
  elapsed: string;
  currentStage: string;
  remaining: string;
};

export type InitializationLogStatus = "success" | "running" | "pending" | "error";

export type InitializationLogItem = {
  id: string;
  time: string;
  status: InitializationLogStatus;
  message: string;
};

export type InitializationState = {
  progress: number;
  currentStepId: string;
  steps: InitializationStep[];
  taskDetail: InitializationTaskDetail;
  logs: InitializationLogItem[];
};
