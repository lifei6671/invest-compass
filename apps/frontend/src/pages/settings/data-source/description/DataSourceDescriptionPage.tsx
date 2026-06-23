import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { AIContextExplanationCard } from "./components/AIContextExplanationCard";
import { DataComplianceBoundaryCard } from "./components/DataComplianceBoundaryCard";
import { DataFAQCard } from "./components/DataFAQCard";
import { DataFreshnessCard } from "./components/DataFreshnessCard";
import { DataSourceExplanationCard } from "./components/DataSourceExplanationCard";
import { DataUsageOverviewCard } from "./components/DataUsageOverviewCard";
import { FieldDescriptionCard } from "./components/FieldDescriptionCard";
import {
  aiContextDataTypes,
  aiOutputNatures,
  dataUsageOverview,
  faqItems,
  fieldDescriptions,
  freshnessItems,
  sourceItems,
} from "./staticData";

type DataSourceDescriptionPageProps = {
  onViewOverview: () => void;
  onViewCredentials: () => void;
};

export function DataSourceDescriptionPage({ onViewOverview, onViewCredentials }: DataSourceDescriptionPageProps) {
  const { message } = AntApp.useApp();

  return (
    <>
      <div className="data-description-workspace">
        <DataUsageOverviewCard value={dataUsageOverview} onViewOverview={onViewOverview} />
        <div className="data-description-card-grid">
          <DataSourceExplanationCard items={sourceItems} onViewDataSource={onViewOverview} />
          <DataFreshnessCard items={freshnessItems} />
          <AIContextExplanationCard dataTypes={aiContextDataTypes} outputNatures={aiOutputNatures} onViewCredentials={onViewCredentials} />
          <DataComplianceBoundaryCard />
          <FieldDescriptionCard items={fieldDescriptions} onViewMore={() => message.info("字段说明待接入")} />
          <DataFAQCard items={faqItems} onOpenFAQ={() => message.info("FAQ 详情待接入")} onViewMore={() => message.info("更多 FAQ 待接入")} />
        </div>
      </div>
      <DataDescriptionRiskNotice />
    </>
  );
}

function DataDescriptionRiskNotice() {
  return (
    <div className="settings-basic-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleOutlined />
        <span>敏感凭据仅保存在本地安全存储；日志导出前将自动清理 API Key 与代理密码。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
