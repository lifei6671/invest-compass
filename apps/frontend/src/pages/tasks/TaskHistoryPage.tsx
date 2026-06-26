import { useCallback, useEffect, useMemo, useState } from "react";
import { App as AntApp, Drawer, Grid } from "antd";
import { InfoCircleFilled, SafetyCertificateOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import type { TaskEvent, TaskFilters, TaskItem, TaskSortMode, TaskSummary } from "./types";
import { TaskDetailPanel } from "./components/TaskDetailPanel";
import { TaskFilterCard } from "./components/TaskFilterCard";
import { TaskLogDrawer } from "./components/TaskLogDrawer";
import { TaskSummaryCards } from "./components/TaskSummaryCards";
import { TaskTableCard } from "./components/TaskTableCard";
import {
  aiConfigList,
  analysisTaskCancel,
  analysisTaskCreate,
  taskEvents,
  taskGet,
  taskList,
  type TaskEventItem as CoreTaskEventItem,
  type TaskItem as CoreTaskItem,
} from "../../services/coreClient";
import { DEFAULT_PAGE_SIZE } from "../../lib/pagination";

type RetryTaskPayload = {
  symbol: string;
  analysisType: "stock_full" | "technical";
  aiConfigID: number;
  promptTemplateID: number;
  hasUserPosition: boolean;
};

const initialTaskFilters: TaskFilters = {
  taskType: "全部类型",
  status: "全部状态",
  dateRangeLabel: "近7天",
  dateRangeStart: "",
  dateRangeEnd: "",
  keyword: "",
};

export function TaskHistoryPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const screens = Grid.useBreakpoint();
  const [filters, setFilters] = useState<TaskFilters>(initialTaskFilters);
  const [appliedFilters, setAppliedFilters] = useState<TaskFilters>(initialTaskFilters);
  const [tasks, setTasks] = useState<TaskItem[]>([]);
  const [taskEventItems, setTaskEventItems] = useState<TaskEvent[]>([]);
  const [selectedTaskId, setSelectedTaskId] = useState("");
  const [detailOpen, setDetailOpen] = useState(false);
  const [logDrawerOpen, setLogDrawerOpen] = useState(false);
  const [sortMode, setSortMode] = useState<TaskSortMode>("按开始时间倒序");
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);

  const selectedTask = tasks.find((task) => task.id === selectedTaskId) ?? tasks[0];
  const drawerWidth = screens.xl ? "50vw" : "100vw";
  const taskSummary = useMemo<TaskSummary>(
    () => ({
      runningCount: tasks.filter((task) => task.status === "RUNNING").length,
      successTodayCount: tasks.filter((task) => task.status === "SUCCESS").length,
      failedCount: tasks.filter((task) => task.status === "FAILED").length,
    }),
    [tasks],
  );

  const visibleTasks = useMemo(() => {
    const keyword = appliedFilters.keyword.trim().toLowerCase();
    const filtered = tasks.filter((task) => {
      const keywordMatched =
        !keyword ||
        task.id.toLowerCase().includes(keyword) ||
        task.title.toLowerCase().includes(keyword) ||
        task.stockCode?.toLowerCase().includes(keyword) ||
        task.stockName?.toLowerCase().includes(keyword);
      const typeMatched = appliedFilters.taskType === "全部类型" || task.taskType === appliedFilters.taskType;
      const statusMatched = appliedFilters.status === "全部状态" || task.status === appliedFilters.status;
      const taskDate = task.startedDate;
      const startMatched = !appliedFilters.dateRangeStart || taskDate >= appliedFilters.dateRangeStart;
      const endMatched = !appliedFilters.dateRangeEnd || taskDate <= appliedFilters.dateRangeEnd;
      return keywordMatched && typeMatched && statusMatched && startMatched && endMatched;
    });

    const sorted = [...filtered];
    if (sortMode === "按开始时间升序") {
      sorted.reverse();
    }
    if (sortMode === "按耗时倒序") {
      sorted.sort((left, right) => right.duration.localeCompare(left.duration));
    }
    if (sortMode === "按状态排序") {
      sorted.sort((left, right) => left.status.localeCompare(right.status));
    }
    return sorted;
  }, [appliedFilters, sortMode, tasks]);

  const loadTasks = useCallback(async () => {
    try {
      const result = await taskList(100);
      setTasks(result.items.map(mapCoreTaskItem));
    } catch (error) {
      message.error(error instanceof Error ? error.message : "任务列表加载失败");
    }
  }, [message]);

  const pagedTasks = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return visibleTasks.slice(start, start + pageSize);
  }, [currentPage, pageSize, visibleTasks]);

  useEffect(() => {
    const maxPage = Math.max(1, Math.ceil(visibleTasks.length / pageSize));
    if (currentPage > maxPage) {
      setCurrentPage(maxPage);
    }
  }, [currentPage, pageSize, visibleTasks.length]);

  useEffect(() => {
    void loadTasks();
  }, [loadTasks]);

  const updateFilters = (patch: Partial<TaskFilters>) => setFilters((current) => ({ ...current, ...patch }));

  const handleQuery = () => {
    setAppliedFilters(filters);
    setCurrentPage(1);
    message.success("查询完成");
  };

  const handleReset = () => {
    setFilters(initialTaskFilters);
    setAppliedFilters(initialTaskFilters);
    setCurrentPage(1);
    message.success("筛选条件已重置");
  };

  const handleSortChange = (nextSortMode: TaskSortMode) => {
    setSortMode(nextSortMode);
    setCurrentPage(1);
    message.success("排序已更新");
  };

  const handlePageChange = (page: number, nextPageSize: number) => {
    setPageSize(nextPageSize);
    setCurrentPage(nextPageSize === pageSize ? page : 1);
  };

  const loadTaskDetail = useCallback(
    async (task: TaskItem) => {
      try {
        const [detail, events] = await Promise.all([taskGet(task.id), taskEvents(task.id, 0)]);
        const mappedDetail = mapCoreTaskItem(detail);
        setTasks((current) => current.map((item) => (item.id === mappedDetail.id ? mappedDetail : item)));
        setTaskEventItems(events.items.map(mapCoreTaskEventItem));
      } catch (error) {
        setTaskEventItems([]);
        message.error(error instanceof Error ? error.message : "任务详情加载失败");
      }
    },
    [message],
  );

  const handleSelectTask = (task: TaskItem) => {
    setSelectedTaskId(task.id);
    setLogDrawerOpen(false);
    setDetailOpen(true);
    void loadTaskDetail(task);
  };

  const handleOpenTaskLog = (task: TaskItem) => {
    setSelectedTaskId(task.id);
    setDetailOpen(false);
    setLogDrawerOpen(true);
  };

  const handleCancelTask = async (task: TaskItem) => {
    try {
      const result = await analysisTaskCancel(task.id);
      setTasks((current) =>
        current.map((item) =>
          item.id === task.id
            ? { ...item, status: normalizeTaskStatus(result.status), progress: Math.max(item.progress, 45), endedAt: "—", errorSummary: "用户取消" }
            : item,
        ),
      );
      setSelectedTaskId(task.id);
      setDetailOpen(true);
      message.warning("任务已取消");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "任务取消失败");
    }
  };

  const handleOpenReport = useCallback(
    (task: TaskItem) => {
      if (!task.reportId) {
        message.info("报告尚未生成");
        return;
      }
      navigate(`/reports/${encodeURIComponent(task.reportId)}`);
    },
    [message, navigate],
  );

  const handleRetryTask = useCallback(
    async (task: TaskItem) => {
      if (task.taskType !== "AI 分析") {
        message.info("当前任务类型不支持重试");
        return;
      }
      try {
        const events = await taskEvents(task.id, 0);
        const createdEvent = events.items.find((item) => item.event === "TASK_CREATED");
        const retryPayload = createdEvent ? parseRetryTaskPayload(createdEvent.data) : null;
        if (!retryPayload) {
          message.error("原任务缺少可重试参数，请重新创建分析任务");
          return;
        }
        if (retryPayload.hasUserPosition) {
          message.error("原任务包含一次性持仓输入，无法自动重试，请重新创建分析任务");
          return;
        }

        const configs = await aiConfigList();
        const model = configs.items.find((item) => item.id === retryPayload.aiConfigID);
        if (!model?.has_api_key || !model.api_key_ref) {
          message.error("原任务使用的 AI 模型缺少可用凭据，请先检查模型配置");
          return;
        }

        const result = await analysisTaskCreate({
          symbol: retryPayload.symbol,
          analysis_type: retryPayload.analysisType,
          ai_config_id: retryPayload.aiConfigID,
          api_key_ref: model.api_key_ref,
          prompt_template_id: retryPayload.promptTemplateID,
          retry_of_task_id: task.id,
          user_position: null,
        });
        message.success("重试任务已创建");
        navigate(`/analysis/running?taskId=${encodeURIComponent(result.task_id)}`);
      } catch (error) {
        message.error(error instanceof Error ? error.message : "任务重试失败");
      }
    },
    [message, navigate],
  );

  const handleCloseDetail = () => {
    setDetailOpen(false);
  };

  const handleCloseLogDrawer = () => {
    setLogDrawerOpen(false);
  };

  return (
    <main className="task-history-page">
      <header className="task-history-title-row">
        <div>
          <h1>任务历史</h1>
          <p>跟踪 AI 分析、行情刷新、资讯同步、缓存清理和数据重建任务</p>
        </div>
        <TaskSummaryCards summary={taskSummary} />
      </header>

      <TaskFilterCard filters={filters} onChange={updateFilters} onReset={handleReset} onQuery={handleQuery} />

      <div className="task-history-content-grid">
        <TaskTableCard
          tasks={pagedTasks}
          total={visibleTasks.length}
          currentPage={currentPage}
          pageSize={pageSize}
          selectedTaskId={selectedTask?.id ?? ""}
          sortMode={sortMode}
          onPageChange={handlePageChange}
          onSortChange={handleSortChange}
          onSelectTask={handleSelectTask}
          onLog={handleOpenTaskLog}
          onCancel={handleCancelTask}
          onDetail={handleSelectTask}
          onReport={handleOpenReport}
          onRetry={handleRetryTask}
          onColumnSettings={() => message.info("列设置待接入")}
        />
      </div>

      <Drawer
        className="task-detail-drawer"
        rootClassName="task-detail-drawer-root"
        placement="right"
        open={detailOpen && Boolean(selectedTask)}
        closable={false}
        destroyOnHidden
        maskClosable
        onClose={handleCloseDetail}
        styles={{ wrapper: { width: drawerWidth }, body: { padding: 0 } }}
      >
        {selectedTask ? (
          <TaskDetailPanel
            task={selectedTask}
            events={taskEventItems}
            onClose={handleCloseDetail}
            onFullLog={() => {
              setDetailOpen(false);
              setLogDrawerOpen(true);
            }}
          />
        ) : null}
      </Drawer>

      <TaskLogDrawer
        open={logDrawerOpen}
        task={selectedTask ?? null}
        onClose={handleCloseLogDrawer}
        onRetryTask={handleRetryTask}
        onOpenProxySettings={() => navigate("/settings?tab=proxy")}
      />

      <TaskRiskNotice />
    </main>
  );
}

function mapCoreTaskItem(item: CoreTaskItem): TaskItem {
  const startedValue = item.started_at || item.created_at || "";
  const finishedValue = item.finished_at || item.updated_at || "";
  return {
    id: item.id,
    taskType: normalizeTaskType(item.type),
    title: item.title || item.id,
    status: normalizeTaskStatus(item.status),
    progress: normalizeProgress(item.progress),
    startedAt: formatTaskDateTime(startedValue),
    startedDate: taskDateKey(startedValue),
    endedAt: formatTaskDateTime(finishedValue),
    duration: formatTaskDuration(startedValue, item.finished_at),
    errorSummary: item.error_message || "—",
    reportId: item.report_id,
  };
}

function mapCoreTaskEventItem(item: CoreTaskEventItem): TaskEvent {
  return {
    id: String(item.id),
    time: formatTaskTime(item.created_at),
    eventType: normalizeTaskEventType(item.event),
    description: eventDescription(item.data),
  };
}

function normalizeTaskType(value: string): TaskItem["taskType"] {
  const normalized = value.toUpperCase();
  if (normalized.includes("NEWS")) {
    return "资讯同步";
  }
  if (normalized.includes("QUOTE") || normalized.includes("MARKET")) {
    return "行情刷新";
  }
  if (normalized.includes("CACHE")) {
    return "缓存清理";
  }
  if (normalized.includes("SEARCH") || normalized.includes("REBUILD") || normalized.includes("INDEX")) {
    return "数据重建";
  }
  return "AI 分析";
}

function normalizeTaskStatus(value: string): TaskItem["status"] {
  if (value === "SUCCESS" || value === "FAILED" || value === "CANCELLED") {
    return value;
  }
  return "RUNNING";
}

function normalizeTaskEventType(value: string): TaskEvent["eventType"] {
  if (
    value === "TASK_CREATED" ||
    value === "TASK_STARTED" ||
    value === "TASK_PROGRESS" ||
    value === "TASK_LOG" ||
    value === "TASK_CHUNK" ||
    value === "TASK_SUCCESS" ||
    value === "TASK_FAILED" ||
    value === "TASK_CANCELLED"
  ) {
    return value;
  }
  return "TASK_LOG";
}

function normalizeProgress(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.min(100, Math.round(value)));
}

function formatTaskTime(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleTimeString("zh-CN", { hour12: false });
}

function formatTaskDateTime(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function taskDateKey(value?: string): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return /^\d{4}-\d{2}-\d{2}/.test(value) ? value.slice(0, 10) : "";
  }
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function formatTaskDuration(startedAt?: string, finishedAt?: string): string {
  if (!startedAt || !finishedAt) {
    return "运行中";
  }
  const started = new Date(startedAt);
  const finished = new Date(finishedAt);
  if (Number.isNaN(started.getTime()) || Number.isNaN(finished.getTime())) {
    return "—";
  }
  const seconds = Math.max(0, Math.round((finished.getTime() - started.getTime()) / 1000));
  return `${seconds}s`;
}

function eventDescription(data: Record<string, unknown>): string {
  const value = data.message ?? data.description ?? data.summary;
  return typeof value === "string" && value.trim() ? value : "任务事件已记录。";
}

function parseRetryTaskPayload(data: Record<string, unknown>): RetryTaskPayload | null {
  const symbol = data.symbol;
  const analysisType = data.analysis_type;
  const aiConfigID = data.ai_config_id;
  const promptTemplateID = data.prompt_template_id;
  if (typeof symbol !== "string" || !symbol.trim()) {
    return null;
  }
  if (analysisType !== "stock_full" && analysisType !== "technical") {
    return null;
  }
  if (!Number.isFinite(aiConfigID) || !Number.isFinite(promptTemplateID)) {
    return null;
  }
  return {
    symbol: symbol.trim(),
    analysisType,
    aiConfigID: Number(aiConfigID),
    promptTemplateID: Number(promptTemplateID),
    hasUserPosition: data.has_user_position === true,
  };
}

function TaskRiskNotice() {
  return (
    <div className="settings-basic-risk-notice task-history-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleFilled />
        <span>历史任务记录在本地，仅供研究使用。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>历史任务仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
