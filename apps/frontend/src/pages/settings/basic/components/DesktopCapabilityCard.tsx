import { Switch } from "antd";
import { DesktopOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { DesktopSettingsState } from "../types";

type DesktopCapabilityCardProps = {
  value: DesktopSettingsState;
  onChange: (value: DesktopSettingsState) => void;
};

export function DesktopCapabilityCard(props: DesktopCapabilityCardProps) {
  const update = (key: keyof DesktopSettingsState, value: boolean) => {
    props.onChange({ ...props.value, [key]: value });
  };

  return (
    <section className="settings-basic-card settings-basic-mini-card">
      <CardHeader icon={<DesktopOutlined />} title="桌面能力" description="配置桌面端行为与系统集成能力" />
      <div className="settings-basic-switch-list">
        <SwitchRow
          title="开机自启动"
          description="系统启动时自动运行应用"
          checked={props.value.autostart}
          onChange={(checked) => update("autostart", checked)}
        />
        <SwitchRow
          title="关闭后最小化到托盘"
          description="关闭窗口后最小化到系统托盘"
          checked={props.value.closeToTray}
          onChange={(checked) => update("closeToTray", checked)}
        />
      </div>
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

function SwitchRow(props: { title: string; description: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <div className="settings-basic-switch-row">
      <div>
        <div className="settings-basic-switch-title">{props.title}</div>
        <div className="settings-basic-switch-desc">{props.description}</div>
      </div>
      <Switch checked={props.checked} onChange={props.onChange} />
    </div>
  );
}
