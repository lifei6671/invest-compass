import { App as AntApp } from "antd";
import { useState } from "react";
import { SettingsRiskNotice } from "../components/SettingsRiskNotice";
import { DataSourceCredentialPage } from "./credentials/DataSourceCredentialPage";
import { DataSourceSubTabs, type DataSourceSubTabKey } from "./credentials/components/DataSourceSubTabs";
import { DataSourceDescriptionPage } from "./description/DataSourceDescriptionPage";
import { DataComplianceCard } from "./components/DataComplianceCard";
import { DataSourceBaseSettingsCard } from "./components/DataSourceBaseSettingsCard";
import { DataSourceHealthCard } from "./components/DataSourceHealthCard";
import { LocalCacheSnapshotCard } from "./components/LocalCacheSnapshotCard";
import { MarketDataSourceCard } from "./components/MarketDataSourceCard";
import { NewsSourceCard } from "./components/NewsSourceCard";
import { SyncStrategyCard } from "./components/SyncStrategyCard";
import {
  cacheSnapshot,
  healthItems,
  initialBaseSettings,
  marketDataSources,
  newsSources,
  syncStrategy,
  type DataSourceBaseSettings,
} from "./types";

export function DataSourceSettingsPage() {
  const { message } = AntApp.useApp();
  const [activeSubTab, setActiveSubTab] = useState<DataSourceSubTabKey>("description");
  const [baseSettings, setBaseSettings] = useState<DataSourceBaseSettings>(initialBaseSettings);

  const changeSubTab = (key: DataSourceSubTabKey) => {
    setActiveSubTab(key);
  };

  return (
    <>
      <DataSourceSubTabs activeKey={activeSubTab} onChange={changeSubTab} />
      {activeSubTab === "credentials" ? (
        <DataSourceCredentialPage />
      ) : activeSubTab === "description" ? (
        <DataSourceDescriptionPage />
      ) : (
        <>
          <DataSourceBaseSettingsCard
            value={baseSettings}
            onChange={(value) => {
              setBaseSettings(value);
              message.success("数据源设置已更新");
            }}
          />
          <div className="settings-basic-card-grid settings-data-source-card-grid">
            <MarketDataSourceCard items={marketDataSources} onTestConnection={() => message.success("数据源连接测试完成")} onEditConfig={() => message.info("行情数据源配置待接入")} />
            <NewsSourceCard items={newsSources} onSyncNow={() => message.success("新闻同步任务已提交")} onViewLog={() => message.info("同步日志待接入")} />
            <SyncStrategyCard value={syncStrategy} onViewScheduler={() => message.info("任务调度配置待接入")} />
            <LocalCacheSnapshotCard value={cacheSnapshot} onCleanCache={() => message.success("缓存清理完成")} />
            <DataSourceHealthCard items={healthItems} onRefresh={() => message.success("数据源状态检测完成")} />
            <DataComplianceCard onViewDescription={() => setActiveSubTab("description")} />
          </div>
          <SettingsRiskNotice />
        </>
      )}
    </>
  );
}
