import type { ColumnsType } from "antd/es/table";
import { Button, Pagination, Progress, Select, Table, Tag, Tooltip } from "antd";
import { FileTextOutlined, FolderOpenOutlined, ReloadOutlined, StopOutlined, UnorderedListOutlined } from "@ant-design/icons";
import { taskStatusLabels, type TaskItem, type TaskSortMode } from "../types";

type TaskTableCardProps = {
  tasks: TaskItem[];
  selectedTaskId: string;
  sortMode: TaskSortMode;
  onSortChange: (sortMode: TaskSortMode) => void;
  onSelectTask: (task: TaskItem) => void;
  onLog: (task: TaskItem) => void;
  onCancel: (task: TaskItem) => void;
  onDetail: (task: TaskItem) => void;
  onReport: () => void;
  onRetry: () => void;
  onColumnSettings: () => void;
};

const typeClassMap: Record<TaskItem["taskType"], string> = {
  "AI 分析": "task-type-ai",
  资讯同步: "task-type-news",
  行情刷新: "task-type-quote",
  缓存清理: "task-type-cache",
};

const statusClassMap: Record<TaskItem["status"], string> = {
  RUNNING: "task-status-running",
  SUCCESS: "task-status-success",
  FAILED: "task-status-failed",
  CANCELLED: "task-status-cancelled",
};

const progressStatus: Record<TaskItem["status"], "active" | "success" | "exception" | "normal"> = {
  RUNNING: "active",
  SUCCESS: "success",
  FAILED: "exception",
  CANCELLED: "normal",
};

const sortOptions: TaskSortMode[] = ["按开始时间倒序", "按开始时间升序", "按耗时倒序", "按状态排序"];

export function TaskTableCard(props: TaskTableCardProps) {
  const columns: ColumnsType<TaskItem> = [
    {
      title: "任务 ID",
      dataIndex: "id",
      width: 178,
      render: (id: string) => <span className="task-id-cell">{id}</span>,
    },
    {
      title: "任务类型",
      dataIndex: "taskType",
      width: 90,
      render: (type: TaskItem["taskType"]) => <span className={`task-type-tag ${typeClassMap[type]}`}>{type}</span>,
    },
    {
      title: "标题",
      dataIndex: "title",
      width: 170,
      render: (title: string) => <span className="task-title-cell">{title}</span>,
    },
    {
      title: "关联股票",
      width: 104,
      render: (_, task) => (
        <div className="task-stock-cell">
          <strong>{task.stockCode ?? "—"}</strong>
          {task.stockName ? <span>{task.stockName}</span> : null}
        </div>
      ),
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 92,
      render: (status: TaskItem["status"]) => <span className={`task-status-tag ${statusClassMap[status]}`}>{taskStatusLabels[status]}</span>,
    },
    {
      title: "进度",
      width: 112,
      render: (_, task) => <Progress percent={task.progress} size="small" status={progressStatus[task.status]} />,
    },
    { title: "开始时间", dataIndex: "startedAt", width: 84 },
    { title: "结束时间", dataIndex: "endedAt", width: 84 },
    { title: "耗时", dataIndex: "duration", width: 82 },
    {
      title: "错误摘要",
      dataIndex: "errorSummary",
      width: 128,
      render: (value: string | undefined) => <span className={value && value !== "—" ? "task-error-cell" : ""}>{value || "—"}</span>,
    },
    {
      title: "操作",
      width: 142,
      fixed: "right",
      render: (_, task) => (
        <div className="task-table-actions">
          <Tooltip title="查看日志">
            <Button type="text" size="small" icon={<FileTextOutlined />} onClick={(event) => { event.stopPropagation(); props.onLog(task); }}>
              日志
            </Button>
          </Tooltip>
          {task.status === "RUNNING" ? (
            <Tooltip title="取消任务">
              <Button type="text" size="small" icon={<StopOutlined />} onClick={(event) => { event.stopPropagation(); props.onCancel(task); }}>
                取消
              </Button>
            </Tooltip>
          ) : null}
          {task.status === "SUCCESS" ? (
            <Tooltip title={task.taskType === "AI 分析" ? "查看报告" : "查看详情"}>
              <Button
                type="text"
                size="small"
                icon={<FolderOpenOutlined />}
                onClick={(event) => {
                  event.stopPropagation();
                  if (task.taskType === "AI 分析") {
                    props.onReport();
                  } else {
                    props.onDetail(task);
                  }
                }}
              >
                {task.taskType === "AI 分析" ? "报告" : "详情"}
              </Button>
            </Tooltip>
          ) : null}
          {task.status === "FAILED" ? (
            <Tooltip title="重试任务">
              <Button type="text" size="small" icon={<ReloadOutlined />} onClick={(event) => { event.stopPropagation(); props.onRetry(); }}>
                重试
              </Button>
            </Tooltip>
          ) : null}
          {task.status === "CANCELLED" ? (
            <Tooltip title="查看详情">
              <Button type="text" size="small" icon={<FolderOpenOutlined />} onClick={(event) => { event.stopPropagation(); props.onDetail(task); }}>
                详情
              </Button>
            </Tooltip>
          ) : null}
        </div>
      ),
    },
  ];

  return (
    <section className="task-table-card">
      <header className="task-table-header">
        <div className="task-table-title">
          <h2>任务列表</h2>
          <span>（共 56 条）</span>
        </div>
        <div className="task-table-tools">
          <Select<TaskSortMode>
            className="task-sort-select"
            value={props.sortMode}
            options={sortOptions.map((value) => ({ label: value, value }))}
            onChange={props.onSortChange}
          />
          <Button icon={<UnorderedListOutlined />} onClick={props.onColumnSettings} />
        </div>
      </header>

      <Table<TaskItem>
        className="task-history-table"
        rowKey="id"
        size="small"
        columns={columns}
        dataSource={props.tasks}
        pagination={false}
        scroll={{ x: 1160, y: 428 }}
        rowClassName={(task) => (task.id === props.selectedTaskId ? "task-row-selected" : "")}
        onRow={(task) => ({
          onClick: () => props.onSelectTask(task),
        })}
      />

      <footer className="task-table-pagination">
        <span>共 56 条</span>
        <Pagination
          size="small"
          current={1}
          total={56}
          pageSize={20}
          onChange={() => undefined}
          showSizeChanger
          pageSizeOptions={[20]}
          showQuickJumper={{ goButton: "页" }}
          locale={{ items_per_page: "条/页", jump_to: "跳至", page: "页" }}
        />
      </footer>
    </section>
  );
}
