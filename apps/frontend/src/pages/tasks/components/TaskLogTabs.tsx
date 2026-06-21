import { Tabs } from "antd";
import type { TaskLogTab } from "../taskLogTypes";

type TaskLogTabsProps = {
  activeTab: TaskLogTab;
  onChange: (tab: TaskLogTab) => void;
};

const tabItems: Array<{ key: TaskLogTab; label: string }> = [
  { key: "events", label: "事件流" },
  { key: "logs", label: "执行日志" },
  { key: "diagnosis", label: "错误诊断" },
  { key: "context", label: "上下文摘要" },
];

export function TaskLogTabs({ activeTab, onChange }: TaskLogTabsProps) {
  return (
    <Tabs
      className="task-log-tabs"
      activeKey={activeTab}
      items={tabItems}
      onChange={(key) => onChange(key as TaskLogTab)}
    />
  );
}
