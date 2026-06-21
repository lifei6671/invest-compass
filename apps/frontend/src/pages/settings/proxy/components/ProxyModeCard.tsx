import { ProxyModeSelector } from "./ProxyModeSelector";
import type { ProxyMode } from "../types";

type ProxyModeCardProps = {
  value: ProxyMode;
  onChange: (mode: ProxyMode) => void;
};

export function ProxyModeCard(props: ProxyModeCardProps) {
  return (
    <section className="settings-basic-card settings-proxy-mode-card">
      <header className="settings-basic-card-header settings-proxy-card-header">
        <h2>代理模式</h2>
        <p>选择应用访问外部网络时使用的代理模式</p>
      </header>
      <ProxyModeSelector value={props.value} onChange={props.onChange} />
    </section>
  );
}
