import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { AboutHeroCard } from "./components/AboutHeroCard";
import { AppInfoCard } from "./components/AppInfoCard";
import { CheckUpdateCard } from "./components/CheckUpdateCard";
import { LicenseStatusCard } from "./components/LicenseStatusCard";
import { LogDiagnosticCard } from "./components/LogDiagnosticCard";
import { OpenSourceLicenseCard } from "./components/OpenSourceLicenseCard";
import { UserManualCard } from "./components/UserManualCard";
import { appInfoItems, licenseInfo, resourceLinks, updateInfo } from "./types";

export function AboutAppPage() {
  const { message } = AntApp.useApp();
  const licenseLink = resourceLinks.find((item) => item.type === "license")!;
  const manualLink = resourceLinks.find((item) => item.type === "manual")!;
  const logsLink = resourceLinks.find((item) => item.type === "logs")!;

  const checkUpdate = () => {
    message.info("检查更新待接入");
  };

  return (
    <>
      <AboutHeroCard />
      <div className="settings-about-info-grid">
        <AppInfoCard items={appInfoItems} />
        <CheckUpdateCard
          value={updateInfo}
          checking={false}
          onCheck={checkUpdate}
          onViewReleaseNote={() => message.info("发布说明待接入")}
        />
        <LicenseStatusCard value={licenseInfo} />
      </div>
      <div className="settings-about-resource-grid">
        <OpenSourceLicenseCard
          description={licenseLink.description}
          actionText={licenseLink.actionText}
          onAction={() => message.info("LICENSE 查看待接入")}
        />
        <UserManualCard
          description={manualLink.description}
          actionText={manualLink.actionText}
          onAction={() => message.info("用户手册待接入")}
        />
        <LogDiagnosticCard
          description={logsLink.description}
          actionText={logsLink.actionText}
          onAction={() => message.info("日志导出待接入")}
        />
      </div>
      <AboutRiskNotice />
    </>
  );
}

function AboutRiskNotice() {
  return (
    <div className="settings-basic-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleOutlined />
        <span>本应用定位为投研辅助工具，输出内容不构成投资建议。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
