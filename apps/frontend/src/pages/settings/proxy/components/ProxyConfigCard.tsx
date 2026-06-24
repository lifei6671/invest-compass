import { DeleteOutlined, InfoCircleOutlined, ReloadOutlined, SaveOutlined } from "@ant-design/icons";
import { Button, Input, InputNumber, Select, Switch } from "antd";
import type { ReactNode } from "react";
import {
  httpProxyProtocolOptions,
  initialHttpProxyConfig,
  initialSocks5ProxyConfig,
  socksProxyVersionOptions,
  type HttpProxyConfig,
  type ProxyMode,
  type Socks5ProxyConfig,
  type SystemProxyStatus,
} from "../types";

type ProxyConfigCardProps = {
  mode: ProxyMode;
  value: SystemProxyStatus;
  httpConfig: HttpProxyConfig;
  socks5Config: Socks5ProxyConfig;
  onHttpConfigChange: (value: HttpProxyConfig) => void;
  onSocks5ConfigChange: (value: Socks5ProxyConfig) => void;
  onRefresh: () => void;
  onSaveHttpConfig: () => void;
  onClearHttpConfig: () => void;
  onSaveSocks5Config: () => void;
  onClearSocks5Config: () => void;
};

export function ProxyConfigCard(props: ProxyConfigCardProps) {
  if (props.mode === "http") {
    return (
      <section className="settings-basic-card settings-proxy-config-card settings-proxy-manual-config-card">
        <header className="settings-basic-card-header settings-proxy-card-header">
          <h2>代理配置</h2>
          <p>以下设置仅对当前应用生效，不会修改系统代理设置</p>
        </header>
        <div className="settings-proxy-manual-form">
          <ManualField label="代理地址">
            <Input
              className="settings-proxy-manual-input"
              value={props.httpConfig.host}
              onChange={(event) => props.onHttpConfigChange({ ...props.httpConfig, host: event.target.value })}
            />
          </ManualField>
          <ManualField label="端口">
            <InputNumber
              className="settings-proxy-manual-number"
              controls
              min={1}
              max={65535}
              value={props.httpConfig.port}
              onChange={(value) => props.onHttpConfigChange({ ...props.httpConfig, port: Number(value ?? initialHttpProxyConfig.port) })}
            />
          </ManualField>
          <ManualField label="协议类型">
            <Select
              className="settings-basic-select settings-proxy-manual-select"
              value={props.httpConfig.protocol}
              options={httpProxyProtocolOptions}
              onChange={(protocol) => props.onHttpConfigChange({ ...props.httpConfig, protocol })}
            />
          </ManualField>
          <ManualField label="身份认证">
            <Switch
              className="settings-proxy-auth-switch"
              checked={props.httpConfig.authenticationEnabled}
              onChange={(authenticationEnabled) => props.onHttpConfigChange({ ...props.httpConfig, authenticationEnabled })}
            />
          </ManualField>
          <ManualField label="用户名">
            <Input
              className="settings-proxy-manual-input"
              value={props.httpConfig.username}
              onChange={(event) => props.onHttpConfigChange({ ...props.httpConfig, username: event.target.value })}
            />
          </ManualField>
          <ManualField label="密码">
            <>
              <Input.Password
                className="settings-proxy-manual-input"
                value={props.httpConfig.password}
                visibilityToggle
                onChange={(event) => props.onHttpConfigChange({ ...props.httpConfig, password: event.target.value })}
              />
              {props.httpConfig.hasSavedPassword ? <p className="settings-proxy-helper-text">已保存代理密码，输入新密码可替换</p> : null}
            </>
          </ManualField>
          <ManualField label="连接超时">
            <div className="settings-proxy-timeout-control">
              <InputNumber
                className="settings-proxy-manual-number settings-proxy-timeout-number"
                controls
                min={1}
                max={120}
                value={props.httpConfig.timeoutSeconds}
                onChange={(value) =>
                  props.onHttpConfigChange({ ...props.httpConfig, timeoutSeconds: Number(value ?? initialHttpProxyConfig.timeoutSeconds) })
                }
              />
              <span>秒</span>
            </div>
          </ManualField>
        </div>
        <div className="settings-proxy-manual-button-row">
          <Button type="primary" className="settings-proxy-manual-save-button" icon={<ReloadOutlined />} onClick={props.onSaveHttpConfig}>
            保存代理配置
          </Button>
          <Button className="settings-proxy-manual-clear-button" icon={<DeleteOutlined />} onClick={props.onClearHttpConfig}>
            清空配置
          </Button>
        </div>
      </section>
    );
  }

  if (props.mode === "socks5") {
    return (
      <section className="settings-basic-card settings-proxy-config-card settings-proxy-manual-config-card">
        <header className="settings-basic-card-header settings-proxy-card-header">
          <h2>代理配置</h2>
          <p>配置 SOCKS5 代理服务器连接参数</p>
        </header>
        <div className="settings-proxy-manual-form">
          <ManualField label="代理地址">
            <Input
              className="settings-proxy-manual-input"
              value={props.socks5Config.host}
              onChange={(event) => props.onSocks5ConfigChange({ ...props.socks5Config, host: event.target.value })}
            />
          </ManualField>
          <ManualField label="端口">
            <InputNumber
              className="settings-proxy-manual-number"
              controls
              min={1}
              max={65535}
              value={props.socks5Config.port}
              onChange={(value) => props.onSocks5ConfigChange({ ...props.socks5Config, port: Number(value ?? initialSocks5ProxyConfig.port) })}
            />
          </ManualField>
          <ManualField label="SOCKS 版本">
            <Select
              className="settings-basic-select settings-proxy-manual-select"
              value={props.socks5Config.version}
              options={socksProxyVersionOptions}
              onChange={(version) => props.onSocks5ConfigChange({ ...props.socks5Config, version })}
            />
          </ManualField>
          <ManualField label="身份认证">
            <Switch
              className="settings-proxy-auth-switch"
              checked={props.socks5Config.authenticationEnabled}
              onChange={(authenticationEnabled) => props.onSocks5ConfigChange({ ...props.socks5Config, authenticationEnabled })}
            />
          </ManualField>
          <ManualField label="用户名">
            <Input
              className="settings-proxy-manual-input"
              value={props.socks5Config.username}
              onChange={(event) => props.onSocks5ConfigChange({ ...props.socks5Config, username: event.target.value })}
            />
          </ManualField>
          <ManualField label="密码">
            <>
              <Input.Password
                className="settings-proxy-manual-input"
                value={props.socks5Config.password}
                visibilityToggle
                onChange={(event) => props.onSocks5ConfigChange({ ...props.socks5Config, password: event.target.value })}
              />
              {props.socks5Config.hasSavedPassword ? <p className="settings-proxy-helper-text">已保存代理密码，输入新密码可替换</p> : null}
            </>
          </ManualField>
          <ManualField label="连接超时">
            <div className="settings-proxy-timeout-control">
              <InputNumber
                className="settings-proxy-manual-number settings-proxy-timeout-number"
                controls
                min={1}
                max={120}
                value={props.socks5Config.timeoutSeconds}
                onChange={(value) =>
                  props.onSocks5ConfigChange({ ...props.socks5Config, timeoutSeconds: Number(value ?? initialSocks5ProxyConfig.timeoutSeconds) })
                }
              />
              <span>秒</span>
            </div>
          </ManualField>
        </div>
        <div className="settings-proxy-manual-button-row">
          <Button type="primary" className="settings-proxy-manual-save-button" icon={<SaveOutlined />} onClick={props.onSaveSocks5Config}>
            保存代理配置
          </Button>
          <Button className="settings-proxy-manual-clear-button" icon={<DeleteOutlined />} onClick={props.onClearSocks5Config}>
            清空配置
          </Button>
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
              <span className="settings-data-source-dot settings-data-source-dot-ok" />
              已启用
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
