import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { useState } from "react";
import { ProxyBypassRulesCard } from "./components/ProxyBypassRulesCard";
import { ProxyConfigCard } from "./components/ProxyConfigCard";
import { ProxyConnectionTestCard } from "./components/ProxyConnectionTestCard";
import { ProxyModeCard } from "./components/ProxyModeCard";
import { initialHttpProxyConfig, initialProxyState, initialSocks5ProxyConfig, type ProxyMode, type ProxySettingsState, type ProxyTestTarget } from "./types";

export function ProxySettingsPage() {
  const { message } = AntApp.useApp();
  const [state, setState] = useState<ProxySettingsState>(initialProxyState);

  const changeProxyMode = (proxyMode: ProxyMode) => {
    setState((current) => ({ ...current, proxyMode }));
    if (proxyMode === "system") {
      message.success("已切换为系统代理");
      return;
    }
    message.info(proxyMode === "http" ? "HTTP 代理配置待接入" : "SOCKS5 代理配置待接入");
  };

  const testConnection = () => {
    setState((current) => ({ ...current, testing: true }));
    window.setTimeout(() => {
      setState((current) => ({
        ...current,
        testing: false,
        testResult: {
          status: "success",
          responseTimeMs: 128,
          checkedAt: "2025-05-20 15:30:00",
        },
      }));
      message.success("连接测试完成");
    }, 600);
  };

  return (
    <>
      <ProxyModeCard value={state.proxyMode} onChange={changeProxyMode} />
      <div className="settings-proxy-content-grid">
        <ProxyConfigCard
          mode={state.proxyMode}
          value={state.systemProxyStatus}
          httpConfig={state.httpProxyConfig}
          socks5Config={state.socks5ProxyConfig}
          onHttpConfigChange={(httpProxyConfig) => setState((current) => ({ ...current, httpProxyConfig }))}
          onSocks5ConfigChange={(socks5ProxyConfig) => setState((current) => ({ ...current, socks5ProxyConfig }))}
          onRefresh={() => message.success("代理状态已刷新")}
          onSaveHttpConfig={() => message.success("代理配置已保存")}
          onClearHttpConfig={() => {
            setState((current) => ({ ...current, httpProxyConfig: initialHttpProxyConfig }));
            message.success("代理配置已清空");
          }}
          onSaveSocks5Config={() => message.success("代理配置已保存")}
          onClearSocks5Config={() => {
            setState((current) => ({ ...current, socks5ProxyConfig: initialSocks5ProxyConfig }));
            message.success("代理配置已清空");
          }}
        />
        <div className="settings-proxy-side-column">
          <ProxyConnectionTestCard
            target={state.testTarget}
            result={state.testResult}
            testing={state.testing}
            onTargetChange={(testTarget: ProxyTestTarget) => setState((current) => ({ ...current, testTarget }))}
            onTest={testConnection}
          />
          <ProxyBypassRulesCard
            value={state.bypassRules}
            onChange={(bypassRules) => setState((current) => ({ ...current, bypassRules }))}
            onSave={() => message.success("绕过代理规则已保存")}
          />
        </div>
      </div>
      <ProxyRiskNotice />
    </>
  );
}

function ProxyRiskNotice() {
  return (
    <div className="settings-basic-risk-notice">
      <div className="settings-basic-risk-left">
        <InfoCircleOutlined />
        <span>代理配置仅影响应用访问外部网络的行为，不会修改系统或其他应用的网络设置。</span>
      </div>
      <div className="settings-basic-risk-right">
        <SafetyCertificateOutlined />
        <span>仅供研究，不构成投资建议。</span>
      </div>
    </div>
  );
}
