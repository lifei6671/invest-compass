import { Tabs } from "antd";
import { settingsTabs } from "../defaults";
import type { SettingsTabKey } from "../types";

type SettingsTabsProps = {
  activeKey: SettingsTabKey;
  onChange: (key: SettingsTabKey) => void;
};

export function SettingsTabs(props: SettingsTabsProps) {
  return (
    <Tabs
      activeKey={props.activeKey}
      className="settings-tabs"
      items={settingsTabs.map((item) => ({ key: item.key, label: item.label, children: null }))}
      onChange={(key) => props.onChange(key as SettingsTabKey)}
    />
  );
}
