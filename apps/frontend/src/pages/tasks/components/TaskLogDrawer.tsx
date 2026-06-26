import { useCallback, useEffect, useMemo, useRef, useState, type RefObject } from "react";
import {
  Alert,
  App as AntApp,
  Button,
  Checkbox,
  Drawer,
  Empty,
  Grid,
  Input,
  Select,
  Tag,
} from "antd";
import {
  CloseOutlined,
  DownloadOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  SearchOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import {
  selectDirectory,
  taskEvents,
  type TaskEventItem,
} from "../../../services/coreClient";
import {
  taskLogContext,
  taskLogDiagnosis,
  taskLogGet,
  taskLogSummary,
  taskLogsExport,
  taskLogsList,
  type TaskLogContextSummary,
  type TaskLogDiagnosis,
  type TaskLogRow,
  type TaskLogSummary as RemoteTaskLogSummary,
} from "../../../services/taskLogs";
import { taskStatusLabels } from "../types";
import type { TaskItem, TaskStatus } from "../types";
import type {
  TaskLogContextItem,
  TaskLogDiagnosisView,
  TaskLogDrawerProps,
  TaskLogEvent,
  TaskLogLevel,
  TaskLogRecord,
  TaskLogSummary,
  TaskLogTab,
} from "../taskLogTypes";
import { ExecutionLogTable } from "./ExecutionLogTable";
import { RawJsonPanel } from "./RawJsonPanel";
import { TaskLogSummaryCard } from "./TaskLogSummaryCard";
import { TaskLogTabs } from "./TaskLogTabs";

const statusColorMap: Record<TaskStatus, string> = {
  RUNNING: "blue",
  SUCCESS: "green",
  FAILED: "red",
  CANCELLED: "default",
};

const logLevelOptions: Array<"全部级别" | TaskLogLevel> = ["全部级别", "INFO", "WARN", "ERROR"];
const defaultStageOptions = [
  "全部阶段",
  "quote_fetch",
  "kline_fetch",
  "calc_macd",
  "prompt_build",
  "stream_timeout",
  "stream_failed",
];

function normalizeStatus(status: string | undefined): TaskStatus {
  if (status === "SUCCESS" || status === "FAILED" || status === "CANCELLED") {
    return status;
  }
  return "RUNNING";
}

function buildFallbackSummary(task: TaskItem): TaskLogSummary {
  return {
    title: task.title,
    taskId: task.id,
    taskType: task.taskType,
    stock: task.stockCode && task.stockName ? `${task.stockCode}    ${task.stockName}` : "—",
    model: task.model ?? "—",
    startedAt: task.startedAt?.includes("-") ? task.startedAt : `2025-05-20 ${task.startedAt}`,
    duration: task.duration,
    requestId: "—",
    traceId: "—",
    status: task.status,
  };
}

function mapRemoteSummary(remote: RemoteTaskLogSummary, task: TaskItem): TaskLogSummary {
  return {
    title: remote.title || task.title,
    taskId: remote.task_id || task.id,
    taskType: remote.task_type || task.taskType,
    stock: remote.stock || (task.stockCode && task.stockName ? `${task.stockCode}    ${task.stockName}` : "—"),
    model: remote.model || task.model || "—",
    startedAt: remote.started_at || buildFallbackSummary(task).startedAt,
    duration: remote.duration || task.duration,
    requestId: remote.request_id || "—",
    traceId: remote.trace_id || "—",
    status: normalizeStatus(remote.status),
  };
}

function mapLogRow(row: TaskLogRow): TaskLogRecord {
  return {
    id: row.id,
    time: row.time,
    level: row.level,
    module: row.module,
    stage: row.stage,
    message: row.message,
  };
}

function parseRawJson(rawJSON: string): Record<string, unknown> {
  try {
    const parsed = JSON.parse(rawJSON) as unknown;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
    return { value: parsed };
  } catch {
    return { raw_json: rawJSON };
  }
}

function formatTimeLabel(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleTimeString("zh-CN", { hour12: false });
}

function eventDescription(event: TaskEventItem): string {
  const data = event.data ?? {};
  const messageValue = data.message ?? data.description ?? data.summary;
  return typeof messageValue === "string" && messageValue.trim() ? messageValue : "任务事件已记录。";
}

function mapTaskEvent(event: TaskEventItem): TaskLogEvent {
  return {
    id: String(event.id),
    time: formatTimeLabel(event.created_at),
    eventType: event.event as TaskLogEvent["eventType"],
    description: eventDescription(event),
  };
}

function deriveEventsFromLogs(records: TaskLogRecord[]): TaskLogEvent[] {
  return records.map((record) => {
    const eventType: TaskLogEvent["eventType"] =
      record.level === "ERROR"
        ? "TASK_FAILED"
        : record.stage.includes("chunk")
          ? "TASK_CHUNK"
          : "TASK_PROGRESS";
    return {
      id: `log-${record.id}`,
      time: record.time,
      eventType,
      description: record.message,
    };
  });
}

function mergeRecords(previous: TaskLogRecord[], next: TaskLogRecord[]): TaskLogRecord[] {
  const merged = new Map<number, TaskLogRecord>();
  previous.forEach((record) => merged.set(record.id, record));
  next.forEach((record) => merged.set(record.id, record));
  return Array.from(merged.values()).sort((a, b) => a.id - b.id);
}

function logText(records: TaskLogRecord[]): string {
  return records.map((record) => `${record.time} [${record.level}] ${record.module}/${record.stage} ${record.message}`).join("\n");
}

function diagnosisFromRemote(value: TaskLogDiagnosis | null, task: TaskItem): TaskLogDiagnosisView {
  if (value) {
    return {
      summary: value.summary || "当前任务暂无错误诊断。",
      causes: value.causes?.length ? value.causes : ["暂无明确原因。"],
      suggestions: value.suggestions?.length ? value.suggestions : ["可稍后重试或检查模型配置。"],
      retryable: value.retryable,
      errorCode: value.error_code,
      errorStage: value.error_stage,
    };
  }
  if (task.status === "FAILED") {
    return {
      summary: task.errorSummary || "任务执行失败，暂无进一步诊断信息。",
      causes: ["模型服务响应超时或网络异常", "代理配置异常或模型不可用"],
      suggestions: ["检查代理设置并重新测试连接", "更换 AI 模型后重试", "稍后重新发起分析任务"],
      retryable: true,
    };
  }
  return {
    summary: "当前任务暂无错误诊断。",
    causes: ["未检测到失败日志。"],
    suggestions: ["任务异常时可在此查看诊断建议。"],
    retryable: false,
  };
}

function contextItemsFromRemote(value: TaskLogContextSummary | null, task: TaskItem): TaskLogContextItem[] {
  return [
    ["股票", value?.stock || (task.stockCode && task.stockName ? `${task.stockName} ${task.stockCode}` : "—")],
    ["分析类型", value?.analysis_type || (task.title.includes("技术面") ? "技术面分析" : "个股综合分析")],
    ["使用模型", value?.model || task.model || "—"],
    ["Prompt 模板", value?.prompt_template || "未提供"],
    ["行情数据", value?.quote_status || "未记录"],
    ["K线数据", value?.kline_status || "未记录"],
    ["技术指标", value?.indicator_status || "未记录"],
    ["新闻资讯", value?.news_status || "未记录"],
    ["用户持仓", value?.user_position || "未提供"],
    ["数据更新时间", value?.data_updated_at || "—"],
    ["报告生成时间", value?.report_created_at || "—"],
  ].map(([label, itemValue]) => ({ label, value: itemValue }));
}

export function TaskLogDrawer({ open, task, onClose, onRetryTask, onOpenProxySettings }: TaskLogDrawerProps) {
  const { message } = AntApp.useApp();
  const screens = Grid.useBreakpoint();
  const [activeTab, setActiveTab] = useState<TaskLogTab>("logs");
  const [keyword, setKeyword] = useState("");
  const [level, setLevel] = useState<"全部级别" | TaskLogLevel>("全部级别");
  const [stage, setStage] = useState("全部阶段");
  const [errorsOnly, setErrorsOnly] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [summary, setSummary] = useState<TaskLogSummary | null>(null);
  const [records, setRecords] = useState<TaskLogRecord[]>([]);
  const [events, setEvents] = useState<TaskLogEvent[]>([]);
  const [diagnosis, setDiagnosis] = useState<TaskLogDiagnosis | null>(null);
  const [contextSummary, setContextSummary] = useState<TaskLogContextSummary | null>(null);
  const [selectedRecordId, setSelectedRecordId] = useState<number | null>(null);
  const [rawJson, setRawJson] = useState<Record<string, unknown> | null>(null);
  const [nextAfterId, setNextAfterId] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [logsLoading, setLogsLoading] = useState(false);
  const [rawLoading, setRawLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const logTableViewportRef = useRef<HTMLDivElement>(null);

  const drawerWidth = screens.xl ? "clamp(620px, 48vw, 760px)" : screens.md ? "72vw" : "100vw";
  const stageSelectOptions = useMemo(() => {
    const values = new Set(defaultStageOptions);
    records.forEach((record) => values.add(record.stage));
    return Array.from(values).map((value) => ({ label: value, value }));
  }, [records]);

  const loadRawDetail = useCallback(
    async (id: number) => {
      setRawLoading(true);
      try {
        const detail = await taskLogGet(id);
        setSelectedRecordId(id);
        setRawJson(parseRawJson(detail.raw_json));
      } catch (error) {
        setRawJson(null);
        message.error(error instanceof Error ? error.message : "日志详情加载失败");
      } finally {
        setRawLoading(false);
      }
    },
    [message],
  );

  const loadLogs = useCallback(
    async (options?: { afterId?: number; append?: boolean; silent?: boolean }) => {
      if (!task) {
        return;
      }
      if (!options?.silent) {
        setLogsLoading(true);
      }
      try {
        const result = await taskLogsList({
          taskId: task.id,
          level: level === "全部级别" ? "" : level,
          stage: stage === "全部阶段" ? "" : stage,
          keyword,
          onlyError: errorsOnly,
          afterId: options?.afterId ?? 0,
          limit: 200,
        });
        const nextRecords = result.rows.map(mapLogRow);
        setRecords((current) => (options?.append ? mergeRecords(current, nextRecords) : nextRecords));
        setNextAfterId(result.next_after_id);
        setHasMore(result.has_more);
        setLoadError("");
        if (!options?.append) {
          const initialRecord = nextRecords.find((record) => record.level === "ERROR") ?? nextRecords[0] ?? null;
          setSelectedRecordId(initialRecord?.id ?? null);
          setRawJson(null);
          if (initialRecord) {
            void loadRawDetail(initialRecord.id);
          }
        }
      } catch (error) {
        setLoadError(error instanceof Error ? error.message : "任务日志加载失败");
      } finally {
        if (!options?.silent) {
          setLogsLoading(false);
        }
      }
    },
    [errorsOnly, keyword, level, loadRawDetail, stage, task],
  );

  useEffect(() => {
    if (!open || !task) {
      return;
    }
    setActiveTab("logs");
    setKeyword("");
    setLevel("全部级别");
    setStage("全部阶段");
    setErrorsOnly(false);
    setAutoScroll(true);
    setSummary(buildFallbackSummary(task));
    setRecords([]);
    setEvents([]);
    setDiagnosis(null);
    setContextSummary(null);
    setSelectedRecordId(null);
    setRawJson(null);
    setNextAfterId(0);
    setHasMore(false);
    setLoadError("");
  }, [open, task]);

  useEffect(() => {
    if (!open || !task) {
      return;
    }
    const currentTask = task;
    let cancelled = false;
    async function loadMeta() {
      const [summaryResult, diagnosisResult, contextResult, eventsResult] = await Promise.allSettled([
        taskLogSummary(currentTask.id),
        taskLogDiagnosis(currentTask.id),
        taskLogContext(currentTask.id),
        taskEvents(currentTask.id, 0),
      ]);
      if (cancelled) {
        return;
      }
      if (summaryResult.status === "fulfilled") {
        setSummary(mapRemoteSummary(summaryResult.value, currentTask));
      }
      if (diagnosisResult.status === "fulfilled") {
        setDiagnosis(diagnosisResult.value);
      }
      if (contextResult.status === "fulfilled") {
        setContextSummary(contextResult.value);
      }
      if (eventsResult.status === "fulfilled") {
        setEvents(eventsResult.value.items.map(mapTaskEvent));
      }
    }
    void loadMeta();
    return () => {
      cancelled = true;
    };
  }, [open, task]);

  useEffect(() => {
    if (!open || !task) {
      return;
    }
    void loadLogs({ afterId: 0, append: false });
  }, [open, task, keyword, level, stage, errorsOnly, loadLogs]);

  useEffect(() => {
    if (!open || !task || task.status !== "RUNNING") {
      return;
    }
    const timer = window.setInterval(() => {
      void loadLogs({ afterId: nextAfterId, append: true, silent: true });
    }, 3000);
    return () => window.clearInterval(timer);
  }, [loadLogs, nextAfterId, open, task]);

  useEffect(() => {
    if (!autoScroll) {
      return;
    }
    window.requestAnimationFrame(() => {
      const viewport = logTableViewportRef.current;
      if (viewport) {
        viewport.scrollTop = viewport.scrollHeight;
      }
    });
  }, [autoScroll, records.length]);

  const renderedEvents = events.length > 0 ? events : deriveEventsFromLogs(records);
  const diagnosisView = task ? diagnosisFromRemote(diagnosis, task) : null;
  const contextItems = task ? contextItemsFromRemote(contextSummary, task) : [];

  const handleCopyCurrentLogs = async () => {
    await navigator.clipboard.writeText(logText(records));
    message.success("当前日志已复制");
  };

  const handleExport = async () => {
    if (!task) {
      return;
    }
    const targetDir = await selectDirectory();
    if (!targetDir) {
      message.info("已取消导出");
      return;
    }
    const result = await taskLogsExport(task.id, targetDir);
    message.success(`脱敏日志已导出：${result.file_name}`);
  };

  return (
    <Drawer
      className="task-log-drawer"
      rootClassName="task-log-drawer-root"
      placement="right"
      open={open && Boolean(task)}
      size={drawerWidth}
      mask
      maskClosable
      keyboard
      closable={false}
      destroyOnHidden={false}
      onClose={onClose}
      styles={{ body: { padding: 0 } }}
    >
      {task && summary ? (
        <div className="task-log-drawer-content">
          <header className="task-log-drawer-header">
            <div>
              <h2>完整日志</h2>
              <p>任务执行详情与排障信息</p>
            </div>
            <div className="task-log-drawer-header-actions">
              <Tag color={statusColorMap[summary.status]}>{taskStatusLabels[summary.status]}</Tag>
              <button type="button" aria-label="关闭日志抽屉" onClick={onClose}>
                <CloseOutlined />
              </button>
            </div>
          </header>

          <TaskLogSummaryCard summary={summary} />
          <TaskLogTabs activeTab={activeTab} onChange={setActiveTab} />

          <div className="task-log-drawer-body">
            {activeTab === "events" ? <EventTimeline events={renderedEvents} /> : null}
            {activeTab === "logs" ? (
              <ExecutionLogsTab
                failed={task.status === "FAILED"}
                diagnosisSummary={diagnosisView?.summary}
                keyword={keyword}
                level={level}
                stage={stage}
                stageOptions={stageSelectOptions}
                errorsOnly={errorsOnly}
                autoScroll={autoScroll}
                records={records}
                loading={logsLoading}
                loadError={loadError}
                hasMore={hasMore}
                selectedRecordId={selectedRecordId}
                rawJson={rawJson}
                rawLoading={rawLoading}
                tableViewportRef={logTableViewportRef}
                onKeywordChange={setKeyword}
                onLevelChange={setLevel}
                onStageChange={setStage}
                onErrorsOnlyChange={setErrorsOnly}
                onAutoScrollChange={setAutoScroll}
                onSelectRecord={(record) => void loadRawDetail(record.id)}
              />
            ) : null}
            {activeTab === "diagnosis" && diagnosisView ? (
              <DiagnosisTab
                diagnosis={diagnosisView}
                task={task}
                onRetryTask={onRetryTask}
                onOpenProxySettings={onOpenProxySettings}
              />
            ) : null}
            {activeTab === "context" ? <ContextTab items={contextItems} /> : null}
          </div>

          <footer className="task-log-footer">
            <Button onClick={handleCopyCurrentLogs} disabled={records.length === 0}>
              复制当前日志
            </Button>
            <Button type="primary" icon={<DownloadOutlined />} onClick={handleExport}>
              导出脱敏日志
            </Button>
            <Button onClick={onClose}>关闭</Button>
          </footer>
        </div>
      ) : null}
    </Drawer>
  );
}

type ExecutionLogsTabProps = {
  failed: boolean;
  diagnosisSummary?: string;
  keyword: string;
  level: "全部级别" | TaskLogLevel;
  stage: string;
  stageOptions: Array<{ label: string; value: string }>;
  errorsOnly: boolean;
  autoScroll: boolean;
  records: TaskLogRecord[];
  loading: boolean;
  loadError: string;
  hasMore: boolean;
  selectedRecordId: number | null;
  rawJson: Record<string, unknown> | null;
  rawLoading: boolean;
  tableViewportRef: RefObject<HTMLDivElement | null>;
  onKeywordChange: (value: string) => void;
  onLevelChange: (value: "全部级别" | TaskLogLevel) => void;
  onStageChange: (value: string) => void;
  onErrorsOnlyChange: (value: boolean) => void;
  onAutoScrollChange: (value: boolean) => void;
  onSelectRecord: (record: TaskLogRecord) => void;
};

function ExecutionLogsTab(props: ExecutionLogsTabProps) {
  return (
    <>
      {props.failed ? (
        <Alert
          className="task-log-error-alert"
          type="error"
          showIcon
          title={`错误摘要：${props.diagnosisSummary || "任务执行失败，可检查代理、API Key 或更换模型后重试。"}`}
        />
      ) : null}

      {props.loadError ? (
        <Alert
          className="task-log-load-alert"
          type="warning"
          showIcon
          title="日志加载失败"
          description={props.loadError}
        />
      ) : null}

      <div className="task-log-toolbar">
        <Input
          className="task-log-search"
          allowClear
          prefix={<SearchOutlined />}
          placeholder="搜索日志关键字"
          value={props.keyword}
          onChange={(event) => props.onKeywordChange(event.target.value)}
        />
        <Select
          className="task-log-level-select"
          value={props.level}
          options={logLevelOptions.map((value) => ({ label: value, value }))}
          virtual={false}
          onChange={props.onLevelChange}
        />
        <Select
          className="task-log-stage-select"
          value={props.stage}
          options={props.stageOptions}
          virtual={false}
          onChange={props.onStageChange}
        />
        <Checkbox checked={props.errorsOnly} onChange={(event) => props.onErrorsOnlyChange(event.target.checked)}>
          仅看错误
        </Checkbox>
        <Button
          className={props.autoScroll ? "task-log-scroll-button-active" : ""}
          icon={<PlayCircleOutlined />}
          onClick={() => props.onAutoScrollChange(true)}
        >
          自动滚动
        </Button>
        <Button icon={<PauseCircleOutlined />} onClick={() => props.onAutoScrollChange(false)}>
          暂停滚动
        </Button>
      </div>

      <ExecutionLogTable
        records={props.records}
        loading={props.loading}
        selectedRecordId={props.selectedRecordId}
        viewportRef={props.tableViewportRef}
        onSelectRecord={props.onSelectRecord}
      />
      <div className="task-log-filter-count">
        当前显示 {props.records.length} 条日志{props.hasMore ? "，仍有更多可继续拉取" : ""}
      </div>
      {props.records.length === 0 && !props.loading ? (
        <Empty className="task-log-empty" image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无匹配日志" />
      ) : null}
      <RawJsonPanel value={props.rawJson} loading={props.rawLoading} />
    </>
  );
}

function EventTimeline({ events }: { events: TaskLogEvent[] }) {
  if (events.length === 0) {
    return <Empty className="task-log-empty" image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无事件流" />;
  }
  return (
    <section className="task-log-event-panel">
      {events.map((event) => (
        <div className={`task-log-event-item task-log-event-${event.eventType.toLowerCase()}`} key={event.id}>
          <time>{event.time}</time>
          <span />
          <div>
            <strong>{event.eventType}</strong>
            <p>{event.description}</p>
          </div>
        </div>
      ))}
    </section>
  );
}

function DiagnosisTab({
  diagnosis,
  task,
  onRetryTask,
  onOpenProxySettings,
}: {
  diagnosis: TaskLogDiagnosisView;
  task: TaskItem;
  onRetryTask?: (task: TaskItem) => void;
  onOpenProxySettings?: () => void;
}) {
  return (
    <section className="task-log-diagnosis">
      <Alert type={diagnosis.retryable ? "warning" : "info"} showIcon title={diagnosis.summary} />
      <div>
        <h3>可能原因</h3>
        <ol>
          {diagnosis.causes.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ol>
      </div>
      <div>
        <h3>建议处理</h3>
        <ol>
          {diagnosis.suggestions.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ol>
      </div>
      {onRetryTask || onOpenProxySettings ? (
        <div className="task-log-diagnosis-actions">
          {onRetryTask ? (
            <Button type="primary" icon={<ReloadOutlined />} onClick={() => onRetryTask(task)}>
              重新分析
            </Button>
          ) : null}
          {onOpenProxySettings ? (
            <Button icon={<SettingOutlined />} onClick={onOpenProxySettings}>
              检查代理设置
            </Button>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

function ContextTab({ items }: { items: TaskLogContextItem[] }) {
  return (
    <section className="task-log-context">
      {items.map((item) => (
        <div key={item.label}>
          <span>{item.label}</span>
          <strong>{item.value}</strong>
        </div>
      ))}
      <p>上下文摘要仅用于排障展示，已脱敏且不包含完整 Prompt、API Key 或用户隐私输入。</p>
    </section>
  );
}
