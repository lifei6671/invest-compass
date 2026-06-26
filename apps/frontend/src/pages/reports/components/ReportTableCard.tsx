import type { Key } from "react";
import { Button, Pagination, Select, Table, Tag, Tooltip } from "antd";
import type { ColumnsType } from "antd/es/table";
import { DeleteOutlined, DownloadOutlined, EyeOutlined, ReloadOutlined, StarFilled, StarOutlined } from "@ant-design/icons";
import type { ReportItem, ReportStatus } from "../types";
import { PAGE_SIZE_OPTIONS } from "../../../lib/pagination";

type ReportTableCardProps = {
  reports: ReportItem[];
  loading: boolean;
  total: number;
  currentPage: number;
  pageSize: number;
  selectedRowKeys: Key[];
  onSelectedRowKeysChange: (keys: Key[]) => void;
  onBatchAction: (action: "export" | "delete") => void;
  onRefresh: () => void;
  onPageChange: (page: number, pageSize: number) => void;
  onView: (report: ReportItem) => void;
  onDownload: (report: ReportItem) => void;
  onDelete: (report: ReportItem) => void;
  onFavoriteToggle: (report: ReportItem) => void;
};

const statusMeta: Record<ReportStatus, { label: string; colorClass: string }> = {
  success: { label: "成功", colorClass: "report-status-success" },
  failed: { label: "失败", colorClass: "report-status-failed" },
  running: { label: "生成中", colorClass: "report-status-running" },
};

export function ReportTableCard({
  reports,
  loading,
  total,
  currentPage,
  pageSize,
  selectedRowKeys,
  onSelectedRowKeysChange,
  onBatchAction,
  onRefresh,
  onPageChange,
  onView,
  onDownload,
  onDelete,
  onFavoriteToggle,
}: ReportTableCardProps) {
  const columns: ColumnsType<ReportItem> = [
    {
      title: "报告标题",
      dataIndex: "title",
      width: 190,
      render: (_, report) => (
        <div className="report-title-cell">
          <button type="button" className="report-title-link" onClick={() => onView(report)}>
            {report.title}
          </button>
          <Button
            type="text"
            size="small"
            className="report-favorite-button"
            aria-label={`${report.favorite ? "取消收藏" : "收藏"} ${report.title}`}
            icon={report.favorite ? <StarFilled /> : <StarOutlined />}
            onClick={() => onFavoriteToggle(report)}
          />
        </div>
      ),
    },
    {
      title: "关联股票",
      dataIndex: "stockName",
      width: 120,
      render: (_, report) => (
        <div className="report-stock-cell">
          <strong>{report.stockName}</strong>
          <span>{report.stockCode}</span>
        </div>
      ),
    },
    {
      title: "分析类型",
      dataIndex: "analysisType",
      width: 110,
    },
    {
      title: "使用模型",
      dataIndex: "model",
      width: 120,
    },
    {
      title: "生成时间",
      dataIndex: "generatedAt",
      width: 132,
      sorter: false,
      render: (value) => (
        <span className="report-generated-time">
          {value}
          <span className="report-sort-arrow">↓</span>
        </span>
      ),
    },
    {
      title: "风险摘要",
      dataIndex: "riskSummary",
      width: 200,
      ellipsis: true,
      render: (value) => <span className="report-risk-summary">{value}</span>,
    },
    {
      title: "任务状态",
      dataIndex: "status",
      width: 88,
      render: (status: ReportStatus) => <Tag className={statusMeta[status].colorClass}>{statusMeta[status].label}</Tag>,
    },
    {
      title: "操作",
      key: "actions",
      width: 116,
      fixed: "right",
      render: (_, report) => (
        <div className="report-table-actions">
          <Tooltip title="查看报告">
            <Button type="text" size="small" aria-label={`查看 ${report.title}`} icon={<EyeOutlined />} onClick={() => onView(report)} />
          </Tooltip>
          <Tooltip title="导出报告">
            <Button type="text" size="small" aria-label={`下载 ${report.title}`} icon={<DownloadOutlined />} onClick={() => onDownload(report)} />
          </Tooltip>
          <Tooltip title="删除报告">
            <Button type="text" danger size="small" aria-label={`删除 ${report.title}`} icon={<DeleteOutlined />} onClick={() => onDelete(report)} />
          </Tooltip>
        </div>
      ),
    },
  ];

  return (
    <section className="report-table-card">
      <div className="report-table-header">
        <div className="report-table-title">
          <h2>报告列表</h2>
          <span>（共 {total} 条）</span>
        </div>
        <div className="report-table-tools">
          <Select
            size="small"
            placeholder="批量操作"
            className="report-batch-select"
            onSelect={onBatchAction}
            options={[
              { value: "export", label: "批量导出", disabled: true },
              { value: "delete", label: "批量删除" },
            ]}
          />
          <Button size="small" icon={<ReloadOutlined />} aria-label="刷新报告列表" onClick={onRefresh} />
        </div>
      </div>

      <Table<ReportItem>
        className="report-history-table"
        rowKey="id"
        size="small"
        columns={columns}
        dataSource={reports}
        loading={loading}
        pagination={false}
        scroll={{ x: 1080, y: 430 }}
        rowSelection={{
          selectedRowKeys,
          onChange: onSelectedRowKeysChange,
        }}
      />

      <div className="report-pagination-row">
        <Pagination
          size="small"
          total={total}
          current={currentPage}
          pageSize={pageSize}
          onChange={onPageChange}
          showSizeChanger
          pageSizeOptions={[...PAGE_SIZE_OPTIONS]}
          showQuickJumper={{ goButton: "确定" }}
          showTotal={(total) => `共 ${total} 条`}
          locale={{
            items_per_page: "条/页",
            jump_to: "跳至",
            jump_to_confirm: "确定",
            page: "页",
            prev_page: "上一页",
            next_page: "下一页",
            prev_5: "向前 5 页",
            next_5: "向后 5 页",
            prev_3: "向前 3 页",
            next_3: "向后 3 页",
            page_size: "页码",
          }}
        />
      </div>
    </section>
  );
}
