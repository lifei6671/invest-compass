import { App as AntApp } from "antd";
import { useState } from "react";
import { AboutAppPage } from "./about/AboutAppPage";
import { SettingsBasicPage } from "./basic/SettingsBasicPage";
import { SettingsTabs } from "./basic/components/SettingsTabs";
import type { SettingsTabKey } from "./basic/types";
import { DataSourceSettingsPage } from "./data-source/DataSourceSettingsPage";
import { ModelConfigPage } from "./model-config/ModelConfigPage";
import { ProxySettingsPage } from "./proxy/ProxySettingsPage";
import { PromptTemplatePage } from "../prompt-template/PromptTemplatePage";

type SettingsPageProps = {
  initialActiveTab?: SettingsTabKey;
};

export function SettingsPage(props: SettingsPageProps = {}) {
  const { message } = AntApp.useApp();
  const [activeTab, setActiveTab] = useState<SettingsTabKey>(props.initialActiveTab ?? "basic");

  return (
    <section className="settings-basic-page">
      <SettingsTabs
        activeKey={activeTab}
        onChange={(key) => {
          if (key === "basic" || key === "model-config" || key === "prompt-template" || key === "data-source" || key === "proxy" || key === "about") {
            setActiveTab(key);
            return;
          }
          message.info("该设置页待接入");
        }}
      />
      {activeTab === "model-config" ? (
        <ModelConfigPage showTabs={false} />
      ) : activeTab === "prompt-template" ? (
        <PromptTemplatePage />
      ) : activeTab === "data-source" ? (
        <DataSourceSettingsPage />
      ) : activeTab === "proxy" ? (
        <ProxySettingsPage />
      ) : activeTab === "about" ? (
        <AboutAppPage />
      ) : (
        <SettingsBasicPage />
      )}
    </section>
  );
}
