import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  credentialConfigs,
  credentialHealthItems,
  credentialOperationLogs,
  credentialOverview,
  credentialProviders,
  credentialTestResult,
  credentialTestTargets,
  selectedProviderId as initialSelectedProviderId,
} from "./mock";
import type { CredentialAuthType, CredentialConfig, CredentialTestResult, DataSourceProvider } from "./types";
import { CredentialConfigCard } from "./components/CredentialConfigCard";
import { CredentialConnectionTestCard } from "./components/CredentialConnectionTestCard";
import { CredentialHealthCard } from "./components/CredentialHealthCard";
import { CredentialOperationLogCard } from "./components/CredentialOperationLogCard";
import { CredentialOverviewCard } from "./components/CredentialOverviewCard";
import { CredentialProviderList } from "./components/CredentialProviderList";
import { CredentialSecurityCard } from "./components/CredentialSecurityCard";

export function DataSourceCredentialPage() {
  const { message, modal } = AntApp.useApp();
  const [providers, setProviders] = useState(credentialProviders);
  const [configs, setConfigs] = useState<Record<string, CredentialConfig>>(credentialConfigs);
  const [selectedProviderId, setSelectedProviderId] = useState(initialSelectedProviderId);
  const [testTarget, setTestTarget] = useState(credentialTestTargets[0].value);
  const [testResult, setTestResult] = useState<CredentialTestResult>(credentialTestResult);
  const [testing, setTesting] = useState(false);
  const testTimerRef = useRef<number | null>(null);

  useEffect(
    () => () => {
      if (testTimerRef.current !== null) {
        window.clearTimeout(testTimerRef.current);
      }
    },
    [],
  );

  const selectedConfig = configs[selectedProviderId] ?? configs[initialSelectedProviderId];
  const selectedProvider = useMemo(() => providers.find((item) => item.id === selectedProviderId) ?? providers[0], [providers, selectedProviderId]);

  const updateSelectedConfig = (value: CredentialConfig) => {
    setConfigs((current) => ({ ...current, [value.providerId]: value }));
  };

  const runLocalConnectionTest = () => {
    if (testTimerRef.current !== null) {
      window.clearTimeout(testTimerRef.current);
    }
    setTesting(true);
    testTimerRef.current = window.setTimeout(() => {
      testTimerRef.current = null;
      setTesting(false);
      setTestResult({
        status: "success",
        responseTimeMs: 186,
        testedAt: "2025-05-20 15:28:41",
        messages: ["行情接口可访问", "新闻接口已授权"],
      });
      message.success("连接测试完成");
    }, 600);
  };

  const selectProvider = (provider: DataSourceProvider) => {
    setSelectedProviderId(provider.id);
    setTestResult(provider.status === "not_configured" ? { status: "untested", messages: ["当前 Provider 尚未配置凭据"] } : credentialTestResult);
    message.info("已切换 Provider");
  };

  const changeAuthType = (authType: CredentialAuthType) => {
    updateSelectedConfig({ ...selectedConfig, authType });
    message.info("认证方式已更新");
  };

  const saveCredential = () => {
    if (!selectedConfig.providerName.trim() || !selectedConfig.baseUrl.trim() || !selectedConfig.authType) {
      message.error("请填写 Provider 名称、Base URL 和认证方式");
      return;
    }
    setProviders((current) => current.map((item) => (item.id === selectedConfig.providerId ? { ...item, status: "normal", authType: selectedConfig.authType } : item)));
    updateSelectedConfig({ ...selectedConfig, credentialStatus: "normal" });
    message.success("凭据配置已保存");
  };

  const clearCredential = () => {
    modal.confirm({
      title: `确认清除 ${selectedProvider.name} 的凭据？`,
      content: "清除后只更新当前页面本地状态，不会访问真实数据源或本地存储。",
      okText: "确认清除",
      cancelText: "取消",
      onOk: () => {
        setProviders((current) => current.map((item) => (item.id === selectedProviderId ? { ...item, status: "not_configured" } : item)));
        updateSelectedConfig({ ...selectedConfig, credentialStatus: "not_configured", maskedCredential: "" });
        setTestResult({ status: "untested", messages: ["凭据已清除，尚未重新测试"] });
        message.success("凭据已清除");
      },
    });
  };

  return (
    <>
      <div className="credential-workspace">
        <div className="credential-main-grid">
          <CredentialProviderList items={providers} selectedProviderId={selectedProviderId} onSelect={selectProvider} />
          <CredentialConfigCard value={selectedConfig} testing={testing} onChange={updateSelectedConfig} onAuthTypeChange={changeAuthType} onSave={saveCredential} onTest={runLocalConnectionTest} onClear={clearCredential} />
          <div className="credential-side-stack">
            <CredentialConnectionTestCard targets={credentialTestTargets} target={testTarget} result={testResult} testing={testing} onTargetChange={setTestTarget} onRetest={runLocalConnectionTest} />
            <CredentialSecurityCard />
          </div>
        </div>
        <div className="credential-bottom-grid">
          <CredentialOverviewCard value={credentialOverview} />
          <CredentialHealthCard items={credentialHealthItems} />
          <CredentialOperationLogCard items={credentialOperationLogs} />
        </div>
      </div>
      <CredentialRiskNotice />
    </>
  );
}

function CredentialRiskNotice() {
  return (
    <div className="settings-basic-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleOutlined />
        <span>敏感凭据仅保存在本地安全存储；数据库仅保存引用与脱敏状态。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
