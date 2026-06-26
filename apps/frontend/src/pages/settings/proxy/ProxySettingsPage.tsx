import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { useCallback, useEffect, useState } from "react";
import { proxyConnectionTest, settingsGet, settingsSet, type SettingItem, type SettingsSetPayload } from "../../../services/coreClient";
import { ProxyBypassRulesCard } from "./components/ProxyBypassRulesCard";
import { ProxyConfigCard } from "./components/ProxyConfigCard";
import { ProxyConnectionTestCard } from "./components/ProxyConnectionTestCard";
import { ProxyModeCard } from "./components/ProxyModeCard";
import {
  initialHttpProxyConfig,
  initialProxyState,
  initialSocks5ProxyConfig,
  type CustomProxyProtocol,
  type HttpProxyConfig,
  type ProxyMode,
  type ProxySettingsState,
  type ProxyTestTarget,
  type Socks5ProxyConfig,
} from "./types";

const proxySettingKeys = ["proxy.mode", "proxy.http_url", "proxy.socks5_url", "proxy.no_proxy", "proxy.username", "proxy_credential_ref"];

export function ProxySettingsPage() {
  const { message } = AntApp.useApp();
  const [state, setState] = useState<ProxySettingsState>(initialProxyState);

  const loadProxySettings = useCallback(async () => {
    try {
      const result = await settingsGet(proxySettingKeys);
      setState((current) => proxyStateFromItems(current, result.items));
    } catch (error) {
      message.error(error instanceof Error ? error.message : "代理设置读取失败");
    }
  }, [message]);

  useEffect(() => {
    void loadProxySettings();
  }, [loadProxySettings]);

  const saveProxyPayload = async (payload: SettingsSetPayload) => {
    try {
      await settingsSet(payload);
      setState((current) => ({
        ...current,
        httpProxyConfig: {
          ...current.httpProxyConfig,
          password: "",
          hasSavedPassword: Boolean(payload.proxy_password) || current.httpProxyConfig.hasSavedPassword,
        },
        socks5ProxyConfig: {
          ...current.socks5ProxyConfig,
          password: "",
          hasSavedPassword: Boolean(payload.proxy_password) || current.socks5ProxyConfig.hasSavedPassword,
        },
      }));
      message.success("代理配置已保存");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "代理配置保存失败");
    }
  };

  const changeProxyMode = (proxyMode: ProxyMode) => {
    setState((current) => ({ ...current, proxyMode, systemProxyStatus: systemProxyStatusFromState({ ...current, proxyMode }) }));
    if (proxyMode === "none") {
      void saveProxyPayload(buildNoProxyPayload());
      return;
    }
    if (proxyMode === "custom") {
      void saveProxyPayload({
        items: [
          { key: "proxy.mode", value: "custom" },
          { key: "proxy.username", value: "" },
        ],
        clear_proxy_credential: true,
      });
      return;
    }
    void saveProxyPayload({ items: [{ key: "proxy.mode", value: proxyMode }] });
  };

  const testConnection = async () => {
    setState((current) => ({ ...current, testing: true }));
    try {
      const data = await proxyConnectionTest({ target: state.testTarget });
      setState((current) => ({
        ...current,
        testResult: {
          status: data.result.ok ? "success" : "failed",
          responseTimeMs: data.result.duration_ms,
          checkedAt: data.result.checked_at,
        },
      }));
      if (data.result.ok) {
        message.success("代理连接测试完成");
      } else {
        message.error("代理连接测试失败");
      }
    } catch (error) {
      setState((current) => ({ ...current, testResult: { status: "failed" } }));
      message.error(error instanceof Error ? error.message : "代理连接测试失败");
    } finally {
      setState((current) => ({ ...current, testing: false }));
    }
  };

  return (
    <>
      <ProxyModeCard value={state.proxyMode} onChange={changeProxyMode} />
      <div className="settings-proxy-content-grid">
        <ProxyConfigCard
          mode={state.proxyMode}
          customProtocol={state.customProtocol}
          value={state.systemProxyStatus}
          httpConfig={state.httpProxyConfig}
          socks5Config={state.socks5ProxyConfig}
          onCustomProtocolChange={(customProtocol) => setState((current) => ({ ...current, customProtocol }))}
          onHttpConfigChange={(httpProxyConfig) => setState((current) => ({ ...current, httpProxyConfig }))}
          onSocks5ConfigChange={(socks5ProxyConfig) => setState((current) => ({ ...current, socks5ProxyConfig }))}
          onRefresh={() => void loadProxySettings()}
          onSaveHttpConfig={() => void saveProxyPayload(buildHttpProxyPayload(state))}
          onClearHttpConfig={() => {
            setState((current) => {
              const nextState = { ...current, httpProxyConfig: initialHttpProxyConfig, proxyMode: "none" as const, customProtocol: "http" as const };
              return { ...nextState, systemProxyStatus: systemProxyStatusFromState(nextState) };
            });
            void saveProxyPayload(buildNoProxyPayload());
          }}
          onSaveSocks5Config={() => void saveProxyPayload(buildSocks5ProxyPayload(state))}
          onClearSocks5Config={() => {
            setState((current) => {
              const nextState = { ...current, socks5ProxyConfig: initialSocks5ProxyConfig, proxyMode: "none" as const, customProtocol: "socks5" as const };
              return { ...nextState, systemProxyStatus: systemProxyStatusFromState(nextState) };
            });
            void saveProxyPayload(buildNoProxyPayload());
          }}
        />
        <div className="settings-proxy-side-column">
          <ProxyConnectionTestCard
            target={state.testTarget}
            result={state.testResult}
            testing={state.testing}
            onTargetChange={(testTarget: ProxyTestTarget) => setState((current) => ({ ...current, testTarget }))}
            onTest={() => void testConnection()}
          />
          <ProxyBypassRulesCard
            value={state.bypassRules}
            onChange={(bypassRules) => setState((current) => ({ ...current, bypassRules }))}
            onSave={() => void saveProxyPayload({ items: [{ key: "proxy.no_proxy", value: state.bypassRules }] })}
          />
        </div>
      </div>
      <ProxyRiskNotice />
    </>
  );
}

function proxyStateFromItems(current: ProxySettingsState, items: SettingItem[]): ProxySettingsState {
  const values = new Map(items.map((item) => [item.key, item.value]));
  const rawProxyMode = values.get("proxy.mode");
  const proxyMode = readProxyMode(rawProxyMode) ?? current.proxyMode;
  const customProtocol = customProtocolFromItems(rawProxyMode, values.get("proxy.http_url"), values.get("proxy.socks5_url"), current.customProtocol);
  const httpProxyConfig = proxyConfigFromURL(current.httpProxyConfig, values.get("proxy.http_url"), "http");
  const socks5ProxyConfig = proxyConfigFromURL(current.socks5ProxyConfig, values.get("proxy.socks5_url"), "socks5");
  const username = values.get("proxy.username");
  const hasSavedPassword = Boolean(values.get("proxy_credential_ref"));
  if (username !== undefined || hasSavedPassword) {
    httpProxyConfig.username = "";
    socks5ProxyConfig.username = "";
  }
  httpProxyConfig.authenticationEnabled = false;
  socks5ProxyConfig.authenticationEnabled = false;
  httpProxyConfig.hasSavedPassword = false;
  socks5ProxyConfig.hasSavedPassword = false;

  const nextState = {
    ...current,
    proxyMode,
    customProtocol,
    bypassRules: values.get("proxy.no_proxy") ?? current.bypassRules,
    httpProxyConfig,
    socks5ProxyConfig,
  };
  return {
    ...nextState,
    systemProxyStatus: systemProxyStatusFromState(nextState),
  };
}

function readProxyMode(value: string | undefined): ProxyMode | undefined {
  if (value === "http" || value === "socks5") {
    return "custom";
  }
  return value === "system" || value === "none" || value === "custom" ? value : undefined;
}

function customProtocolFromItems(
  rawMode: string | undefined,
  httpURL: string | undefined,
  socks5URL: string | undefined,
  fallback: CustomProxyProtocol,
): CustomProxyProtocol {
  if (rawMode === "http") {
    return "http";
  }
  if (rawMode === "socks5") {
    return "socks5";
  }
  if (socks5URL && !httpURL) {
    return "socks5";
  }
  if (httpURL) {
    return "http";
  }
  return fallback;
}

function proxyConfigFromURL<Config extends HttpProxyConfig | Socks5ProxyConfig>(fallback: Config, value: string | undefined, defaultScheme: string): Config {
  if (!value) {
    return { ...fallback, password: "" };
  }
  try {
    const url = new URL(value);
    return {
      ...fallback,
      host: url.hostname || fallback.host,
      port: Number(url.port || fallback.port),
      password: "",
    };
  } catch {
    const [host, port] = value.replace(`${defaultScheme}://`, "").split(":");
    return {
      ...fallback,
      host: host || fallback.host,
      port: Number(port || fallback.port),
      password: "",
    };
  }
}

function buildHttpProxyPayload(state: ProxySettingsState): SettingsSetPayload {
  return {
    items: [
      { key: "proxy.mode", value: "custom" },
      { key: "proxy.http_url", value: `http://${state.httpProxyConfig.host}:${state.httpProxyConfig.port}` },
      { key: "proxy.socks5_url", value: "" },
      { key: "proxy.no_proxy", value: state.bypassRules },
      { key: "proxy.username", value: "" },
    ],
    clear_proxy_credential: true,
  };
}

function buildSocks5ProxyPayload(state: ProxySettingsState): SettingsSetPayload {
  return {
    items: [
      { key: "proxy.mode", value: "custom" },
      { key: "proxy.http_url", value: "" },
      { key: "proxy.socks5_url", value: `${state.socks5ProxyConfig.version}://${state.socks5ProxyConfig.host}:${state.socks5ProxyConfig.port}` },
      { key: "proxy.no_proxy", value: state.bypassRules },
      { key: "proxy.username", value: "" },
    ],
    clear_proxy_credential: true,
  };
}

function buildNoProxyPayload(): SettingsSetPayload {
  return {
    items: [
      { key: "proxy.mode", value: "none" },
      { key: "proxy.http_url", value: "" },
      { key: "proxy.socks5_url", value: "" },
      { key: "proxy.username", value: "" },
    ],
    clear_proxy_credential: true,
  };
}

function systemProxyStatusFromState(state: ProxySettingsState) {
  if (state.proxyMode === "none") {
    return {
      source: "应用设置",
      enabled: false,
      pacMode: "不使用",
      proxyAddress: "直连",
      bypassAddress: "全部外部请求直连",
      lastCheckedAt: "已从 settings 读取",
    };
  }
  if (state.proxyMode === "custom") {
    const address = state.customProtocol === "http"
      ? `${state.httpProxyConfig.host}:${state.httpProxyConfig.port}`
      : `${state.socks5ProxyConfig.host}:${state.socks5ProxyConfig.port}`;
    return {
      source: "应用手动配置",
      enabled: true,
      pacMode: state.customProtocol === "http" ? "HTTP" : "SOCKS",
      proxyAddress: address,
      bypassAddress: state.bypassRules || "未配置",
      lastCheckedAt: "已从 settings 读取",
    };
  }
  return {
    source: "操作系统",
    enabled: true,
    pacMode: "由系统决定",
    proxyAddress: "根据系统设置",
    bypassAddress: state.bypassRules || "根据系统设置",
    lastCheckedAt: "已从 settings 读取",
  };
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
