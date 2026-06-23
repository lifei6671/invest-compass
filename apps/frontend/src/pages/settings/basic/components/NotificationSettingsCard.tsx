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
          title="应用内通知"
          description="在应用右上角通知区域展示应用内消息"
          checked={props.value.inAppEnabled}
          onChange={(checked) => update("inAppEnabled", checked)}
        />
        <SwitchRow
          title="系统级通知"
          description="允许通过系统通知中心提示重要事件"
          checked={props.value.systemEnabled}
          onChange={(checked) => update("systemEnabled", checked)}
        />
        <SwitchRow
          title="任务成功通知"
          description="任务执行成功时生成通知"
          checked={props.value.taskSuccessNotification}
          onChange={(checked) => update("taskSuccessNotification", checked)}
        />
        <SwitchRow
          title="任务失败通知"
          description="任务执行失败时生成通知"
          checked={props.value.taskFailedNotification}
          onChange={(checked) => update("taskFailedNotification", checked)}
        />
        <SwitchRow
          title="Provider 异常通知"
          description="数据源异常时生成通知"
          checked={props.value.providerErrorNotification}
          onChange={(checked) => update("providerErrorNotification", checked)}
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
