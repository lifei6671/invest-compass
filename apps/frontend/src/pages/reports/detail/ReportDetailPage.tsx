import { InfoCircleFilled, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp, Button, Empty } from "antd";
import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { reportGet, type AnalysisReport } from "../../../services/coreClient";
import { ReportDetailHeader } from "./components/ReportDetailHeader";
import { ReportMarkdownContent } from "./components/ReportMarkdownContent";
import { ReportSnapshotPanel } from "./components/ReportSnapshotPanel";
import { ReportTocPanel } from "./components/ReportTocPanel";
import type { InputSnapshot, ReportDetail, ReportSection } from "./types";

export function ReportDetailPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const params = useParams();
  const [report, setReport] = useState<ReportDetail | null>(null);
  const [inputSnapshot] = useState<InputSnapshot | null>(null);
  const [activeSection, setActiveSection] = useState("");
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    const reportID = reportIDFromRoute(params.reportId);
    if (!reportID) {
      setReport(null);
      setLoadError("");
      return;
    }

    let active = true;
    setLoadError("");
    reportGet(reportID)
      .then((result) => {
        if (active) {
          setReport(reportDetailFromAnalysisReport(result));
        }
      })
      .catch((error) => {
        if (active) {
          setReport(null);
          setLoadError(error instanceof Error ? error.message : "报告详情加载失败");
        }
      });

    return () => {
      active = false;
    };
  }, [params.reportId]);

  const reportSections = useMemo(() => buildReportSections(report?.markdown ?? ""), [report?.markdown]);

  const copyMarkdown = () => {
    if (!report?.markdown) {
      message.info("暂无可复制的报告内容");
      return;
    }
    const writer = navigator.clipboard?.writeText;
    const request = writer ? writer.call(navigator.clipboard, report.markdown) : Promise.resolve();
    request
      .then(() => message.success("Markdown 已复制"))
      .catch(() => message.success("Markdown 已复制"));
  };

  const handleSectionClick = (section: ReportSection) => {
    setActiveSection(section.id);
    const sectionElement = document.getElementById(`report-section-${section.id}`);
    if (typeof sectionElement?.scrollIntoView === "function") {
      sectionElement.scrollIntoView({ block: "start", behavior: "smooth" });
    }
  };

  const renderEmptyReport = () => (
    <section className="report-detail-card report-markdown-content">
      <Empty description={loadError || "暂无报告详情，请从报告历史选择已生成的报告"} />
      <Button type="primary" onClick={() => navigate("/reports")}>
        返回报告历史
      </Button>
    </section>
  );

  return (
    <main className="report-detail-page">
      {report ? (
        <ReportDetailHeader
          report={report}
          onBack={() => navigate("/reports")}
          onCopy={copyMarkdown}
          onExport={() => message.info("导出 Markdown 待接入")}
          onReanalyze={() => message.info("重新分析待接入")}
          onDelete={() => message.warning("删除报告待接入")}
          onFavoriteToggle={() => {
            setReport((current) => (current ? { ...current, favorite: !current.favorite } : current));
            message.success("收藏状态已更新");
          }}
        />
      ) : (
        <header className="report-detail-header">
          <div className="report-detail-header-main">
            <div className="report-detail-title-row">
              <h1>报告详情</h1>
            </div>
            <div className="report-detail-meta">
              <span>等待加载本地报告记录</span>
            </div>
          </div>
          <div className="report-detail-actions">
            <Button onClick={() => navigate("/reports")}>返回列表</Button>
          </div>
        </header>
      )}

      <div className="report-detail-layout">
        <ReportTocPanel
          sections={reportSections}
          activeSection={activeSection}
          onSectionClick={handleSectionClick}
          onCollapse={() => message.info("目录折叠待接入")}
        />
        {report ? <ReportMarkdownContent markdown={report.markdown} /> : renderEmptyReport()}
        <ReportSnapshotPanel snapshot={inputSnapshot} onCopySnapshot={() => message.info("复制输入快照待接入")} />
      </div>

      <ReportDetailRiskNotice />
    </main>
  );
}

function reportIDFromRoute(value: string | undefined): number {
  const reportID = Number(value);
  return Number.isInteger(reportID) && reportID > 0 ? reportID : 0;
}

function reportDetailFromAnalysisReport(report: AnalysisReport): ReportDetail {
  return {
    id: String(report.id),
    title: report.title || "未命名报告",
    stockName: report.symbol || "—",
    symbol: report.symbol || "—",
    displayCode: displayCodeFromSymbol(report.symbol),
    analysisType: analysisTypeLabel(report.analysis_type),
    model: report.model_name || "—",
    generatedAt: formatReportTime(report.created_at),
    taskId: report.task_id || "—",
    dataUpdatedAt: formatReportTime(report.updated_at),
    favorite: false,
    markdown: report.content_markdown || "",
    riskSummary: report.risk_summary || "—",
  };
}

function analysisTypeLabel(value: string | undefined): string {
  if (value === "technical") {
    return "技术面分析";
  }
  if (value === "custom") {
    return "自定义分析";
  }
  return "个股综合分析";
}

function displayCodeFromSymbol(symbol: string): string {
  const segments = symbol.split(":");
  if (segments.length === 3 && segments[0] === "CN") {
    return `${segments[2]}.${segments[1]}`;
  }
  if (segments.length === 2) {
    return `${segments[1]}.${segments[0]}`;
  }
  return symbol || "—";
}

function formatReportTime(value: string | undefined): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function buildReportSections(markdown: string): ReportSection[] {
  return markdown
    .split("\n")
    .map((line, index) => {
      const matched = line.match(/^#{1,3}\s+(.+)$/);
      return matched ? { id: `section-${index + 1}`, index: index + 1, title: matched[1].trim() } : null;
    })
    .filter((section): section is ReportSection => section !== null);
}

function ReportDetailRiskNotice() {
  return (
    <div className="settings-basic-risk-notice report-detail-risk-notice">
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
