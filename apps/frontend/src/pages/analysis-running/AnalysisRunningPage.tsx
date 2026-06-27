import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { listen } from "@tauri-apps/api/event";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import {
  analysisTaskCreate,
  analysisTaskCancel,
  analysisTaskSubscribe,
  reportGet,
  stockProfile,
  taskEvents,
  taskGet,
  type AnalysisTaskCreatePayload,
  type TaskEventItem,
  type TaskItem,
} from "../../services/coreClient";
import { RunningActionBar } from "./components/RunningActionBar";
import { RunningTaskHeader } from "./components/RunningTaskHeader";
import { StreamingOutputPanel } from "./components/StreamingOutputPanel";
import { TaskLogPanel } from "./components/TaskLogPanel";
import { TaskStepTimeline } from "./components/TaskStepTimeline";
import { takeAnalysisTaskCreateDraft } from "./analysisTaskCreateDraft";
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

type AnalysisRunningRouteState = {
  stockName?: unknown;
  stockCode?: unknown;
  stockSymbol?: unknown;
  analysisType?: unknown;
  analysisTypeLabel?: unknown;
  createPayload?: unknown;
  createDraftId?: unknown;
};

type StepStageKey = string;

export function AnalysisRunningPage() {
  const { message } = AntApp.useApp();
  const location = useLocation();
  const navigate = useNavigate();
  const taskID = new URLSearchParams(location.search).get("taskId")?.trim() ?? "";
  const routeTaskSummary = useMemo(() => taskSummaryFromRouteState(location.state, taskID), [location.state, taskID]);
  const createDraftID = useMemo(() => createDraftIDFromRouteState(location.state), [location.state]);
  const legacyCreateTask = useMemo(() => createLegacyTaskFromRouteState(location.state), [location.state]);
  const createPromiseRef = useRef<ReturnType<typeof analysisTaskCreate> | null>(null);
  const pendingCreateTaskRef = useRef<(() => ReturnType<typeof analysisTaskCreate>) | null>(null);
  const hydratedReportIDRef = useRef<number | null>(null);
  const lastEventIDRef = useRef(0);
  const terminalReachedRef = useRef(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [streamingMarkdown, setStreamingMarkdown] = useState("");
  const [reportID, setReportID] = useState<number | null>(null);
  const [taskSummary, setTaskSummary] = useState<RunningTaskSummary>(routeTaskSummary);
  const [steps, setSteps] = useState<TaskStep[]>([]);
  const [logs, setLogs] = useState<TaskLogItem[]>([]);

  const applyTaskSnapshot = useCallback((task: TaskItem) => {
    const terminalTask = isTerminalStatus(task.status);
    if (terminalTask) {
      terminalReachedRef.current = true;
    }
    setTaskSummary((current) => enrichTaskSummary(enrichTaskSummary(taskSummaryFromItem(task), current), routeTaskSummary));
    setReportID((current) => typeof task.report_id === "number" && task.report_id > 0 ? task.report_id : current);
    setGenerating(!terminalTask);
  }, [routeTaskSummary]);

  const applyEvents = useCallback((events: TaskEventItem[]) => {
    const freshEvents = events
      .map(normalizeRunningTaskEvent)
      .filter((event) => event.id > lastEventIDRef.current && eventBelongsToTask(event, taskID));
    if (freshEvents.length === 0) {
      return;
    }
    lastEventIDRef.current = Math.max(...freshEvents.map((event) => event.id));
    const terminalEvent = [...freshEvents].reverse().find((event) => isTerminalEvent(event.event));
    setSteps((current) => applyStepEvents(current, freshEvents));
    setLogs((current) => [...current, ...freshEvents.map(mapTaskLog)]);

    const chunks = freshEvents
      .map((event) => event.event === "TASK_CHUNK" ? readChunkContent(event.data) : "")
      .filter(Boolean);
    if (chunks.length > 0) {
      setStreamingMarkdown((current) => `${current}${current ? "\n" : ""}${chunks.join("\n")}`);
    }

    if (terminalEvent) {
      terminalReachedRef.current = true;
      const terminalReportID = readNumber(terminalEvent.data.report_id);
      if (terminalReportID !== null && terminalReportID > 0) {
        setReportID(terminalReportID);
      }
      setGenerating(false);
    }
    setTaskSummary((current) => enrichTaskSummaryWithEvents(terminalEvent ? { ...current, status: statusFromTerminalEvent(terminalEvent.event) } : current, freshEvents));
  }, [taskID]);

  useEffect(() => {
    let active = true;
    let unlisten: (() => void) | undefined;
    lastEventIDRef.current = 0;
    hydratedReportIDRef.current = null;
    terminalReachedRef.current = false;
    setStreamingMarkdown("");
    setReportID(null);
    setSteps([]);
    setLogs([]);
    setTaskSummary(routeTaskSummary);

    if (!taskID && !pendingCreateTaskRef.current) {
      pendingCreateTaskRef.current = createDraftID ? takeAnalysisTaskCreateDraft(createDraftID) : legacyCreateTask;
    }
    const pendingCreateTask = pendingCreateTaskRef.current;

    if (!taskID && pendingCreateTask) {
      setTaskSummary({ ...routeTaskSummary, status: "RUNNING", taskId: "创建中" });
      setGenerating(true);
      if (!createPromiseRef.current) {
        createPromiseRef.current = pendingCreateTask();
      }
      createPromiseRef.current
        .then((result) => {
          if (!active) {
            return;
          }
          createPromiseRef.current = null;
          pendingCreateTaskRef.current = null;
          navigate(`/analysis/running?taskId=${encodeURIComponent(result.task_id)}`, {
            replace: true,
            state: routeStateWithoutCreatePayload(location.state),
          });
        })
        .catch((error) => {
          if (!active) {
            return;
          }
          createPromiseRef.current = null;
          pendingCreateTaskRef.current = null;
          setGenerating(false);
          setTaskSummary((current) => ({ ...current, status: "FAILED", taskId: "创建失败" }));
          message.error(error instanceof Error ? error.message : "AI 分析任务创建失败");
        });
      return () => {
        active = false;
      };
    }

    createPromiseRef.current = null;
    pendingCreateTaskRef.current = null;

    if (!taskID) {
      setTaskSummary(emptyTaskSummary);
      setGenerating(false);
      terminalReachedRef.current = true;
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
        applyTaskSnapshot(task);
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
    const pollTimer = window.setInterval(() => {
      if (!active || terminalReachedRef.current) {
        window.clearInterval(pollTimer);
        return;
      }
      Promise.all([taskEvents(taskID, lastEventIDRef.current), taskGet(taskID)])
        .then(([eventResult, task]) => {
          if (!active) {
            return;
          }
          applyTaskSnapshot(task);
          applyEvents(eventResult.items);
          if (terminalReachedRef.current) {
            window.clearInterval(pollTimer);
          }
        })
        .catch(() => {
          // SSE 是主通道，轮询只做兜底；单次失败不打断运行页。
        });
    }, 1000);

    return () => {
      active = false;
      window.clearInterval(pollTimer);
      unlisten?.();
    };
  }, [applyEvents, applyTaskSnapshot, createDraftID, legacyCreateTask, location.state, message, navigate, routeTaskSummary, taskID]);

  useEffect(() => {
    if (!taskSummary.stockCode || taskSummary.stockName) {
      return undefined;
    }
    let active = true;
    stockProfile(taskSummary.stockCode)
      .then((profile) => {
        if (!active || !profile.name) {
          return;
        }
        setTaskSummary((current) => {
          if (current.stockName || current.stockCode !== taskSummary.stockCode) {
            return current;
          }
          const stockName = profile.name;
          return {
            ...current,
            stockName,
            title: formatRunningTaskTitle(stockName, current.stockCode, analysisTypeLabelFromValue(current.analysisType), current.title),
          };
        });
      })
      .catch(() => {
        // 股票名称补全只影响标题展示，失败时保留任务自身标题，避免干扰任务执行状态。
      });
    return () => {
      active = false;
    };
  }, [taskSummary.stockCode, taskSummary.stockName]);

  useEffect(() => {
    if (!reportID || streamingMarkdown.trim() || hydratedReportIDRef.current === reportID) {
      return undefined;
    }
    hydratedReportIDRef.current = reportID;
    let active = true;
    reportGet(reportID)
      .then((report) => {
        if (!active) {
          return;
        }
        const markdown = readReportMarkdown(report);
        if (!markdown) {
          return;
        }
        setStreamingMarkdown((current) => current.trim() ? current : markdown);
      })
      .catch(() => {
        // 报告正文回填只用于兜底展示，失败时保留任务事件视图，避免打断运行页。
      });
    return () => {
      active = false;
    };
  }, [reportID, streamingMarkdown]);

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
          generating={generating}
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

function taskSummaryFromRouteState(state: unknown, taskID: string): RunningTaskSummary {
  const routeState = state && typeof state === "object" ? state as AnalysisRunningRouteState : {};
  const stockName = readString(routeState.stockName);
  const stockCode = readString(routeState.stockCode) || readString(routeState.stockSymbol);
  const analysisType = readString(routeState.analysisType);
  const analysisTypeLabel = readString(routeState.analysisTypeLabel) || analysisTypeLabelFromValue(analysisType);
  return {
    ...emptyTaskSummary,
    title: formatRunningTaskTitle(stockName, stockCode, analysisTypeLabel),
    stockName,
    stockCode,
    analysisType: analysisType || analysisTypeLabel,
    taskId: taskID || "暂无",
  };
}

export function createDraftIDFromRouteState(state: unknown): string {
  const routeState = state && typeof state === "object" ? state as AnalysisRunningRouteState : {};
  return readString(routeState.createDraftId);
}

function createLegacyTaskFromRouteState(state: unknown): (() => ReturnType<typeof analysisTaskCreate>) | null {
  const routeState = state && typeof state === "object" ? state as AnalysisRunningRouteState : {};
  // 兼容旧路由状态：读取后会在 replace 导航时移除，不再由新入口写入。
  const payload = routeState.createPayload;
  if (!payload || typeof payload !== "object") {
    return null;
  }
  const candidate = payload as Partial<AnalysisTaskCreatePayload>;
  if (
    !candidate.symbol ||
    !candidate.analysis_type ||
    !candidate.ai_config_id ||
    !candidate.api_key_ref ||
    !candidate.prompt_template_id
  ) {
    return null;
  }
  if (candidate.analysis_type !== "stock_full" && candidate.analysis_type !== "technical") {
    return null;
  }
  const createPayload: AnalysisTaskCreatePayload = {
    symbol: candidate.symbol,
    analysis_type: candidate.analysis_type,
    ai_config_id: candidate.ai_config_id,
    api_key_ref: candidate.api_key_ref,
    prompt_template_id: candidate.prompt_template_id,
    retry_of_task_id: candidate.retry_of_task_id,
    user_position: candidate.user_position,
  };
  return () => analysisTaskCreate(createPayload);
}

function routeStateWithoutCreatePayload(state: unknown): AnalysisRunningRouteState {
  const routeState = state && typeof state === "object" ? state as AnalysisRunningRouteState : {};
  const { createPayload: _createPayload, createDraftId: _createDraftId, ...rest } = routeState;
  return rest;
}

function taskSummaryFromItem(task: TaskItem): RunningTaskSummary {
  const parsedTitle = parseTaskTitle(task.title);
  const analysisType = parsedTitle.analysisType || task.type || "ANALYSIS";
  return {
    title: formatRunningTaskTitle(parsedTitle.stockName, parsedTitle.stockCode, analysisTypeLabelFromValue(analysisType), task.title || "AI 分析任务"),
    stockName: parsedTitle.stockName,
    stockCode: parsedTitle.stockCode,
    analysisType,
    status: normalizeTaskStatus(task.status),
    taskId: task.id || "暂无",
    elapsed: formatElapsed(task.started_at, task.finished_at),
    model: "由任务配置决定",
  };
}

function enrichTaskSummary(base: RunningTaskSummary, preferred: RunningTaskSummary): RunningTaskSummary {
  const stockName = preferred.stockName || base.stockName;
  const stockCode = preferred.stockCode || base.stockCode;
  const analysisType = preferred.analysisType || base.analysisType;
  const analysisTypeLabel = analysisTypeLabelFromValue(preferred.analysisType) || analysisTypeLabelFromValue(base.analysisType);
  return {
    ...base,
    stockName,
    stockCode,
    analysisType,
    title: formatRunningTaskTitle(stockName, stockCode, analysisTypeLabel, base.title),
  };
}

function enrichTaskSummaryWithEvents(current: RunningTaskSummary, events: TaskEventItem[]): RunningTaskSummary {
  const identity = events.reduce(
    (acc, event) => ({
      stockName: acc.stockName || readString(event.data.stock_name) || readString(event.data.stockName),
      stockCode: acc.stockCode || readString(event.data.symbol) || readString(event.data.stock_code) || readString(event.data.stockCode),
      analysisType: acc.analysisType || readString(event.data.analysis_type) || readString(event.data.analysisType),
    }),
    { stockName: "", stockCode: "", analysisType: "" },
  );
  if (!identity.stockName && !identity.stockCode && !identity.analysisType) {
    return current;
  }
  const stockName = identity.stockName || current.stockName;
  const stockCode = identity.stockCode || current.stockCode;
  const analysisType = identity.analysisType || current.analysisType;
  return {
    ...current,
    stockName,
    stockCode,
    analysisType,
    title: formatRunningTaskTitle(stockName, stockCode, analysisTypeLabelFromValue(analysisType), current.title),
  };
}

function eventBelongsToTask(event: TaskEventItem, taskID: string): boolean {
  return !event.task_id || event.task_id === taskID;
}

type RawRunningTaskEvent = TaskEventItem & {
  event_type?: string;
  payload?: string | Record<string, unknown>;
};

function normalizeRunningTaskEvent(event: TaskEventItem): TaskEventItem {
  const raw = event as RawRunningTaskEvent;
  return {
    id: raw.id,
    task_id: raw.task_id,
    event: raw.event || raw.event_type || "",
    data: raw.data ?? parseEventPayload(raw.payload),
    created_at: raw.created_at,
  };
}

function mapTaskStep(event: TaskEventItem, index: number): TaskStep | null {
  const eventType = normalizeEventType(event.event);
  const stageKey = stepStageKey(eventType, event.data);
  if (!stageKey) {
    return null;
  }
  return {
    id: index,
    stageKey,
    title: stepTitle(stageKey, eventType, event.data),
    description: eventDescription(eventType, event.data),
    status: stepStatus(eventType),
    time: formatTime(event.created_at),
    progressText: readNumber(event.data.progress) !== null ? `${readNumber(event.data.progress)}%` : undefined,
  };
}

function applyStepEvents(current: TaskStep[], events: TaskEventItem[]): TaskStep[] {
  return events.reduce<TaskStep[]>((steps, event) => {
    const nextStep = mapTaskStep(event, steps.length + 1);
    if (!nextStep) {
      return steps;
    }
    const settledSteps = steps.map((step) => {
      if (step.status !== "running") {
        return step;
      }
      if (nextStep.status === "failed") {
        return { ...step, status: "failed" as const };
      }
      return { ...step, status: "success" as const };
    });
    const existingIndex = settledSteps.findIndex((step) => step.stageKey === nextStep.stageKey);
    if (existingIndex >= 0) {
      return settledSteps.map((step, index) => index === existingIndex ? { ...nextStep, id: step.id } : step);
    }
    return [...settledSteps, { ...nextStep, id: settledSteps.length + 1 }];
  }, current);
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
    event === "TASK_CREATED" ||
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

function stepStageKey(event: TaskLogEventType, data: Record<string, unknown>): StepStageKey | "" {
  if (event === "TASK_STARTED") {
    return "started";
  }
  if (event === "TASK_PROGRESS") {
    const stage = readString(data.stage);
    if (stage) {
      return `progress:${stage}`;
    }
    const message = readString(data.message);
    if (/行情|上下文|K\s*线|数据|指标|资讯/.test(message)) {
      return "context";
    }
    return "model";
  }
  if (event === "TASK_CHUNK") {
    return "model";
  }
  if (event === "TASK_SUCCESS") {
    return "finished";
  }
  if (event === "TASK_FAILED") {
    return "failed";
  }
  if (event === "TASK_CANCELLED") {
    return "cancelled";
  }
  return "";
}

function stepTitle(stageKey: StepStageKey, event: TaskLogEventType, data: Record<string, unknown>): string {
  if (stageKey === "started") {
    return "任务已开始";
  }
  if (stageKey === "context") {
    return readString(data.message) || "任务处理中";
  }
  if (stageKey === "model") {
    return event === "TASK_CHUNK" ? "生成分析内容" : readString(data.message) || "调用 AI 模型";
  }
  if (stageKey === "finished") {
    return "任务已完成";
  }
  if (stageKey === "failed") {
    return "任务失败";
  }
  if (stageKey === "cancelled") {
    return "任务已取消";
  }
  return readString(data.message) || "任务日志";
}

function eventDescription(event: TaskLogEventType, data: Record<string, unknown>): string {
  if (event === "TASK_CREATED") {
    return readString(data.message) || "分析任务已创建。";
  }
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

function parseTaskTitle(title: string): Pick<RunningTaskSummary, "stockName" | "stockCode" | "analysisType"> {
  const trimmed = title.trim();
  const match = trimmed.match(/^(.+?)\s+(stock_full|technical|fundamental|news|ANALYSIS)$/i);
  if (!match) {
    return { stockName: "", stockCode: "", analysisType: "" };
  }
  return {
    stockName: "",
    stockCode: match[1].trim(),
    analysisType: match[2].trim(),
  };
}

function analysisTypeLabelFromValue(value: string): string {
  switch (value) {
    case "stock_full":
      return "个股综合分析";
    case "technical":
      return "技术面分析";
    case "fundamental":
      return "基本面分析";
    case "news":
      return "消息面分析";
    default:
      return value;
  }
}

function formatRunningTaskTitle(stockName: string, stockCode: string, analysisTypeLabel: string, fallback = "AI 分析任务"): string {
  const normalizedName = stockName.trim();
  const normalizedCode = stockCode.trim();
  const normalizedAnalysisType = analysisTypeLabel.trim();
  if (normalizedName && normalizedCode) {
    return `${normalizedName}（${normalizedCode}）${normalizedAnalysisType ? ` ${normalizedAnalysisType}` : ""}`;
  }
  if (normalizedName) {
    return `${normalizedName}${normalizedAnalysisType ? ` ${normalizedAnalysisType}` : ""}`;
  }
  if (normalizedCode) {
    return `${normalizedCode}${normalizedAnalysisType ? ` ${normalizedAnalysisType}` : ""}`;
  }
  return fallback;
}

function readString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function readChunkContent(data: Record<string, unknown>): string {
  return readString(data.content) || readString(data.markdown) || readString(data.chunk);
}

function readReportMarkdown(report: unknown): string {
  if (!report || typeof report !== "object") {
    return "";
  }
  const data = report as { content_markdown?: unknown; content_md?: unknown; content?: unknown };
  return readString(data.content_markdown) || readString(data.content_md) || readString(data.content);
}

function parseEventPayload(payload: RawRunningTaskEvent["payload"]): Record<string, unknown> {
  if (!payload) {
    return {};
  }
  if (typeof payload !== "string") {
    return payload;
  }
  try {
    const parsed = JSON.parse(payload) as unknown;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed as Record<string, unknown> : {};
  } catch {
    return {};
  }
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
