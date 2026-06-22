import type { Key } from "react";
import { useMemo, useState } from "react";
import { App as AntApp, Button } from "antd";
import { InfoCircleFilled, PlusOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { reportDelete, searchReports, type DocumentSearchItem } from "../../services/coreClient";
import type { ReportFilters, ReportItem } from "./types";
import { ReportFilterCard } from "./components/ReportFilterCard";
import { ReportStatsPanel } from "./components/ReportStatsPanel";
import { ReportTableCard } from "./components/ReportTableCard";

const statusLabelMap: Record<ReportItem["status"], ReportFilters["status"]> = {
  success: "成功",
  failed: "失败",
  running: "生成中",
};
const initialReportFilters: ReportFilters = {
  keyword: "",
  analysisType: "全部类型",
  model: "全部模型",
  dateRangeLabel: "",
  dateRangeStart: "",
  dateRangeEnd: "",
  status: "全部状态",
};

export function ReportHistoryPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const [filters, setFilters] = useState<ReportFilters>(initialReportFilters);
  const [appliedFilters, setAppliedFilters] = useState<ReportFilters>(initialReportFilters);
  const [reports, setReports] = useState<ReportItem[]>([]);
  const [remoteSearchMode, setRemoteSearchMode] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<Key[]>([]);

  const visibleReports = useMemo(() => {
    if (remoteSearchMode) {
      return reports;
    }
    const keyword = remoteSearchMode ? "" : appliedFilters.keyword.trim().toLowerCase();
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
  }, [appliedFilters, remoteSearchMode, reports]);

  const updateFilters = (patch: Partial<ReportFilters>) => setFilters((current) => ({ ...current, ...patch }));

  const handleQuery = async () => {
    const keyword = filters.keyword.trim();
    if (!keyword) {
      setAppliedFilters(filters);
      setRemoteSearchMode(false);
      message.success("查询完成");
      return;
    }
    try {
      const results = await searchReports({
        keyword,
        symbols: reportSearchSymbols(keyword),
        limit: 20,
        offset: 0,
        sort: "relevance",
      });
      setReports(results.filter((item) => item.doc_type === "report").map(reportItemFromSearchResult));
      setAppliedFilters(filters);
      setRemoteSearchMode(true);
      setSelectedRowKeys([]);
      message.success("查询完成");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "报告搜索失败");
    }
  };

  const handleReset = () => {
    setFilters(initialReportFilters);
    setAppliedFilters(initialReportFilters);
    setReports([]);
    setRemoteSearchMode(false);
    message.success("筛选条件已重置");
  };

  const handleDelete = async (report: ReportItem) => {
    const reportID = reportIDFromItem(report);
    if (!reportID) {
      message.error("无法识别报告 ID，删除失败");
      return;
    }

    try {
      await reportDelete(reportID);
      setReports((current) => current.filter((item) => item.id !== report.id));
      setSelectedRowKeys((current) => current.filter((key) => key !== report.id));
      message.success("报告已删除");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "报告删除失败");
    }
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
          <p>查看、筛选、复制和导出历史分析报告</p>
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

function reportItemFromSearchResult(item: DocumentSearchItem): ReportItem {
  return {
    id: item.ref_id || item.doc_uid,
    title: item.title,
    stockName: item.symbol || "—",
    stockCode: item.symbol || "—",
    analysisType: analysisTypeFromReportTitle(item.title),
    model: item.source || "—",
    generatedAt: formatSearchSourceTime(item.source_time),
    riskSummary: item.summary || "—",
    status: "success",
  };
}

function reportIDFromItem(report: ReportItem): number {
  const reportID = Number(report.id);
  return Number.isInteger(reportID) && reportID > 0 ? reportID : 0;
}

function analysisTypeFromReportTitle(title: string): ReportItem["analysisType"] {
  if (title.includes("技术")) {
    return "技术面分析";
  }
  if (title.includes("财务")) {
    return "财务分析";
  }
  if (title.includes("持仓")) {
    return "持仓分析";
  }
  return "个股综合分析";
}

function formatSearchSourceTime(value: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function reportSearchSymbols(keyword: string): string[] {
  const trimmed = keyword.trim().toUpperCase();
  if (/^[A-Z]{2}:[A-Z]{2}:[0-9]{6}$/.test(trimmed) || /^[A-Z]{2}:[0-9A-Z.-]+$/.test(trimmed)) {
    return [trimmed];
  }
  return [];
}
