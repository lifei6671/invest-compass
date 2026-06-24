import { App as AntApp } from "antd";
import { useCallback, useEffect, useState } from "react";
import {
  cacheStats,
  providersStatus,
  schedulerStatus,
  settingsGet,
  settingsSet,
  type CacheStatsResult,
  type ProviderStatusItem,
  type SchedulerStatus,
  type SettingItem,
} from "../../../services/coreClient";
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
  initialCacheSnapshot,
  initialBaseSettings,
  initialSyncStrategy,
  type CacheSnapshot,
  type DataSourceBaseSettings,
  type HealthStatusItem,
  type KlineRange,
  type MarketDataSourceItem,
  type MarketScope,
  type MarketSource,
  type NewsSourceItem,
  type NewsSource,
  type NewsSyncInterval,
  type QuoteRefreshInterval,
  type SyncStrategy,
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
  const [overview, setOverview] = useState<DataSourceOverviewState>(initialOverviewState);

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

  const loadOverviewStatus = useCallback(async () => {
    const [providerResult, cacheResult, schedulerResult] = await Promise.allSettled([
      providersStatus(),
      cacheStats(),
      schedulerStatus(),
    ]);

    setOverview({
      providers: providerResult.status === "fulfilled" ? providerResult.value.items : [],
      providerError: providerResult.status === "rejected" ? errorMessage(providerResult.reason, "Provider 状态读取失败") : "",
      cacheSnapshot: cacheResult.status === "fulfilled" ? cacheSnapshotFromStats(cacheResult.value) : failedCacheSnapshot(errorMessage(cacheResult.reason, "缓存统计读取失败")),
      syncStrategy: schedulerResult.status === "fulfilled" ? syncStrategyFromStatus(schedulerResult.value) : failedSyncStrategy(errorMessage(schedulerResult.reason, "调度状态读取失败")),
      cacheError: cacheResult.status === "rejected" ? errorMessage(cacheResult.reason, "缓存统计读取失败") : "",
      schedulerError: schedulerResult.status === "rejected" ? errorMessage(schedulerResult.reason, "调度状态读取失败") : "",
    });
  }, []);

  useEffect(() => {
    if (activeSubTab === "overview") {
      void loadOverviewStatus();
    }
  }, [activeSubTab, loadOverviewStatus]);

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
            <MarketDataSourceCard items={marketDataSourcesFromProviders(overview.providers)} onTestConnection={() => message.info("数据源连接测试待接入")} onEditConfig={() => message.info("行情数据源配置待接入")} />
            <NewsSourceCard items={newsSourcesFromProviders(overview.providers)} onSyncNow={() => message.info("新闻同步待接入")} onViewLog={() => message.info("同步日志待接入")} />
            <SyncStrategyCard value={overview.syncStrategy} onViewScheduler={() => message.info("任务调度配置待接入")} />
            <LocalCacheSnapshotCard value={overview.cacheSnapshot} onCleanCache={() => message.info("数据源缓存清理待接入")} />
            <DataSourceHealthCard items={healthItemsFromOverview(overview)} onRefresh={loadOverviewStatus} />
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

type DataSourceOverviewState = {
  providers: ProviderStatusItem[];
  providerError: string;
  cacheSnapshot: CacheSnapshot;
  syncStrategy: SyncStrategy;
  cacheError: string;
  schedulerError: string;
};

const initialOverviewState: DataSourceOverviewState = {
  providers: [],
  providerError: "",
  cacheSnapshot: initialCacheSnapshot,
  syncStrategy: initialSyncStrategy,
  cacheError: "",
  schedulerError: "",
};

function marketDataSourcesFromProviders(providers: ProviderStatusItem[]): MarketDataSourceItem[] {
  return providers
    .filter((provider) => !isNewsProvider(provider))
    .map((provider) => ({
      label: provider.name,
      source: provider.source || "后端未返回",
      status: providerStatus(provider),
    }));
}

function newsSourcesFromProviders(providers: ProviderStatusItem[]): NewsSourceItem[] {
  return providers
    .filter(isNewsProvider)
    .map((provider) => ({
      label: provider.name,
      source: provider.source || "后端未返回",
      status: providerStatus(provider),
    }));
}

function healthItemsFromOverview(overview: DataSourceOverviewState): HealthStatusItem[] {
  const items = overview.providers.map((provider) => ({
    label: provider.name,
    statusText: provider.available ? "正常" : provider.last_error || (provider.source === "unconfigured" ? "未配置" : "异常"),
    status: provider.available ? "normal" as const : provider.source === "unconfigured" ? "disabled" as const : "failed" as const,
  }));
  if (overview.providerError) {
    items.push({ label: "Provider 状态", statusText: overview.providerError, status: "failed" });
  }
  if (overview.cacheError) {
    items.push({ label: "缓存统计", statusText: overview.cacheError, status: "failed" });
  }
  if (overview.schedulerError) {
    items.push({ label: "调度状态", statusText: overview.schedulerError, status: "failed" });
  }
  return items.length > 0 ? items : [{ label: "Provider 状态", statusText: "暂无后端状态", status: "disabled" }];
}

function providerStatus(provider: ProviderStatusItem): MarketDataSourceItem["status"] {
  if (provider.available) {
    return "normal";
  }
  return provider.source === "unconfigured" ? "disabled" : "failed";
}

function isNewsProvider(provider: ProviderStatusItem): boolean {
  const text = `${provider.name} ${provider.source}`.toLowerCase();
  return text.includes("news") || text.includes("新闻") || text.includes("资讯") || text.includes("cls") || text.includes("财联社");
}

function cacheSnapshotFromStats(stats: CacheStatsResult): CacheSnapshot {
  const quoteBytes = bytesForTarget(stats, "quote");
  const newsBytes = bytesForTarget(stats, "news");
  return {
    quoteCacheSize: formatBytes(quoteBytes),
    newsCacheSize: formatBytes(newsBytes),
    latestSnapshotTime: "后端未提供",
    retentionPolicy: formatBytes(stats.total_bytes),
  };
}

function failedCacheSnapshot(message: string): CacheSnapshot {
  return {
    quoteCacheSize: "读取失败",
    newsCacheSize: "读取失败",
    latestSnapshotTime: "后端未提供",
    retentionPolicy: message,
  };
}

function bytesForTarget(stats: CacheStatsResult, target: string): number {
  return stats.items.find((item) => item.target === target)?.bytes ?? 0;
}

function syncStrategyFromStatus(status: SchedulerStatus): SyncStrategy {
  return {
    enabledJobsText: `${status.jobs_enabled} / ${status.jobs_total}`,
    activeRunsText: `${status.queued_runs} / ${status.running_runs}`,
    failedRunsText: String(status.failed_runs),
    sourceText: "scheduler_status",
  };
}

function failedSyncStrategy(message: string): SyncStrategy {
  return {
    enabledJobsText: "读取失败",
    activeRunsText: "读取失败",
    failedRunsText: "读取失败",
    sourceText: message,
  };
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${formatNumber(bytes / 1024)} KB`;
  }
  return `${formatNumber(bytes / 1024 / 1024)} MB`;
}

function formatNumber(value: number): string {
  return Number.isInteger(value) ? String(value) : value.toFixed(1);
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}
