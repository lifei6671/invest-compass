export type MarketSource = "akshare-eastmoney" | "eastmoney" | "sina-tencent" | "custom";
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
  status: SourceStatus;
};

export type SyncStrategy = {
  quotePolling: boolean;
  newsSyncInterval: string;
  retryTimes: number;
  startupWarmup: boolean;
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
  defaultMarketSource: "akshare-eastmoney",
  defaultNewsSource: "aggregated",
  defaultMarketScope: "CN",
  klineRange: "5y",
  quoteRefreshInterval: "60s",
  newsSyncInterval: "30m",
  syncOnStartup: true,
  reduceFrequencyOutsideTradingHours: true,
};

export const marketDataSources: MarketDataSourceItem[] = [
  { label: "A股行情主源", status: "normal", source: "EastMoney" },
  { label: "港股 / 美股扩展源", status: "disabled", source: "Alpha Vantage" },
  { label: "K线历史数据", status: "normal", source: "AkShare" },
  { label: "估值与基础资料", status: "normal", source: "EastMoney" },
];

export const newsSources: NewsSourceItem[] = [
  { label: "市场新闻聚合", status: "normal" },
  { label: "个股相关新闻", status: "normal" },
  { label: "行业热点抓取", status: "normal" },
  { label: "去重与摘要缓存", status: "enabled" },
];

export const syncStrategy: SyncStrategy = {
  quotePolling: true,
  newsSyncInterval: "每 30 分钟",
  retryTimes: 3,
  startupWarmup: true,
};

export const cacheSnapshot: CacheSnapshot = {
  quoteCacheSize: "186.4 MB",
  newsCacheSize: "92.7 MB",
  latestSnapshotTime: "2025-05-20 15:28:41",
  retentionPolicy: "最近 30 天",
};

export const healthItems: HealthStatusItem[] = [
  { label: "Go Core 数据适配层", statusText: "正常", status: "normal" },
  { label: "SQLite 本地落库", statusText: "正常", status: "normal" },
  { label: "行情 Provider", statusText: "正常", status: "normal" },
  { label: "新闻 Provider", statusText: "正常", status: "normal" },
  { label: "扩展海外源", statusText: "未启用", status: "disabled" },
];
