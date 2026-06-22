import { App as AntApp, Button, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { RecentReport } from "../types";

export function RecentReportsCard() {
  const { message } = AntApp.useApp();
  const recentReports: RecentReport[] = [];
  const columns: ColumnsType<RecentReport> = [
    {
      title: "报告标题",
      dataIndex: "title",
      ellipsis: true,
      width: 188,
      render: (value: string) => <span className="dashboard-link-text">{value}</span>,
    },
    { title: "关联股票", dataIndex: "stock", width: 92 },
    {
      title: "分析类型",
      dataIndex: "analysisType",
      width: 92,
      render: (value: string) => <ReportTypeTag value={value} />,
    },
    { title: "模型", dataIndex: "model", width: 82, ellipsis: true },
    { title: "生成时间", dataIndex: "generatedAt", width: 84 },
    {
      title: "操作",
      key: "action",
      width: 54,
      render: () => (
        <Button className="dashboard-table-link" type="link" onClick={() => message.info("查看报告待接入")}>
          查看
        </Button>
      ),
    },
  ];

  return (
    <section className="dashboard-surface dashboard-table-card-static">
      <div className="dashboard-card-title">
        <h2>最近分析报告</h2>
      </div>
      <Table
        className="dashboard-table dashboard-compact-table"
        columns={columns}
        dataSource={recentReports}
        pagination={false}
        rowKey="id"
        scroll={{ x: 592 }}
        size="small"
        tableLayout="fixed"
      />
      <button className="dashboard-card-more" type="button" onClick={() => message.info("报告历史待接入")}>
        查看全部报告 <span>›</span>
      </button>
    </section>
  );
}

function ReportTypeTag({ value }: { value: string }) {
  const className =
    value === "深度分析"
      ? "dashboard-report-tag blue"
      : value === "基本面分析"
        ? "dashboard-report-tag green"
        : value === "技术面分析"
          ? "dashboard-report-tag orange"
          : "dashboard-report-tag gray";
  return <Tag className={className}>{value}</Tag>;
}
