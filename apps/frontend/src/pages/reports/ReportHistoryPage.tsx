import type { Key } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { App as AntApp, Button } from "antd";
import { InfoCircleFilled, PlusOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import {
  reportBatchDelete,
  reportDelete,
  reportList,
  reportStats as fetchReportStats,
  searchReports,
  type AnalysisReport,
  type DocumentSearchItem,
  type ReportStatsResult,
  reportExport,
  reportUpdate,
} from "../../services/coreClient";
import type { AnalysisTypeDistributionItem, ReportFilters, ReportItem, ReportStats, TopModelItem } from "./types";
import { ReportFilterCard } from "./components/ReportFilterCard";
import { ReportStatsPanel } from "./components/ReportStatsPanel";
import { ReportTableCard } from "./components/ReportTableCard";
import { DEFAULT_PAGE_SIZE } from "../../lib/pagination";

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
  const [loading, setLoading] = useState(false);
  const [remoteSearchMode, setRemoteSearchMode] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<Key[]>([]);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [reportStats, setReportStats] = useState<ReportStats>(emptyReportStats);
  const [topModels, setTopModels] = useState<TopModelItem[]>([]);
  const [analysisTypeDistribution, setAnalysisTypeDistribution] = useState<AnalysisTypeDistributionItem[]>([]);
  const requestSerialRef = useRef(0);

  const loadReports = useCallback(
    async (showSuccessMessage = false) => {
      setLoading(true);
      const requestSerial = ++requestSerialRef.current;
      try {
        const result = await reportList();
        if (requestSerial !== requestSerialRef.current) {
          return false;
        }
        setReports(result.items.map(reportItemFromAnalysisReport));
        setRemoteSearchMode(false);
        setSelectedRowKeys([]);
        setCurrentPage(1);
        void fetchReportStats()
          .then((stats) => {
            if (requestSerial === requestSerialRef.current) {
              applyReportStats(stats, setReportStats, setTopModels, setAnalysisTypeDistribution);
            }
          })
          .catch((error) => {
            if (requestSerial === requestSerialRef.current) {
              message.error(error instanceof Error ? error.message : "报告统计加载失败");
            }
          });
        if (showSuccessMessage) {
          message.success("报告列表已刷新");
        }
        return true;
      } catch (error) {
        message.error(error instanceof Error ? error.message : "报告列表加载失败");
        return false;
      } finally {
        if (requestSerial === requestSerialRef.current) {
          setLoading(false);
        }
      }
    },
    [message],
  );

  const loadReportStats = useCallback(async () => {
    try {
      applyReportStats(await fetchReportStats(), setReportStats, setTopModels, setAnalysisTypeDistribution);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "报告统计加载失败");
    }
  }, [message]);

  useEffect(() => {
    void loadReports();
  }, [loadReports]);

  const visibleReports = useMemo(() => {
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
      const reportDate = report.generatedDate;
      const startMatched = !appliedFilters.dateRangeStart || reportDate >= appliedFilters.dateRangeStart;
      const endMatched = !appliedFilters.dateRangeEnd || reportDate <= appliedFilters.dateRangeEnd;

      return keywordMatched && analysisTypeMatched && modelMatched && statusMatched && startMatched && endMatched;
    });
  }, [appliedFilters, remoteSearchMode, reports]);

  const modelOptions = useMemo(() => ["全部模型", ...Array.from(new Set(reports.map((report) => report.model).filter(Boolean)))], [reports]);
  const pagedReports = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return visibleReports.slice(start, start + pageSize);
  }, [currentPage, pageSize, visibleReports]);

  const updateFilters = (patch: Partial<ReportFilters>) => setFilters((current) => ({ ...current, ...patch }));

  const handleQuery = async () => {
    const keyword = filters.keyword.trim();
    if (!keyword) {
      setAppliedFilters(filters);
      const loaded = await loadReports();
      if (loaded) {
        message.success("查询完成");
      }
      return;
    }
    try {
      const requestSerial = ++requestSerialRef.current;
      const results = await searchReports({
        keyword,
        symbols: reportSearchSymbols(keyword),
        limit: 20,
        offset: 0,
        sort: "relevance",
      });
      if (requestSerial !== requestSerialRef.current) {
        return;
      }
      setReports(results.filter((item) => item.doc_type === "report").map(reportItemFromSearchResult));
      setAppliedFilters(filters);
      setRemoteSearchMode(true);
      setSelectedRowKeys([]);
      setCurrentPage(1);
      message.success("查询完成");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "报告搜索失败");
    }
  };

  const handleReset = async () => {
    setFilters(initialReportFilters);
    setAppliedFilters(initialReportFilters);
    const loaded = await loadReports();
    if (loaded) {
      message.success("筛选条件已重置");
    }
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
      void loadReportStats();
      message.success("报告已删除");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "报告删除失败");
    }
  };

  const handleBatchAction = async (action: "export" | "delete") => {
    if (action === "export") {
      message.info("批量导出将在第二期接入");
      return;
    }
    const ids = selectedRowKeys.map((key) => Number(key)).filter((id) => Number.isInteger(id) && id > 0);
    if (ids.length === 0) {
      message.warning("请先选择要删除的报告");
      return;
    }
    try {
      await reportBatchDelete(ids);
      const deleted = new Set(ids.map(String));
      setReports((current) => current.filter((report) => !deleted.has(report.id)));
      setSelectedRowKeys([]);
      void loadReportStats();
      message.success("已批量删除报告");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "批量删除报告失败");
    }
  };

  const handleFavoriteToggle = async (report: ReportItem) => {
    const reportID = reportIDFromItem(report);
    if (!reportID) {
      message.error("无法识别报告 ID，收藏失败");
      return;
    }
    const nextFavorite = !report.favorite;
    try {
      await reportUpdate(reportID, nextFavorite);
      setReports((current) => current.map((item) => (item.id === report.id ? { ...item, favorite: nextFavorite } : item)));
      message.success(nextFavorite ? "已收藏报告" : "已取消收藏");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "收藏状态更新失败");
    }
  };

  const handleDownload = async (report: ReportItem) => {
    const reportID = reportIDFromItem(report);
    if (!reportID) {
      message.error("无法识别报告 ID，导出失败");
      return;
    }
    try {
      const result = await reportExport(reportID);
      if (!result.saved) {
        message.info("已取消导出");
        return;
      }
      message.success(`报告已导出：${result.file_name}`);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "报告导出失败");
    }
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

      <ReportFilterCard filters={filters} modelOptions={modelOptions} onChange={updateFilters} onReset={handleReset} onQuery={handleQuery} />

      <div className="report-history-content-grid xl:grid-cols-[minmax(0,1fr)_300px]">
        <ReportTableCard
          reports={pagedReports}
          loading={loading}
          total={visibleReports.length}
          currentPage={currentPage}
          pageSize={pageSize}
          selectedRowKeys={selectedRowKeys}
          onSelectedRowKeysChange={setSelectedRowKeys}
          onBatchAction={handleBatchAction}
          onRefresh={() => void loadReports(true)}
          onPageChange={(page, nextPageSize) => {
            setCurrentPage(page);
            setPageSize(nextPageSize);
          }}
          onView={handleView}
          onDownload={handleDownload}
          onDelete={handleDelete}
          onFavoriteToggle={handleFavoriteToggle}
        />
        <ReportStatsPanel stats={reportStats} topModels={topModels} analysisTypeDistribution={analysisTypeDistribution} />
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
    generatedDate: reportDateKey(item.source_time),
    riskSummary: item.summary || "—",
    status: "success",
    favorite: false,
  };
}

function reportItemFromAnalysisReport(report: AnalysisReport): ReportItem {
  const generatedValue = report.created_at || report.updated_at || "";
  return {
    id: String(report.id),
    title: report.title || "未命名报告",
    stockName: displayStockName(report.symbol),
    stockCode: report.symbol || "—",
    analysisType: analysisTypeFromBackend(report.analysis_type),
    model: report.model_name || "—",
    generatedAt: formatReportTime(generatedValue),
    generatedDate: reportDateKey(generatedValue),
    riskSummary: report.risk_summary || "—",
    status: "success",
    favorite: Boolean(report.favorite),
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

function analysisTypeFromBackend(value: string | undefined): ReportItem["analysisType"] {
  switch ((value || "").toLowerCase()) {
    case "technical":
      return "技术面分析";
    case "fundamental":
      return "财务分析";
    case "position":
      return "持仓分析";
    default:
      return "个股综合分析";
  }
}

function displayStockName(symbol: string): string {
  return symbol || "—";
}

function formatSearchSourceTime(value: string): string {
  return formatReportTime(value);
}

function formatReportTime(value: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function reportDateKey(value: string): string {
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

const emptyReportStats: ReportStats = {
  totalCount: 0,
  uniqueSymbols: 0,
  latestCreatedAt: "",
};

function applyReportStats(
  stats: ReportStatsResult,
  setStats: (stats: ReportStats) => void,
  setModels: (models: TopModelItem[]) => void,
  setDistribution: (items: AnalysisTypeDistributionItem[]) => void,
) {
  setStats({
    totalCount: stats.total,
    uniqueSymbols: stats.unique_symbols,
    latestCreatedAt: formatReportTime(stats.latest_created_at || ""),
  });
  setModels(
    stats.top_models.slice(0, 5).map((item) => ({
      name: item.name,
      count: item.count,
      percent: stats.total > 0 ? Math.round((item.count / stats.total) * 100) : 0,
    })),
  );
  setDistribution(buildAnalysisTypeDistribution(stats.analysis_types, stats.total));
}

function buildAnalysisTypeDistribution(stats: ReportStatsResult["analysis_types"], total: number): AnalysisTypeDistributionItem[] {
  const colors: Record<ReportItem["analysisType"], string> = {
    个股综合分析: "#2563eb",
    技术面分析: "#16a34a",
    财务分析: "#f97316",
    持仓分析: "#7c3aed",
  };
  return stats.map((item) => {
    const type = analysisTypeFromBackend(item.name);
    return {
    type,
    value: item.count,
    percent: total > 0 ? (item.count / total) * 100 : 0,
    color: colors[type],
    };
  });
}

function reportSearchSymbols(keyword: string): string[] {
  const trimmed = keyword.trim().toUpperCase();
  if (/^[A-Z]{2}:[A-Z]{2}:[0-9]{6}$/.test(trimmed) || /^[A-Z]{2}:[0-9A-Z.-]+$/.test(trimmed)) {
    return [trimmed];
  }
  return [];
}
