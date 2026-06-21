import { Switch } from "antd";
import { SlidersOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { OtherSettingsState } from "../types";

type OtherSettingsCardProps = {
  value: OtherSettingsState;
  onChange: (value: OtherSettingsState) => void;
};

export function OtherSettingsCard(props: OtherSettingsCardProps) {
  const update = (key: keyof OtherSettingsState, value: boolean) => {
    props.onChange({ ...props.value, [key]: value });
  };

  return (
    <section className="settings-basic-card settings-basic-mini-card">
      <CardHeader icon={<SlidersOutlined />} title="其他设置" />
      <div className="settings-basic-switch-list settings-basic-other-list">
        <SwitchRow
          title="启动时检查更新"
          description="应用启动时自动检查新版本"
          checked={props.value.checkUpdateOnStartup}
          onChange={(checked) => update("checkUpdateOnStartup", checked)}
        />
        <SwitchRow
          title="加入匿名使用统计"
          description="帮助我们改进产品（不会收集个人信息）"
          checked={props.value.anonymousUsageStats}
          onChange={(checked) => update("anonymousUsageStats", checked)}
        />
      </div>
    </section>
  );
}

function CardHeader(props: { icon: ReactNode; title: string }) {
  return (
    <header className="settings-basic-mini-header settings-basic-mini-header-single">
      <span className="settings-basic-mini-icon">{props.icon}</span>
      <div>
        <h3>{props.title}</h3>
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
