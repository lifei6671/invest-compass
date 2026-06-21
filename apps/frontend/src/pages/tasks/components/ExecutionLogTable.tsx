import type { ColumnsType } from "antd/es/table";
import { Table } from "antd";
import type { TaskLogLevel, TaskLogRecord } from "../taskLogTypes";

type ExecutionLogTableProps = {
  records: TaskLogRecord[];
};

const levelClassMap: Record<TaskLogLevel, string> = {
  INFO: "task-log-level-info",
  WARN: "task-log-level-warn",
  ERROR: "task-log-level-error",
};

export function ExecutionLogTable({ records }: ExecutionLogTableProps) {
  const columns: ColumnsType<TaskLogRecord> = [
    { title: "时间", dataIndex: "time", width: 112 },
    {
      title: "级别",
      dataIndex: "level",
      width: 82,
      render: (level: TaskLogLevel) => <span className={`task-log-level-tag ${levelClassMap[level]}`}>{level}</span>,
    },
    { title: "模块", dataIndex: "module", width: 88 },
    { title: "阶段", dataIndex: "stage", width: 136 },
    { title: "消息", dataIndex: "message", ellipsis: true },
  ];

  return (
    <Table<TaskLogRecord>
      className="task-log-table"
      rowKey="id"
      size="small"
      columns={columns}
      dataSource={records}
      pagination={false}
      rowClassName={(record) => (record.level === "ERROR" ? "task-log-row-error" : "")}
    />
  );
}
