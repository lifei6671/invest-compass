import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";

export function SettingsRiskNotice() {
  return (
    <div className="settings-basic-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleOutlined />
        <span>敏感信息会脱敏保存；日志导出前将自动清理 API Key 与代理密码。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
