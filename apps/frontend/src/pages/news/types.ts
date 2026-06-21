export type NewsSource =
  | "财联社"
  | "证券时报"
  | "芯榜"
  | "界面新闻"
  | "同花顺资讯"
  | "上海证券报"
  | "Wind 资讯"
  | "第一财经"
  | "证券日报";

export type NewsItem = {
  id: string;
  source: NewsSource;
  timeLabel: string;
  title: string;
  summary: string;
  tags: string[];
  url?: string;
};

export type NewsFilters = {
  keyword: string;
  stock: string;
  source: string;
  industry: string;
  timeRange: string;
};

export type HotIndustry = {
  rank: number;
  name: string;
  heat: number;
};

export type MentionedStock = {
  name: string;
  count: number;
};

export type SentimentSummary = {
  positive: {
    count: number;
    percent: number;
  };
  neutral: {
    count: number;
    percent: number;
  };
  negative: {
    count: number;
    percent: number;
  };
  summary: string;
};

export type DataSourceStatus = {
  name: string;
  status: "normal" | "failed" | "disabled";
  lastUpdatedAt: string;
  cacheStatus: "good" | "warning" | "bad";
};

