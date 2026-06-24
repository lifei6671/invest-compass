import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { App as AntApp } from "antd";
import { useCallback, useEffect, useState } from "react";
import { settingsGet, settingsSet, type SettingItem, type SettingsSetPayload } from "../../../services/coreClient";
import { ProxyBypassRulesCard } from "./components/ProxyBypassRulesCard";
import { ProxyConfigCard } from "./components/ProxyConfigCard";
import { ProxyConnectionTestCard } from "./components/ProxyConnectionTestCard";
import { ProxyModeCard } from "./components/ProxyModeCard";
import {
  initialHttpProxyConfig,
  initialProxyState,
  initialSocks5ProxyConfig,
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
    setState((current) => ({ ...current, proxyMode }));
    void saveProxyPayload({ items: [{ key: "proxy.mode", value: proxyMode }] });
  };

  const testConnection = () => {
    message.info("代理连接测试待接入");
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
          onRefresh={() => message.info("代理状态刷新待接入")}
          onSaveHttpConfig={() => void saveProxyPayload(buildHttpProxyPayload(state))}
          onClearHttpConfig={() => {
            setState((current) => ({ ...current, httpProxyConfig: initialHttpProxyConfig, proxyMode: "system" }));
            void saveProxyPayload({
              items: [
                { key: "proxy.mode", value: "system" },
                { key: "proxy.http_url", value: "" },
                { key: "proxy.username", value: "" },
              ],
              clear_proxy_credential: true,
            });
          }}
          onSaveSocks5Config={() => void saveProxyPayload(buildSocks5ProxyPayload(state))}
          onClearSocks5Config={() => {
            setState((current) => ({ ...current, socks5ProxyConfig: initialSocks5ProxyConfig, proxyMode: "system" }));
            void saveProxyPayload({
              items: [
                { key: "proxy.mode", value: "system" },
                { key: "proxy.socks5_url", value: "" },
                { key: "proxy.username", value: "" },
              ],
              clear_proxy_credential: true,
            });
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
  const proxyMode = readProxyMode(values.get("proxy.mode")) ?? current.proxyMode;
  const httpProxyConfig = proxyConfigFromURL(current.httpProxyConfig, values.get("proxy.http_url"), "http");
  const socks5ProxyConfig = proxyConfigFromURL(current.socks5ProxyConfig, values.get("proxy.socks5_url"), "socks5");
  const username = values.get("proxy.username");
  const hasSavedPassword = Boolean(values.get("proxy_credential_ref"));
  if (username !== undefined) {
    httpProxyConfig.username = username;
    socks5ProxyConfig.username = username;
  }
  httpProxyConfig.authenticationEnabled = hasSavedPassword || Boolean(httpProxyConfig.username);
  socks5ProxyConfig.authenticationEnabled = hasSavedPassword || Boolean(socks5ProxyConfig.username);
  httpProxyConfig.hasSavedPassword = hasSavedPassword;
  socks5ProxyConfig.hasSavedPassword = hasSavedPassword;

  return {
    ...current,
    proxyMode,
    bypassRules: values.get("proxy.no_proxy") ?? current.bypassRules,
    httpProxyConfig,
    socks5ProxyConfig,
  };
}

function readProxyMode(value: string | undefined): ProxyMode | undefined {
  return value === "system" || value === "http" || value === "socks5" ? value : undefined;
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
  return withOptionalProxyPassword(
    {
      items: [
        { key: "proxy.mode", value: "http" },
        { key: "proxy.http_url", value: `http://${state.httpProxyConfig.host}:${state.httpProxyConfig.port}` },
        { key: "proxy.no_proxy", value: state.bypassRules },
        { key: "proxy.username", value: state.httpProxyConfig.username },
      ],
    },
    state.httpProxyConfig.password,
  );
}

function buildSocks5ProxyPayload(state: ProxySettingsState): SettingsSetPayload {
  return withOptionalProxyPassword(
    {
      items: [
        { key: "proxy.mode", value: "socks5" },
        { key: "proxy.socks5_url", value: `${state.socks5ProxyConfig.version}://${state.socks5ProxyConfig.host}:${state.socks5ProxyConfig.port}` },
        { key: "proxy.no_proxy", value: state.bypassRules },
        { key: "proxy.username", value: state.socks5ProxyConfig.username },
      ],
    },
    state.socks5ProxyConfig.password,
  );
}

function withOptionalProxyPassword(payload: SettingsSetPayload, password: string): SettingsSetPayload {
  const trimmed = password.trim();
  return trimmed ? { ...payload, proxy_password: trimmed } : payload;
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
