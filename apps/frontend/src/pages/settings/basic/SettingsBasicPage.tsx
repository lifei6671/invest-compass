import { App as AntApp } from "antd";
import { useState } from "react";
import { AppBasicSettingsCard } from "./components/AppBasicSettingsCard";
import { CacheManagementCard } from "./components/CacheManagementCard";
import { DesktopCapabilityCard } from "./components/DesktopCapabilityCard";
import { NotificationSettingsCard } from "./components/NotificationSettingsCard";
import { OtherSettingsCard } from "./components/OtherSettingsCard";
import { ProxySummaryCard } from "./components/ProxySummaryCard";
import { WorkspaceSettingsCard } from "./components/WorkspaceSettingsCard";
import { SettingsRiskNotice } from "../components/SettingsRiskNotice";
import {
  initialBasicSettings,
  initialCacheSummary,
  initialDesktopSettings,
  initialNotificationSettings,
  initialOtherSettings,
  initialProxySummary,
  initialWorkspace,
  type BasicSettingsState,
  type CacheSummary,
  type DesktopSettingsState,
  type NotificationSettingsState,
  type OtherSettingsState,
} from "./types";

export function SettingsBasicPage() {
  const { message } = AntApp.useApp();
  const [basicSettings, setBasicSettings] = useState<BasicSettingsState>(initialBasicSettings);
  const [notificationSettings, setNotificationSettings] = useState<NotificationSettingsState>(initialNotificationSettings);
  const [desktopSettings, setDesktopSettings] = useState<DesktopSettingsState>(initialDesktopSettings);
  const [cacheSummary, setCacheSummary] = useState<CacheSummary>(initialCacheSummary);
  const [otherSettings, setOtherSettings] = useState<OtherSettingsState>(initialOtherSettings);

  return (
    <>
      <AppBasicSettingsCard
        value={basicSettings}
        onChange={(value) => {
          setBasicSettings(value);
          message.success("设置已更新");
        }}
      />
      <div className="settings-basic-card-grid">
        <WorkspaceSettingsCard
          value={initialWorkspace}
          onSelectDirectory={() => message.info("选择目录待接入")}
          onOpenDirectory={() => message.info("打开目录待接入")}
        />
        <NotificationSettingsCard
          value={notificationSettings}
          onChange={(value) => {
            setNotificationSettings(value);
            message.success("通知设置已更新");
          }}
        />
        <DesktopCapabilityCard
          value={desktopSettings}
          onChange={(value) => {
            setDesktopSettings(value);
            message.success("桌面能力设置已更新");
          }}
        />
        <CacheManagementCard
          value={cacheSummary}
          onCleanCache={() => {
            setCacheSummary({ ...cacheSummary, totalSize: "0 MB", tempSize: "0 MB" });
            message.success("缓存清理完成");
          }}
        />
        <ProxySummaryCard value={initialProxySummary} onEditProxy={() => message.info("代理设置页待接入")} />
        <OtherSettingsCard
          value={otherSettings}
          onChange={(value) => {
            setOtherSettings(value);
            message.success("设置已更新");
          }}
        />
      </div>
      <SettingsRiskNotice />
    </>
  );
}
