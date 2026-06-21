import { SafetyCertificateOutlined } from "@ant-design/icons";

export function ReportRiskCard() {
  return (
    <section className="report-detail-card report-risk-card">
      <div className="report-risk-card-title">
        <SafetyCertificateOutlined />
        <h2>风险声明</h2>
      </div>
      <p>本报告由 AI 生成，仅提供研究参考，不构成任何投资建议。投资有风险，决策需谨慎。</p>
    </section>
  );
}
