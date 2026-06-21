import { Switch } from "antd";
import { BellOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { NotificationSettingsState } from "../types";

type NotificationSettingsCardProps = {
  value: NotificationSettingsState;
  onChange: (value: NotificationSettingsState) => void;
};

export function NotificationSettingsCard(props: NotificationSettingsCardProps) {
  const update = (key: keyof NotificationSettingsState, value: boolean) => {
    props.onChange({ ...props.value, [key]: value });
  };

  return (
    <section className="settings-basic-card settings-basic-mini-card">
      <CardHeader icon={<BellOutlined />} title="通知设置" description="配置任务与系统通知的接收方式" />
      <div className="settings-basic-switch-list">
        <SwitchRow
          title="任务成功通知"
          description="任务执行成功时显示系统通知"
          checked={props.value.taskSuccessNotification}
          onChange={(checked) => update("taskSuccessNotification", checked)}
        />
        <SwitchRow
          title="任务失败通知"
          description="任务执行失败时显示系统通知"
          checked={props.value.taskFailedNotification}
          onChange={(checked) => update("taskFailedNotification", checked)}
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
