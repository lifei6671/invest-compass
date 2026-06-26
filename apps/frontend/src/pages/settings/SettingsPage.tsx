import { useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router-dom";
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

const settingsTabKeys: SettingsTabKey[] = [
  "basic",
  "model-config",
  "prompt-template",
  "data-source",
  "proxy",
  "notifications",
  "workspace",
  "cache",
  "about",
];

export function SettingsPage(props: SettingsPageProps = {}) {
  const location = useLocation();
  const queryTab = useMemo(() => new URLSearchParams(location.search).get("tab") as SettingsTabKey | null, [location.search]);
  const initialTab = props.initialActiveTab ?? (queryTab && settingsTabKeys.includes(queryTab) ? queryTab : "basic");
  const [activeTab, setActiveTab] = useState<SettingsTabKey>(initialTab);

  useEffect(() => {
    if (!props.initialActiveTab && queryTab && settingsTabKeys.includes(queryTab)) {
      setActiveTab(queryTab);
    }
  }, [props.initialActiveTab, queryTab, settingsTabKeys]);

  return (
    <section className="settings-basic-page">
      <SettingsTabs
        activeKey={activeTab}
        onChange={(key) => {
          if (settingsTabKeys.includes(key)) {
            setActiveTab(key);
            return;
          }
          setActiveTab("basic");
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
      ) : activeTab === "notifications" ? (
        <SettingsBasicPage section="notifications" onEditProxy={() => setActiveTab("proxy")} />
      ) : activeTab === "workspace" ? (
        <SettingsBasicPage section="workspace" onEditProxy={() => setActiveTab("proxy")} />
      ) : activeTab === "cache" ? (
        <SettingsBasicPage section="cache" onEditProxy={() => setActiveTab("proxy")} />
      ) : activeTab === "about" ? (
        <AboutAppPage />
      ) : (
        <SettingsBasicPage section="basic" onEditProxy={() => setActiveTab("proxy")} />
      )}
    </section>
  );
}
