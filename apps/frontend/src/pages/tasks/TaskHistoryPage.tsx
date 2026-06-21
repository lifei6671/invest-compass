import { useMemo, useState } from "react";
import { App as AntApp, Drawer, Grid } from "antd";
import { InfoCircleFilled, SafetyCertificateOutlined } from "@ant-design/icons";
import { initialTaskFilters, initialTaskItems, taskEvents, taskSummary } from "./mock";
import type { TaskFilters, TaskItem, TaskSortMode } from "./types";
import { TaskDetailPanel } from "./components/TaskDetailPanel";
import { TaskFilterCard } from "./components/TaskFilterCard";
import { TaskLogDrawer } from "./components/TaskLogDrawer";
import { TaskSummaryCards } from "./components/TaskSummaryCards";
import { TaskTableCard } from "./components/TaskTableCard";

export function TaskHistoryPage() {
  const { message } = AntApp.useApp();
  const screens = Grid.useBreakpoint();
  const [filters, setFilters] = useState<TaskFilters>(initialTaskFilters);
  const [appliedFilters, setAppliedFilters] = useState<TaskFilters>(initialTaskFilters);
  const [tasks, setTasks] = useState<TaskItem[]>(initialTaskItems);
  const [selectedTaskId, setSelectedTaskId] = useState(initialTaskItems[0]?.id ?? "");
  const [detailOpen, setDetailOpen] = useState(false);
  const [logDrawerOpen, setLogDrawerOpen] = useState(false);
  const [sortMode, setSortMode] = useState<TaskSortMode>("按开始时间倒序");

  const selectedTask = tasks.find((task) => task.id === selectedTaskId) ?? tasks[0];
  const drawerWidth = screens.xl ? "50vw" : "100vw";

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
      const taskDate = `${task.id.slice(5, 9)}-${task.id.slice(9, 11)}-${task.id.slice(11, 13)}`;
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

  const updateFilters = (patch: Partial<TaskFilters>) => setFilters((current) => ({ ...current, ...patch }));

  const handleQuery = () => {
    setAppliedFilters(filters);
    message.success("查询完成");
  };

  const handleReset = () => {
    setFilters(initialTaskFilters);
    setAppliedFilters(initialTaskFilters);
    message.success("筛选条件已重置");
  };

  const handleSortChange = (nextSortMode: TaskSortMode) => {
    setSortMode(nextSortMode);
    message.success("排序已更新");
  };

  const handleSelectTask = (task: TaskItem) => {
    setSelectedTaskId(task.id);
    setLogDrawerOpen(false);
    setDetailOpen(true);
  };

  const handleOpenTaskLog = (task: TaskItem) => {
    setSelectedTaskId(task.id);
    setDetailOpen(false);
    setLogDrawerOpen(true);
  };

  const handleCancelTask = (task: TaskItem) => {
    setTasks((current) =>
      current.map((item) =>
        item.id === task.id
          ? { ...item, status: "CANCELLED", progress: Math.max(item.progress, 45), endedAt: "—", errorSummary: "用户取消" }
          : item,
      ),
    );
    setSelectedTaskId(task.id);
    setDetailOpen(true);
    message.warning("任务已取消");
  };

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
          <p>跟踪 AI 分析、行情刷新、资讯同步和缓存清理任务</p>
        </div>
        <TaskSummaryCards summary={taskSummary} />
      </header>

      <TaskFilterCard filters={filters} onChange={updateFilters} onReset={handleReset} onQuery={handleQuery} />

      <div className="task-history-content-grid">
        <TaskTableCard
          tasks={visibleTasks}
          selectedTaskId={selectedTask?.id ?? ""}
          sortMode={sortMode}
          onSortChange={handleSortChange}
          onSelectTask={handleSelectTask}
          onLog={handleOpenTaskLog}
          onCancel={handleCancelTask}
          onDetail={handleSelectTask}
          onReport={() => message.info("查看报告待接入")}
          onRetry={() => message.info("任务重试待接入")}
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
            events={taskEvents}
            onClose={handleCloseDetail}
            onFullLog={() => {
              setDetailOpen(false);
              setLogDrawerOpen(true);
            }}
          />
        ) : null}
      </Drawer>

      <TaskLogDrawer open={logDrawerOpen} task={selectedTask ?? null} onClose={handleCloseLogDrawer} />

      <TaskRiskNotice />
    </main>
  );
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
