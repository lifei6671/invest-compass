import { InfoCircleFilled, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { inputSnapshot, reportDetail, reportSections } from "./mock";
import { ReportDetailHeader } from "./components/ReportDetailHeader";
import { ReportMarkdownContent } from "./components/ReportMarkdownContent";
import { ReportSnapshotPanel } from "./components/ReportSnapshotPanel";
import { ReportTocPanel } from "./components/ReportTocPanel";
import type { ReportSection } from "./types";

export function ReportDetailPage() {
  const { message } = AntApp.useApp();
  const navigate = useNavigate();
  const [report, setReport] = useState(reportDetail);
  const [activeSection, setActiveSection] = useState(reportSections[0].id);

  const copyMarkdown = () => {
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

  return (
    <main className="report-detail-page">
      <ReportDetailHeader
        report={report}
        onBack={() => navigate("/reports")}
        onCopy={copyMarkdown}
        onExport={() => message.info("导出 Markdown 待接入")}
        onReanalyze={() => message.info("重新分析待接入")}
        onDelete={() => message.warning("删除报告待接入")}
        onFavoriteToggle={() => {
          setReport((current) => ({ ...current, favorite: !current.favorite }));
          message.success("收藏状态已更新");
        }}
      />

      <div className="report-detail-layout">
        <ReportTocPanel
          sections={reportSections}
          activeSection={activeSection}
          onSectionClick={handleSectionClick}
          onCollapse={() => message.info("目录折叠待接入")}
        />
        <ReportMarkdownContent />
        <ReportSnapshotPanel snapshot={inputSnapshot} onCopySnapshot={() => message.info("复制输入快照待接入")} />
      </div>

      <ReportDetailRiskNotice />
    </main>
  );
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
