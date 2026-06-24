export type MarketSource = "auto-fallback" | "akshare-eastmoney" | "eastmoney" | "sina" | "tencent" | "custom";
export type NewsSource = "aggregated" | "cls" | "sina" | "custom";
export type MarketScope = "CN" | "HK" | "US" | "ALL";
export type KlineRange = "1y" | "3y" | "5y" | "all";
export type QuoteRefreshInterval = "15s" | "30s" | "60s" | "120s" | "manual";
export type NewsSyncInterval = "5m" | "15m" | "30m" | "60m" | "manual";

export type DataSourceBaseSettings = {
  defaultMarketSource: MarketSource;
  defaultNewsSource: NewsSource;
  defaultMarketScope: MarketScope;
  klineRange: KlineRange;
  quoteRefreshInterval: QuoteRefreshInterval;
  newsSyncInterval: NewsSyncInterval;
  syncOnStartup: boolean;
  reduceFrequencyOutsideTradingHours: boolean;
};

export type SourceStatus = "normal" | "disabled" | "enabled" | "failed" | "unchecked";

export type MarketDataSourceItem = {
  label: string;
  source: string;
  status: SourceStatus;
};

export type NewsSourceItem = {
  label: string;
  source: string;
  status: SourceStatus;
};

export type SyncStrategy = {
  enabledJobsText: string;
  activeRunsText: string;
  failedRunsText: string;
  sourceText: string;
};

export type CacheSnapshot = {
  quoteCacheSize: string;
  newsCacheSize: string;
  latestSnapshotTime: string;
  retentionPolicy: string;
};

export type HealthStatusItem = {
  label: string;
  statusText: string;
  status: "normal" | "disabled" | "failed";
};

export const initialBaseSettings: DataSourceBaseSettings = {
  defaultMarketSource: "auto-fallback",
  defaultNewsSource: "aggregated",
  defaultMarketScope: "CN",
  klineRange: "5y",
  quoteRefreshInterval: "60s",
  newsSyncInterval: "30m",
  syncOnStartup: true,
  reduceFrequencyOutsideTradingHours: true,
};

export const initialSyncStrategy: SyncStrategy = {
  enabledJobsText: "后端未返回",
  activeRunsText: "后端未返回",
  failedRunsText: "后端未返回",
  sourceText: "scheduler_status",
};

export const initialCacheSnapshot: CacheSnapshot = {
  quoteCacheSize: "0 B",
  newsCacheSize: "0 B",
  latestSnapshotTime: "后端未提供",
  retentionPolicy: "后端未提供",
};
