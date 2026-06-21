import type { Key } from "react";
import { useMemo, useState } from "react";
import { App as AntApp, Button } from "antd";
import { InfoCircleFilled, PlusOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { initialReportFilters, initialReportItems } from "./mock";
import type { ReportFilters, ReportItem } from "./types";
import { ReportFilterCard } from "./components/ReportFilterCard";
import { ReportStatsPanel } from "./components/ReportStatsPanel";
import { ReportTableCard } from "./components/ReportTableCard";

const statusLabelMap: Record<ReportItem["status"], ReportFilters["status"]> = {
  success: "成功",
  failed: "失败",
  running: "生成中",
};

export function ReportHistoryPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const [filters, setFilters] = useState<ReportFilters>(initialReportFilters);
  const [appliedFilters, setAppliedFilters] = useState<ReportFilters>(initialReportFilters);
  const [reports, setReports] = useState<ReportItem[]>(initialReportItems);
  const [selectedRowKeys, setSelectedRowKeys] = useState<Key[]>([]);

  const visibleReports = useMemo(() => {
    const keyword = appliedFilters.keyword.trim().toLowerCase();
    return reports.filter((report) => {
      const keywordMatched =
        !keyword ||
        report.title.toLowerCase().includes(keyword) ||
        report.stockName.toLowerCase().includes(keyword) ||
        report.stockCode.toLowerCase().includes(keyword);
      const analysisTypeMatched = appliedFilters.analysisType === "全部类型" || report.analysisType === appliedFilters.analysisType;
      const modelMatched = appliedFilters.model === "全部模型" || report.model === appliedFilters.model;
      const statusMatched = appliedFilters.status === "全部状态" || statusLabelMap[report.status] === appliedFilters.status;
      const reportDate = report.generatedAt.slice(0, 10);
      const startMatched = !appliedFilters.dateRangeStart || reportDate >= appliedFilters.dateRangeStart;
      const endMatched = !appliedFilters.dateRangeEnd || reportDate <= appliedFilters.dateRangeEnd;

      return keywordMatched && analysisTypeMatched && modelMatched && statusMatched && startMatched && endMatched;
    });
  }, [appliedFilters, reports]);

  const updateFilters = (patch: Partial<ReportFilters>) => setFilters((current) => ({ ...current, ...patch }));

  const handleQuery = () => {
    setAppliedFilters(filters);
    message.success("查询完成");
  };

  const handleReset = () => {
    setFilters(initialReportFilters);
    setAppliedFilters(initialReportFilters);
    message.success("筛选条件已重置");
  };

  const handleDelete = (report: ReportItem) => {
    setReports((current) => current.filter((item) => item.id !== report.id));
    setSelectedRowKeys((current) => current.filter((key) => key !== report.id));
    message.success("报告已删除");
  };

  const handleFavoriteToggle = (report: ReportItem) => {
    setReports((current) => current.map((item) => (item.id === report.id ? { ...item, favorite: !item.favorite } : item)));
  };

  const handleView = (report: ReportItem) => {
    navigate(`/reports/${encodeURIComponent(report.id)}`);
  };

  return (
    <main className="report-history-page">
      <header className="report-history-title-row">
        <div>
          <h1>分析报告历史</h1>
          <p>查看、筛选、复制和导出历史投研报告</p>
        </div>
        <Button type="primary" className="report-new-analysis-button" icon={<PlusOutlined />} onClick={() => { window.location.hash = "#/analysis"; }}>
          新建分析
        </Button>
      </header>

      <ReportFilterCard filters={filters} onChange={updateFilters} onReset={handleReset} onQuery={handleQuery} />

      <div className="report-history-content-grid xl:grid-cols-[minmax(0,1fr)_300px]">
        <ReportTableCard
          reports={visibleReports}
          selectedRowKeys={selectedRowKeys}
          onSelectedRowKeysChange={setSelectedRowKeys}
          onBatchAction={() => message.info("批量操作待接入")}
          onRefresh={() => message.success("报告列表已刷新")}
          onView={handleView}
          onDownload={() => message.info("导出报告待接入")}
          onDelete={handleDelete}
          onFavoriteToggle={handleFavoriteToggle}
        />
        <ReportStatsPanel />
      </div>

      <ReportRiskNotice />
    </main>
  );
}

function ReportRiskNotice() {
  return (
    <div className="settings-basic-risk-notice report-history-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleFilled />
        <span>历史报告内容仅供研究参考，请结合原始数据独立判断。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
