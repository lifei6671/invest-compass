import { App as AntApp } from "antd";
import { useCallback, useEffect, useState } from "react";
import { settingsGet, settingsSet, type SettingItem } from "../../../services/coreClient";
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
  type KlineRange,
  type MarketScope,
  type MarketSource,
  type NewsSource,
  type NewsSyncInterval,
  type QuoteRefreshInterval,
} from "./types";

const dataSourceSettingKeyByField: Record<keyof DataSourceBaseSettings, string> = {
  defaultMarketSource: "data_source.default_market_source",
  defaultNewsSource: "data_source.default_news_source",
  defaultMarketScope: "data_source.default_market_scope",
  klineRange: "data_source.kline_range",
  quoteRefreshInterval: "data_source.quote_refresh_interval",
  newsSyncInterval: "data_source.news_sync_interval",
  syncOnStartup: "data_source.sync_on_startup",
  reduceFrequencyOutsideTradingHours: "data_source.reduce_frequency_outside_trading_hours",
};

const dataSourceSettingsKeys = Object.values(dataSourceSettingKeyByField);
const marketSourceValues: MarketSource[] = ["auto-fallback", "akshare-eastmoney", "eastmoney", "sina", "tencent", "custom"];
const newsSourceValues: NewsSource[] = ["aggregated", "cls", "sina", "custom"];
const marketScopeValues: MarketScope[] = ["CN", "HK", "US", "ALL"];
const klineRangeValues: KlineRange[] = ["1y", "3y", "5y", "all"];
const quoteRefreshValues: QuoteRefreshInterval[] = ["15s", "30s", "60s", "120s", "manual"];
const newsSyncValues: NewsSyncInterval[] = ["5m", "15m", "30m", "60m", "manual"];

export function DataSourceSettingsPage() {
  const { message } = AntApp.useApp();
  const [activeSubTab, setActiveSubTab] = useState<DataSourceSubTabKey>("overview");
  const [baseSettings, setBaseSettings] = useState<DataSourceBaseSettings>(initialBaseSettings);

  const loadDataSourceSettings = useCallback(async () => {
    try {
      const settings = await settingsGet(dataSourceSettingsKeys);
      setBaseSettings((current) => ({ ...current, ...dataSourceSettingsFromItems(settings.items) }));
    } catch (error) {
      message.error(error instanceof Error ? error.message : "数据源设置读取失败");
    }
  }, [message]);

  useEffect(() => {
    void loadDataSourceSettings();
  }, [loadDataSourceSettings]);

  const changeSubTab = (key: DataSourceSubTabKey) => {
    setActiveSubTab(key);
  };

  const handleBaseSettingsChange = (value: DataSourceBaseSettings) => {
    const previousValue = baseSettings;
    setBaseSettings(value);
    const changedKey = findChangedDataSourceSettingsKey(previousValue, value);
    if (!changedKey) {
      return;
    }
    void saveDataSourceSetting(changedKey, value[changedKey], previousValue);
  };

  const saveDataSourceSetting = async (field: keyof DataSourceBaseSettings, value: DataSourceBaseSettings[keyof DataSourceBaseSettings], previousValue: DataSourceBaseSettings) => {
    try {
      await settingsSet({ items: [{ key: dataSourceSettingKeyByField[field], value: String(value) }] });
      message.success("数据源设置已更新");
    } catch (error) {
      setBaseSettings(previousValue);
      message.error(error instanceof Error ? error.message : "数据源设置保存失败");
    }
  };

  return (
    <>
      <DataSourceSubTabs activeKey={activeSubTab} onChange={changeSubTab} />
      {activeSubTab === "credentials" ? (
        <DataSourceCredentialPage />
      ) : activeSubTab === "description" ? (
        <DataSourceDescriptionPage
          onViewOverview={() => setActiveSubTab("overview")}
          onViewCredentials={() => setActiveSubTab("credentials")}
        />
      ) : (
        <>
          <DataSourceBaseSettingsCard
            value={baseSettings}
            onChange={handleBaseSettingsChange}
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

function dataSourceSettingsFromItems(items: SettingItem[]): Partial<DataSourceBaseSettings> {
  const values = new Map(items.map((item) => [item.key, item.value]));
  const settings: Partial<DataSourceBaseSettings> = {};
  assignIfDefined(settings, "defaultMarketSource", readEnum(values, dataSourceSettingKeyByField.defaultMarketSource, marketSourceValues));
  assignIfDefined(settings, "defaultNewsSource", readEnum(values, dataSourceSettingKeyByField.defaultNewsSource, newsSourceValues));
  assignIfDefined(settings, "defaultMarketScope", readEnum(values, dataSourceSettingKeyByField.defaultMarketScope, marketScopeValues));
  assignIfDefined(settings, "klineRange", readEnum(values, dataSourceSettingKeyByField.klineRange, klineRangeValues));
  assignIfDefined(settings, "quoteRefreshInterval", readEnum(values, dataSourceSettingKeyByField.quoteRefreshInterval, quoteRefreshValues));
  assignIfDefined(settings, "newsSyncInterval", readEnum(values, dataSourceSettingKeyByField.newsSyncInterval, newsSyncValues));
  assignIfDefined(settings, "syncOnStartup", readBoolean(values, dataSourceSettingKeyByField.syncOnStartup));
  assignIfDefined(settings, "reduceFrequencyOutsideTradingHours", readBoolean(values, dataSourceSettingKeyByField.reduceFrequencyOutsideTradingHours));
  return settings;
}

function assignIfDefined<Key extends keyof DataSourceBaseSettings>(settings: Partial<DataSourceBaseSettings>, key: Key, value: DataSourceBaseSettings[Key] | undefined) {
  if (value !== undefined) {
    settings[key] = value;
  }
}

function readEnum<Value extends string>(values: Map<string, string>, key: string, allowedValues: Value[]): Value | undefined {
  const value = values.get(key);
  if (!value) {
    return undefined;
  }
  return allowedValues.includes(value as Value) ? (value as Value) : undefined;
}

function readBoolean(values: Map<string, string>, key: string): boolean | undefined {
  const value = values.get(key);
  if (value === "true") {
    return true;
  }
  if (value === "false") {
    return false;
  }
  return undefined;
}

function findChangedDataSourceSettingsKey(previousValue: DataSourceBaseSettings, nextValue: DataSourceBaseSettings): keyof DataSourceBaseSettings | undefined {
  return (Object.keys(dataSourceSettingKeyByField) as Array<keyof DataSourceBaseSettings>).find((key) => previousValue[key] !== nextValue[key]);
}
