import type { TaskItem } from "./types";
import type { TaskLogEvent, TaskLogRecord, TaskLogSummary } from "./taskLogTypes";

export const taskLogRecords: TaskLogRecord[] = [
  {
    id: "quote-start",
    time: "15:28:40.123",
    level: "INFO",
    module: "market",
    stage: "quote_fetch",
    message: "开始拉取行情",
  },
  {
    id: "quote-cache",
    time: "15:28:40.486",
    level: "INFO",
    module: "market",
    stage: "quote_fetch",
    message: "行情缓存命中",
  },
  {
    id: "kline-fetch",
    time: "15:28:41.022",
    level: "INFO",
    module: "kline",
    stage: "kline_fetch",
    message: "拉取日K线 120 条",
  },
  {
    id: "macd-done",
    time: "15:28:42.118",
    level: "INFO",
    module: "indicator",
    stage: "calc_macd",
    message: "MACD 计算完成",
  },
  {
    id: "prompt-built",
    time: "15:28:48.211",
    level: "INFO",
    module: "ai",
    stage: "prompt_build",
    message: "Prompt 构建完成",
  },
  {
    id: "stream-timeout",
    time: "15:28:53.941",
    level: "WARN",
    module: "ai",
    stage: "stream_timeout",
    message: "模型流式响应耗时过长",
  },
  {
    id: "stream-failed",
    time: "15:29:46.120",
    level: "ERROR",
    module: "ai",
    stage: "stream_failed",
    message: "Provider 响应超时，任务终止",
    raw: {
      timestamp: "2025-05-20T15:29:46.120+08:00",
      level: "ERROR",
      module: "ai",
      stage: "stream_failed",
      message: "Provider 响应超时，任务终止",
      request_id: "req_7d29bc9f4a2bc04f71a6b9ced3e",
      trace_id: "trace_91aa2b3c4d5e6f7a8b9c0d1e",
    },
  },
];

export const taskLogEvents: TaskLogEvent[] = [
  { id: "created", time: "15:28:34", eventType: "TASK_CREATED", description: "任务已创建，等待调度。" },
  { id: "started", time: "15:28:35", eventType: "TASK_STARTED", description: "任务开始执行。" },
  { id: "basic", time: "15:28:40", eventType: "TASK_PROGRESS", description: "正在加载股票基础信息..." },
  { id: "quote", time: "15:28:42", eventType: "TASK_PROGRESS", description: "正在拉取行情与 K线数据..." },
  { id: "chunk", time: "15:28:53", eventType: "TASK_CHUNK", description: "已生成部分报告内容..." },
  { id: "failed", time: "15:29:46", eventType: "TASK_FAILED", description: "Provider 响应超时，任务终止。" },
];

export function buildTaskLogSummary(task: TaskItem): TaskLogSummary {
  return {
    title: task.title,
    taskId: task.id,
    taskType: task.taskType,
    stock: task.stockCode && task.stockName ? `${task.stockCode}    ${task.stockName}` : "—",
    model: task.model ?? "—",
    startedAt: `2025-05-20 ${task.startedAt}`,
    duration: task.duration,
    requestId: "req_7d29...",
    traceId: "trace_91aa...",
    status: task.status,
  };
}

export const rawLogJson = {
  timestamp: "2025-05-20T15:29:46.120+08:00",
  level: "ERROR",
  module: "ai",
  stage: "stream_failed",
  message: "Provider 响应超时，任务终止",
  request_id: "req_7d29bc9f4a2bc04f71a6b9ced3e",
  trace_id: "trace_91aa2b3c4d5e6f7a8b9c0d1e",
};
