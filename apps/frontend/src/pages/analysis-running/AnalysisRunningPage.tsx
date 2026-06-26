import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { listen } from "@tauri-apps/api/event";
import { useCallback, useEffect, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import {
  analysisTaskCancel,
  analysisTaskSubscribe,
  taskEvents,
  taskGet,
  type TaskEventItem,
  type TaskItem,
} from "../../services/coreClient";
import { RunningActionBar } from "./components/RunningActionBar";
import { RunningTaskHeader } from "./components/RunningTaskHeader";
import { StreamingOutputPanel } from "./components/StreamingOutputPanel";
import { TaskLogPanel } from "./components/TaskLogPanel";
import { TaskStepTimeline } from "./components/TaskStepTimeline";
import type { RunningTaskSummary, TaskLogItem, TaskLogEventType, TaskStatus, TaskStep } from "./types";

const analysisTaskEventName = "analysis-task-event";

const emptyTaskSummary: RunningTaskSummary = {
  title: "AI 分析任务",
  stockName: "",
  stockCode: "",
  analysisType: "",
  status: "CANCELLED",
  taskId: "暂无",
  elapsed: "暂无",
  model: "暂无",
};

export function AnalysisRunningPage() {
  const { message } = AntApp.useApp();
  const location = useLocation();
  const navigate = useNavigate();
  const taskID = new URLSearchParams(location.search).get("taskId")?.trim() ?? "";
  const lastEventIDRef = useRef(0);
  const [autoScroll, setAutoScroll] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [streamingMarkdown, setStreamingMarkdown] = useState("");
  const [reportID, setReportID] = useState<number | null>(null);
  const [taskSummary, setTaskSummary] = useState<RunningTaskSummary>(emptyTaskSummary);
  const [steps, setSteps] = useState<TaskStep[]>([]);
  const [logs, setLogs] = useState<TaskLogItem[]>([]);

  const applyEvents = useCallback((events: TaskEventItem[]) => {
    const freshEvents = events.filter((event) => event.id > lastEventIDRef.current && eventBelongsToTask(event, taskID));
    if (freshEvents.length === 0) {
      return;
    }
    lastEventIDRef.current = Math.max(...freshEvents.map((event) => event.id));
    setSteps((current) => [...current, ...freshEvents.map((event, index) => mapTaskStep(event, current.length + index + 1))]);
    setLogs((current) => [...current, ...freshEvents.map(mapTaskLog)]);

    const chunks = freshEvents
      .map((event) => event.event === "TASK_CHUNK" ? readString(event.data.content) : "")
      .filter(Boolean);
    if (chunks.length > 0) {
      setStreamingMarkdown((current) => `${current}${current ? "\n" : ""}${chunks.join("\n")}`);
    }

    const terminalEvent = [...freshEvents].reverse().find((event) => isTerminalEvent(event.event));
    if (terminalEvent) {
      const terminalReportID = readNumber(terminalEvent.data.report_id);
      if (terminalReportID !== null && terminalReportID > 0) {
        setReportID(terminalReportID);
      }
      setGenerating(false);
      setTaskSummary((current) => ({
        ...current,
        status: statusFromTerminalEvent(terminalEvent.event),
      }));
    }
  }, [taskID]);

  useEffect(() => {
    let active = true;
    let unlisten: (() => void) | undefined;
    lastEventIDRef.current = 0;
    setStreamingMarkdown("");
    setReportID(null);
    setSteps([]);
    setLogs([]);

    if (!taskID) {
      setTaskSummary(emptyTaskSummary);
      setGenerating(false);
      return undefined;
    }

    const start = async () => {
      try {
        unlisten = await listen<TaskEventItem>(analysisTaskEventName, (event) => {
          if (active) {
            applyEvents([event.payload]);
          }
        });
        const [task, eventResult] = await Promise.all([taskGet(taskID), taskEvents(taskID, 0)]);
        if (!active) {
          return;
        }
        setTaskSummary(taskSummaryFromItem(task));
        setReportID(typeof task.report_id === "number" && task.report_id > 0 ? task.report_id : null);
        setGenerating(!isTerminalStatus(task.status));
        applyEvents(eventResult.items);
        if (!isTerminalStatus(task.status)) {
          void analysisTaskSubscribe(taskID, lastEventIDRef.current).catch((error) => {
            if (active) {
              message.error(error instanceof Error ? error.message : "任务事件订阅失败");
            }
          });
        }
      } catch (error) {
        if (active) {
          message.error(error instanceof Error ? error.message : "任务状态读取失败");
        }
      }
    };

    void start();

    return () => {
      active = false;
      unlisten?.();
    };
  }, [applyEvents, message, taskID]);

  const handleStop = async () => {
    if (!taskID || !generating) {
      return;
    }
    try {
      const result = await analysisTaskCancel(taskID);
      setGenerating(false);
      setTaskSummary((current) => ({ ...current, status: normalizeTaskStatus(result.status) }));
      message.warning("已停止生成");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "取消分析任务失败");
    }
  };

  const copyCurrentContent = () => {
    if (!streamingMarkdown.trim()) {
      message.info("暂无可复制的输出内容");
      return;
    }
    const writer = navigator.clipboard?.writeText;
    const request = writer ? writer.call(navigator.clipboard, streamingMarkdown) : Promise.resolve();
    request
      .then(() => message.success("当前内容已复制"))
      .catch(() => message.success("当前内容已复制"));
  };

  return (
    <section className="analysis-running-page">
      <RunningTaskHeader value={taskSummary} onBack={() => navigate("/analysis")} />
      <div className="analysis-running-workspace">
        <TaskStepTimeline steps={steps} />
        <StreamingOutputPanel
          autoScroll={autoScroll}
          markdown={streamingMarkdown}
          onAutoScrollChange={setAutoScroll}
          onClear={() => setStreamingMarkdown("")}
        />
        <TaskLogPanel logs={logs} onClear={() => setLogs([])} />
      </div>
      <RunningActionBar
        generating={generating}
        reportID={reportID}
        onStop={handleStop}
        onBackground={() => navigate("/tasks")}
        onCopy={copyCurrentContent}
        onViewReport={() => {
          if (reportID) {
            navigate(`/reports/${reportID}`);
          }
        }}
      />
      <AnalysisRunningRiskNotice />
    </section>
  );
}

function taskSummaryFromItem(task: TaskItem): RunningTaskSummary {
  return {
    title: task.title || "AI 分析任务",
    stockName: "",
    stockCode: "",
    analysisType: task.type || "ANALYSIS",
    status: normalizeTaskStatus(task.status),
    taskId: task.id || "暂无",
    elapsed: formatElapsed(task.started_at, task.finished_at),
    model: "由任务配置决定",
  };
}

function eventBelongsToTask(event: TaskEventItem, taskID: string): boolean {
  return !event.task_id || event.task_id === taskID;
}

function mapTaskStep(event: TaskEventItem, index: number): TaskStep {
  const eventType = normalizeEventType(event.event);
  return {
    id: index,
    title: stepTitle(eventType, event.data),
    description: eventDescription(eventType, event.data),
    status: stepStatus(eventType),
    time: formatTime(event.created_at),
    progressText: readNumber(event.data.progress) !== null ? `${readNumber(event.data.progress)}%` : undefined,
  };
}

function mapTaskLog(event: TaskEventItem): TaskLogItem {
  const eventType = normalizeEventType(event.event);
  return {
    id: String(event.id),
    time: formatTime(event.created_at) || "刚刚",
    eventType,
    color: logColor(eventType),
    description: eventDescription(eventType, event.data),
    extra: readNumber(event.data.progress) !== null ? `${readNumber(event.data.progress)}%` : undefined,
    expandable: eventType === "TASK_CHUNK",
  };
}

function normalizeEventType(event: string): TaskLogEventType {
  if (
    event === "TASK_STARTED" ||
    event === "TASK_PROGRESS" ||
    event === "TASK_LOG" ||
    event === "TASK_CHUNK" ||
    event === "TASK_SUCCESS" ||
    event === "TASK_FAILED" ||
    event === "TASK_CANCELLED"
  ) {
    return event;
  }
  return "TASK_LOG";
}

function stepTitle(event: TaskLogEventType, data: Record<string, unknown>): string {
  if (event === "TASK_STARTED") {
    return "任务已开始";
  }
  if (event === "TASK_PROGRESS") {
    return readString(data.message) || "任务处理中";
  }
  if (event === "TASK_CHUNK") {
    return "收到流式输出";
  }
  if (event === "TASK_SUCCESS") {
    return "任务已完成";
  }
  if (event === "TASK_FAILED") {
    return "任务失败";
  }
  if (event === "TASK_CANCELLED") {
    return "任务已取消";
  }
  return readString(data.message) || "任务日志";
}

function eventDescription(event: TaskLogEventType, data: Record<string, unknown>): string {
  if (event === "TASK_STARTED") {
    return "分析任务已进入执行队列。";
  }
  if (event === "TASK_PROGRESS") {
    return readString(data.message) || "分析任务正在推进。";
  }
  if (event === "TASK_CHUNK") {
    return "收到新的 AI 输出片段。";
  }
  if (event === "TASK_SUCCESS") {
    return "分析任务已完成，可在报告历史中查看结果。";
  }
  if (event === "TASK_FAILED") {
    return readString(data.error) || "分析任务执行失败。";
  }
  if (event === "TASK_CANCELLED") {
    return "分析任务已取消。";
  }
  return readString(data.message) || "任务状态已更新。";
}

function stepStatus(event: TaskLogEventType): TaskStep["status"] {
  if (event === "TASK_SUCCESS") {
    return "success";
  }
  if (event === "TASK_FAILED") {
    return "failed";
  }
  if (event === "TASK_CANCELLED") {
    return "pending";
  }
  return "running";
}

function logColor(event: TaskLogEventType): TaskLogItem["color"] {
  if (event === "TASK_SUCCESS") {
    return "green";
  }
  if (event === "TASK_FAILED") {
    return "red";
  }
  if (event === "TASK_CANCELLED") {
    return "gray";
  }
  return "blue";
}

function normalizeTaskStatus(status: string): TaskStatus {
  if (status === "SUCCESS" || status === "FAILED" || status === "CANCELLED") {
    return status;
  }
  return "RUNNING";
}

function isTerminalStatus(status: string): boolean {
  return status === "SUCCESS" || status === "FAILED" || status === "CANCELLED";
}

function isTerminalEvent(event: string): boolean {
  return event === "TASK_SUCCESS" || event === "TASK_FAILED" || event === "TASK_CANCELLED";
}

function statusFromTerminalEvent(event: string): TaskStatus {
  if (event === "TASK_SUCCESS") {
    return "SUCCESS";
  }
  if (event === "TASK_FAILED") {
    return "FAILED";
  }
  return "CANCELLED";
}

function readString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function readNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function formatElapsed(startedAt?: string, finishedAt?: string): string {
  if (!startedAt) {
    return "暂无";
  }
  const start = Date.parse(startedAt);
  const end = finishedAt ? Date.parse(finishedAt) : Date.now();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) {
    return "计算中";
  }
  const seconds = Math.max(1, Math.round((end - start) / 1000));
  return seconds < 60 ? `${seconds} 秒` : `${Math.floor(seconds / 60)} 分 ${seconds % 60} 秒`;
}

function formatTime(value?: string): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleTimeString("zh-CN", { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function AnalysisRunningRiskNotice() {
  return (
    <div className="settings-basic-risk-notice analysis-running-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleOutlined />
        <span>AI 输出需区分事实、推断和观点，仅供研究参考，不构成投资建议。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
