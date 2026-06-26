import { Button } from "antd";
import { GlobalOutlined, SettingOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { ProxySummary } from "../types";

type ProxySummaryCardProps = {
  value: ProxySummary;
  onEditProxy: () => void;
};

const proxyModeText: Record<ProxySummary["mode"], string> = {
  system: "系统代理",
  none: "不使用代理",
  custom: "手动代理",
};

export function ProxySummaryCard(props: ProxySummaryCardProps) {
  return (
    <section className="settings-basic-card settings-basic-mini-card">
      <CardHeader icon={<GlobalOutlined />} title="代理设置摘要" description="应用的网络代理配置" />
      <div className="settings-basic-kv-list settings-basic-proxy-list">
        <KeyValue label="当前代理模式" value={proxyModeText[props.value.mode]} />
        <KeyValue label="代理地址" value={props.value.address ?? "—"} />
      </div>
      <Button className="settings-basic-outline-button" icon={<SettingOutlined />} onClick={props.onEditProxy}>
        编辑代理设置
      </Button>
    </section>
  );
}

function CardHeader(props: { icon: ReactNode; title: string; description: string }) {
  return (
    <header className="settings-basic-mini-header">
      <span className="settings-basic-mini-icon">{props.icon}</span>
      <div>
        <h3>{props.title}</h3>
        <p>{props.description}</p>
      </div>
    </header>
  );
}

function KeyValue(props: { label: string; value: string }) {
  return (
    <div className="settings-basic-kv-row">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
