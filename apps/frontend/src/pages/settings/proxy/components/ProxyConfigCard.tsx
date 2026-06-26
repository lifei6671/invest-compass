import { DeleteOutlined, InfoCircleOutlined, ReloadOutlined } from "@ant-design/icons";
import { Button, Input, InputNumber, Select } from "antd";
import type { ReactNode } from "react";
import {
  customProxyProtocolOptions,
  httpProxyProtocolOptions,
  initialHttpProxyConfig,
  initialSocks5ProxyConfig,
  socksProxyVersionOptions,
  type CustomProxyProtocol,
  type HttpProxyConfig,
  type ProxyMode,
  type Socks5ProxyConfig,
  type SystemProxyStatus,
} from "../types";

type ProxyConfigCardProps = {
  mode: ProxyMode;
  customProtocol: CustomProxyProtocol;
  value: SystemProxyStatus;
  httpConfig: HttpProxyConfig;
  socks5Config: Socks5ProxyConfig;
  onCustomProtocolChange: (value: CustomProxyProtocol) => void;
  onHttpConfigChange: (value: HttpProxyConfig) => void;
  onSocks5ConfigChange: (value: Socks5ProxyConfig) => void;
  onRefresh: () => void;
  onSaveHttpConfig: () => void;
  onClearHttpConfig: () => void;
  onSaveSocks5Config: () => void;
  onClearSocks5Config: () => void;
};

export function ProxyConfigCard(props: ProxyConfigCardProps) {
  if (props.mode === "custom") {
    const useHTTP = props.customProtocol === "http";
    return (
      <section className="settings-basic-card settings-proxy-config-card settings-proxy-manual-config-card">
        <header className="settings-basic-card-header settings-proxy-card-header">
          <h2>代理配置</h2>
          <p>以下设置仅对当前应用生效，不会修改系统代理设置</p>
        </header>
        <div className="settings-proxy-manual-form">
          <ManualField label="代理协议">
            <Select
              className="settings-basic-select settings-proxy-manual-select"
              value={props.customProtocol}
              options={customProxyProtocolOptions}
              onChange={props.onCustomProtocolChange}
            />
          </ManualField>
          <ManualField label="代理地址">
            <Input
              className="settings-proxy-manual-input"
              value={useHTTP ? props.httpConfig.host : props.socks5Config.host}
              onChange={(event) =>
                useHTTP
                  ? props.onHttpConfigChange({ ...props.httpConfig, host: event.target.value })
                  : props.onSocks5ConfigChange({ ...props.socks5Config, host: event.target.value })
              }
            />
          </ManualField>
          <ManualField label="端口">
            <InputNumber
              className="settings-proxy-manual-number"
              controls
              min={1}
              max={65535}
              value={useHTTP ? props.httpConfig.port : props.socks5Config.port}
              onChange={(value) =>
                useHTTP
                  ? props.onHttpConfigChange({ ...props.httpConfig, port: Number(value ?? initialHttpProxyConfig.port) })
                  : props.onSocks5ConfigChange({ ...props.socks5Config, port: Number(value ?? initialSocks5ProxyConfig.port) })
              }
            />
          </ManualField>
          {useHTTP ? (
            <ManualField label="协议类型">
              <Select
                className="settings-basic-select settings-proxy-manual-select"
                value={props.httpConfig.protocol}
                options={httpProxyProtocolOptions}
                onChange={(protocol) => props.onHttpConfigChange({ ...props.httpConfig, protocol })}
              />
            </ManualField>
          ) : (
            <ManualField label="SOCKS 版本">
              <Select
                className="settings-basic-select settings-proxy-manual-select"
                value={props.socks5Config.version}
                options={socksProxyVersionOptions}
                onChange={(version) => props.onSocks5ConfigChange({ ...props.socks5Config, version })}
              />
            </ManualField>
          )}
          <div className="settings-proxy-info-box">
            <InfoCircleOutlined />
            <span>首版手动代理仅支持无认证代理，已保存的代理凭据不会用于运行时请求。</span>
          </div>
          <ManualField label="连接超时">
            <div className="settings-proxy-timeout-control">
              <InputNumber
                className="settings-proxy-manual-number settings-proxy-timeout-number"
                controls
                min={1}
                max={120}
                value={useHTTP ? props.httpConfig.timeoutSeconds : props.socks5Config.timeoutSeconds}
                onChange={(value) =>
                  useHTTP
                    ? props.onHttpConfigChange({ ...props.httpConfig, timeoutSeconds: Number(value ?? initialHttpProxyConfig.timeoutSeconds) })
                    : props.onSocks5ConfigChange({ ...props.socks5Config, timeoutSeconds: Number(value ?? initialSocks5ProxyConfig.timeoutSeconds) })
                }
              />
              <span>秒</span>
            </div>
          </ManualField>
        </div>
        <div className="settings-proxy-manual-button-row">
          <Button type="primary" className="settings-proxy-manual-save-button" icon={<ReloadOutlined />} onClick={useHTTP ? props.onSaveHttpConfig : props.onSaveSocks5Config}>
            保存代理配置
          </Button>
          <Button className="settings-proxy-manual-clear-button" icon={<DeleteOutlined />} onClick={useHTTP ? props.onClearHttpConfig : props.onClearSocks5Config}>
            清空配置
          </Button>
        </div>
      </section>
    );
  }

  if (props.mode === "none") {
    return (
      <section className="settings-basic-card settings-proxy-config-card">
        <header className="settings-basic-card-header settings-proxy-card-header">
          <h2>代理配置</h2>
          <p>当前应用会直连外部数据源</p>
        </header>
        <div className="settings-proxy-info-box">
          <InfoCircleOutlined />
          <span>当前应用的外部数据请求不会使用系统代理或手动代理。</span>
        </div>
      </section>
    );
  }

  return (
    <section className="settings-basic-card settings-proxy-config-card">
      <header className="settings-basic-card-header settings-proxy-card-header">
        <h2>代理配置</h2>
        <p>当前使用系统代理设置，无需手动配置</p>
      </header>
      <div className="settings-proxy-info-box">
        <InfoCircleOutlined />
        <span>系统代理信息由操作系统管理（如 Windows 设置 &gt; 网络和 Internet &gt; 代理，或 macOS 系统设置 &gt; 网络 &gt; 代理）。</span>
      </div>
      <div className="settings-proxy-status-list">
        <StatusRow label="代理来源" value={props.value.source} />
          <StatusRow
            label="代理状态"
            value={
              <span className="settings-proxy-enabled-value">
                <span className={["settings-data-source-dot", props.value.enabled ? "settings-data-source-dot-ok" : "settings-data-source-dot-muted"].join(" ")} />
                {props.value.enabled ? "已启用" : "未启用"}
              </span>
            }
          />
        <StatusRow label="PAC 模式" value={props.value.pacMode} />
        <StatusRow label="代理地址" value={props.value.proxyAddress} />
        <StatusRow label="排除地址" value={props.value.bypassAddress} />
        <StatusRow label="最后检查时间" value={props.value.lastCheckedAt} />
      </div>
      <Button className="settings-basic-outline-button settings-proxy-refresh-button" icon={<ReloadOutlined />} onClick={props.onRefresh}>
        刷新代理状态
      </Button>
    </section>
  );
}

function ManualField(props: { label: string; children: ReactNode }) {
  return (
    <div className="settings-proxy-manual-field">
      <label>{props.label}</label>
      <div className="settings-proxy-manual-control">{props.children}</div>
    </div>
  );
}

function StatusRow(props: { label: string; value: ReactNode }) {
  return (
    <div className="settings-proxy-status-row">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
