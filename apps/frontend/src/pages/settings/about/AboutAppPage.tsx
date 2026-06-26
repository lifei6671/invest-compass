import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { useState } from "react";
import { checkUpdate as checkUpdateRequest, exportLogs as exportLogsRequest, selectDirectory } from "../../../services/coreClient";
import { AboutHeroCard } from "./components/AboutHeroCard";
import { AppInfoCard } from "./components/AppInfoCard";
import { CheckUpdateCard } from "./components/CheckUpdateCard";
import { LicenseStatusCard } from "./components/LicenseStatusCard";
import { LogDiagnosticCard } from "./components/LogDiagnosticCard";
import { OpenSourceLicenseCard } from "./components/OpenSourceLicenseCard";
import { StaticDocumentModal } from "./components/StaticDocumentModal";
import { UserManualCard } from "./components/UserManualCard";
import { appInfoItems, licenseInfo, resourceLinks, updateInfo } from "./types";
import type { StaticDocumentKey } from "./components/StaticDocumentModal";
import type { UpdateInfo } from "./types";

export function AboutAppPage() {
  const { message } = AntApp.useApp();
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [exportingLogs, setExportingLogs] = useState(false);
  const [currentUpdateInfo, setCurrentUpdateInfo] = useState<UpdateInfo>(updateInfo);
  const [activeDocument, setActiveDocument] = useState<StaticDocumentKey | null>(null);
  const licenseLink = resourceLinks.find((item) => item.type === "license")!;
  const manualLink = resourceLinks.find((item) => item.type === "manual")!;
  const logsLink = resourceLinks.find((item) => item.type === "logs")!;

  const checkUpdate = async () => {
    setCheckingUpdate(true);
    try {
      const result = await checkUpdateRequest();
      setCurrentUpdateInfo({
        currentVersion: result.current_version || updateInfo.currentVersion,
        latestVersion: result.latest_version || "—",
        updateStatus: result.has_new_version ? "available" : "latest",
        releaseDate: "—",
        releaseNoteStatus: result.release_notes_url ? "发布说明可用" : "—",
      });
      message.success(result.has_new_version ? "发现新版本" : "当前已是最新版本");
    } catch (error) {
      setCurrentUpdateInfo((current) => ({ ...current, updateStatus: "failed" }));
      message.error(error instanceof Error ? error.message : "检查更新失败");
    } finally {
      setCheckingUpdate(false);
    }
  };

  const exportLogs = async () => {
    setExportingLogs(true);
    try {
      const targetDir = await selectDirectory();
      if (!targetDir) {
        message.info("已取消导出日志");
        return;
      }
      const result = await exportLogsRequest(targetDir);
      message.success(`脱敏日志已导出：${result.file_name}`);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "日志导出失败");
    } finally {
      setExportingLogs(false);
    }
  };

  return (
    <>
      <AboutHeroCard />
      <div className="settings-about-info-grid">
        <AppInfoCard items={appInfoItems} />
        <CheckUpdateCard
          value={currentUpdateInfo}
          checking={checkingUpdate}
          onCheck={checkUpdate}
          onViewReleaseNote={() => setActiveDocument("release-notes")}
        />
        <LicenseStatusCard value={licenseInfo} />
      </div>
      <div className="settings-about-resource-grid">
        <OpenSourceLicenseCard
          description={licenseLink.description}
          actionText={licenseLink.actionText}
          onAction={() => setActiveDocument("license")}
        />
        <UserManualCard
          description={manualLink.description}
          actionText={manualLink.actionText}
          onAction={() => setActiveDocument("manual")}
        />
        <LogDiagnosticCard
          description={logsLink.description}
          actionText={logsLink.actionText}
          loading={exportingLogs}
          onAction={exportLogs}
        />
      </div>
      <AboutRiskNotice />
      <StaticDocumentModal documentKey={activeDocument} onClose={() => setActiveDocument(null)} />
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
