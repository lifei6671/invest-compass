import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp, Spin } from "antd";
import { useEffect, useMemo, useState } from "react";
import { dataSourceCredentialsClear, dataSourceCredentialsList, dataSourceCredentialsSave, dataSourceCredentialsTest } from "../../../../services/coreClient";
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
  const [providers, setProviders] = useState<DataSourceProvider[]>([]);
  const [configs, setConfigs] = useState<Record<string, CredentialConfig>>({});
  const [selectedProviderId, setSelectedProviderId] = useState("");
  const [testTargets, setTestTargets] = useState<Array<{ label: string; value: string }>>([]);
  const [testTarget, setTestTarget] = useState("");
  const [testResult, setTestResult] = useState<CredentialTestResult>({ status: "untested", messages: ["尚未执行本地预检"] });
  const [overview, setOverview] = useState({ configuredCount: 0, expiringSoonCount: 0, expiredCount: 0 });
  const [healthItems, setHealthItems] = useState<Array<{ name: string; status: "normal" | "limited" | "failed"; rateLimitText: string }>>([]);
  const [operationLogs, setOperationLogs] = useState<Array<{ id: string; action: string; status: "success" | "failed"; time: string }>>([]);
  const [credentialInput, setCredentialInput] = useState("");
  const [loading, setLoading] = useState(true);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    let ignore = false;
    async function loadCredentials() {
      setLoading(true);
      try {
        const data = await dataSourceCredentialsList();
        if (ignore) {
          return;
        }
        setProviders(data.providers);
        setConfigs(data.configs);
        setSelectedProviderId(data.selectedProviderId);
        setTestTargets(data.testTargets);
        setTestTarget(data.testTargets[0]?.value ?? "");
        setTestResult(data.testResult);
        setOverview(data.overview);
        setHealthItems(data.healthItems);
        setOperationLogs(data.operationLogs);
      } catch (error) {
        if (!ignore) {
          message.error(error instanceof Error ? error.message : "数据源凭据读取失败");
        }
      } finally {
        if (!ignore) {
          setLoading(false);
        }
      }
    }
    void loadCredentials();
    return () => {
      ignore = true;
    };
  }, [message]);

  const fallbackProviderId = providers[0]?.id ?? "";
  const selectedConfig = configs[selectedProviderId] ?? (fallbackProviderId ? configs[fallbackProviderId] : undefined);
  const selectedProvider = useMemo(() => providers.find((item) => item.id === selectedProviderId) ?? providers[0], [providers, selectedProviderId]);

  const updateSelectedConfig = (value: CredentialConfig) => {
    setConfigs((current) => ({ ...current, [value.providerId]: value }));
  };

  const runLocalConnectionTest = async () => {
    if (!selectedProviderId || !testTarget) {
      message.error("请选择 Provider 和测试目标");
      return;
    }
    setTesting(true);
    try {
      const data = await dataSourceCredentialsTest({ providerId: selectedProviderId, target: testTarget });
      setTestResult(data.result);
      message.success("连接测试完成");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "连接测试失败");
    } finally {
      setTesting(false);
    }
  };

  const selectProvider = (provider: DataSourceProvider) => {
    setSelectedProviderId(provider.id);
    setCredentialInput("");
    setTestResult(provider.status === "not_configured" ? { status: "untested", messages: ["当前 Provider 尚未配置凭据"] } : { status: "untested", messages: ["尚未执行本地预检"] });
    message.info("已切换 Provider");
  };

  const changeAuthType = (authType: CredentialAuthType) => {
    if (!selectedConfig) {
      return;
    }
    updateSelectedConfig({ ...selectedConfig, authType });
    message.info("认证方式已更新");
  };

  const saveCredential = async () => {
    if (!selectedConfig) {
      return;
    }
    if (!selectedConfig.providerName.trim() || !selectedConfig.baseUrl.trim() || !selectedConfig.authType) {
      message.error("请填写 Provider 名称、Base URL 和认证方式");
      return;
    }
    try {
      const data = await dataSourceCredentialsSave({ config: selectedConfig, credential: credentialInput });
      updateSelectedConfig(data.config);
      setProviders((current) => current.map((item) => (item.id === data.config.providerId ? { ...item, status: data.config.credentialStatus, authType: data.config.authType } : item)));
      setCredentialInput("");
      message.success("凭据配置已保存");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "凭据配置保存失败");
    }
  };

  const clearCredential = () => {
    if (!selectedConfig || !selectedProvider) {
      return;
    }
    modal.confirm({
      title: `确认清除 ${selectedProvider.name} 的凭据？`,
      content: "清除后会删除本地已保存的加密凭据，数据库仅保留未配置状态。",
      okText: "确认清除",
      cancelText: "取消",
      onOk: async () => {
        try {
          const data = await dataSourceCredentialsClear({ providerId: selectedProviderId });
          setProviders((current) => current.map((item) => (item.id === selectedProviderId ? { ...item, status: data.config.credentialStatus } : item)));
          updateSelectedConfig(data.config);
          setCredentialInput("");
          setTestResult({ status: "untested", messages: ["凭据已清除，尚未重新测试"] });
          message.success("凭据已清除");
        } catch (error) {
          message.error(error instanceof Error ? error.message : "凭据清除失败");
        }
      },
    });
  };

  if (loading || !selectedConfig) {
    return (
      <>
        <div className="credential-workspace credential-loading-state">
          <Spin />
        </div>
        <CredentialRiskNotice />
      </>
    );
  }

  return (
    <>
      <div className="credential-workspace">
        <div className="credential-main-grid">
          <CredentialProviderList items={providers} selectedProviderId={selectedProviderId} onSelect={selectProvider} />
          <CredentialConfigCard
            value={selectedConfig}
            credentialInput={credentialInput}
            testing={testing}
            onChange={updateSelectedConfig}
            onCredentialInputChange={setCredentialInput}
            onAuthTypeChange={changeAuthType}
            onSave={saveCredential}
            onTest={runLocalConnectionTest}
            onClear={clearCredential}
          />
          <div className="credential-side-stack">
            <CredentialConnectionTestCard targets={testTargets} target={testTarget} result={testResult} testing={testing} onTargetChange={setTestTarget} onRetest={runLocalConnectionTest} />
            <CredentialSecurityCard />
          </div>
        </div>
        <div className="credential-bottom-grid">
          <CredentialOverviewCard value={overview} />
          <CredentialHealthCard items={healthItems} />
          <CredentialOperationLogCard items={operationLogs} />
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
