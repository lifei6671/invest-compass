import { App as AntApp, Progress, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { RecentTask, RecentTaskStatus } from "../types";

export function RecentTasksCard() {
  const { message } = AntApp.useApp();
  const recentTasks: RecentTask[] = [];
  const columns: ColumnsType<RecentTask> = [
    { title: "任务标题", dataIndex: "title", width: 152, ellipsis: true, render: (value: string) => <span className="font-medium text-[#111827]">{value}</span> },
    { title: "类型", dataIndex: "type", width: 90, ellipsis: true },
    { title: "状态", dataIndex: "status", width: 84, render: (value: RecentTaskStatus) => <TaskStatusTag value={value} /> },
    {
      title: "进度",
      dataIndex: "progress",
      width: 110,
      render: (value: number, record) => (
        <div className="dashboard-task-progress">
          <Progress percent={value} railColor="#e5eaf3" showInfo={false} size="small" strokeColor={record.status === "FAILED" ? "#ff4d4f" : record.status === "SUCCESS" ? "#16a34a" : "#1677ff"} />
          <span>{value}%</span>
        </div>
      ),
    },
    { title: "结果摘要", dataIndex: "summary", width: 150, ellipsis: true },
  ];

  return (
    <section className="dashboard-surface dashboard-table-card-static">
      <div className="dashboard-card-title">
        <h2>最近任务状态</h2>
      </div>
      <Table
        className="dashboard-table dashboard-compact-table"
        columns={columns}
        dataSource={recentTasks}
        pagination={false}
        rowKey="id"
        scroll={{ x: 586 }}
        size="small"
        tableLayout="fixed"
      />
      <button className="dashboard-card-more" type="button" onClick={() => message.info("任务历史待接入")}>
        查看全部任务 <span>›</span>
      </button>
    </section>
  );
}

function TaskStatusTag({ value }: { value: RecentTaskStatus }) {
  const className = value === "RUNNING" ? "dashboard-status-tag blue" : value === "SUCCESS" ? "dashboard-status-tag green" : "dashboard-status-tag red";
  return <Tag className={className}>{value}</Tag>;
}
