export type MarketIndexTrend = "up" | "down";

export type MarketIndexItem = {
  name: string;
  code: string;
  value: string;
  change: string;
  changePercent: string;
  time: string;
  status: string;
  trend: MarketIndexTrend;
  sparkline: number[];
};

export type WatchlistDistribution = {
  total: number;
  up: {
    count: number;
    percent: number;
  };
  down: {
    count: number;
    percent: number;
  };
  flat: {
    count: number;
    percent: number;
  };
  avgChangePercent: string;
  upProbability: string;
  changeFromYesterday: string;
};

export type HotTopic = {
  rank: number;
  name: string;
  changePercent: string;
  summary: string;
};

export type RecentReport = {
  id: string;
  title: string;
  stock: string;
  analysisType: string;
  model: string;
  generatedAt: string;
};

export type RecentTaskStatus = "RUNNING" | "SUCCESS" | "FAILED";

export type RecentTask = {
  id: string;
  title: string;
  type: string;
  status: RecentTaskStatus;
  progress: number;
  summary: string;
};
