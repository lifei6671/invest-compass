export type StockDetail = {
  name: string;
  symbol: string;
  code: string;
  industry: string;
  subIndustry: string;
  concepts: string[];
  price: number | null;
  changeAmount: number | null;
  changePercent: number | null;
  open: number | null;
  high: number | null;
  low: number | null;
  previousClose: number | null;
  turnoverRate: string;
  amount: string;
  volume: string;
  updateTime: string;
};

export type BasicInfoItem = {
  label: string;
  value: string;
};

export type TechnicalIndicator = {
  name: string;
  value: string;
  direction: "up" | "down" | "flat";
  desc: string;
};

export type StockNewsItem = {
  id: string;
  title: string;
  source: string;
  publishedAt: string;
  url?: string;
};

export type KlineItem = {
  date: string;
  open: number;
  close: number;
  low: number;
  high: number;
  volume: number;
};
