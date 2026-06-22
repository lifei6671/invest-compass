export type NewsSource = string;

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
