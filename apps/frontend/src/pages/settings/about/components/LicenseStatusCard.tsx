import { SafetyCertificateFilled } from "@ant-design/icons";
import type { LicenseInfo } from "../types";

type LicenseStatusCardProps = {
  value: LicenseInfo;
};

export function LicenseStatusCard(props: LicenseStatusCardProps) {
  return (
    <section className="settings-basic-card settings-about-status-card">
      <header className="settings-about-card-title">
        <span>
          <SafetyCertificateFilled />
        </span>
        <h2>授权状态</h2>
      </header>
      <div className="settings-about-license-list">
        <div className="settings-about-license-row">
          <span>当前授权状态</span>
          <strong className="settings-about-license-status">{props.value.status}</strong>
        </div>
        <div className="settings-about-license-row settings-about-license-desc-row">
          <span>授权说明</span>
          <strong>{props.value.description}</strong>
        </div>
        <div className="settings-about-license-row">
          <span>授权类型</span>
          <strong>{props.value.licenseType}</strong>
        </div>
        <div className="settings-about-license-row">
          <span>到期时间</span>
          <strong>{props.value.expiresAt}</strong>
        </div>
      </div>
    </section>
  );
}
