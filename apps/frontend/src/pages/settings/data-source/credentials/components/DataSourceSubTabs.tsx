import { Tabs } from "antd";

export type DataSourceSubTabKey = "overview" | "credentials" | "description";

type DataSourceSubTabsProps = {
  activeKey: DataSourceSubTabKey;
  onChange: (key: DataSourceSubTabKey) => void;
};

const tabs: Array<{ key: DataSourceSubTabKey; label: string }> = [
  { key: "overview", label: "数据源概览" },
  { key: "credentials", label: "凭据管理" },
  { key: "description", label: "数据说明" },
];

export function DataSourceSubTabs(props: DataSourceSubTabsProps) {
  return <Tabs activeKey={props.activeKey} className="settings-data-source-sub-tabs" items={tabs.map((item) => ({ key: item.key, label: item.label, children: null }))} onChange={(key) => props.onChange(key as DataSourceSubTabKey)} />;
}
