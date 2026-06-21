export type StockDetail = {
  name: string;
  symbol: string;
  code: string;
  industry: string;
  subIndustry: string;
  concepts: string[];
  price: number;
  changeAmount: number;
  changePercent: number;
  open: number;
  high: number;
  low: number;
  previousClose: number;
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
};

export type KlineItem = {
  date: string;
  open: number;
  close: number;
  low: number;
  high: number;
  volume: number;
};

export const stockDetailMock: StockDetail = {
  name: "生益科技",
  symbol: "CN:SH:600183",
  code: "600183.SH",
  industry: "电子材料",
  subIndustry: "覆铜板",
  concepts: ["PCB概念", "5G", "光通信"],
  price: 25.68,
  changeAmount: 0.36,
  changePercent: 1.42,
  open: 25.32,
  high: 25.88,
  low: 25.32,
  previousClose: 25.32,
  turnoverRate: "1.72%",
  amount: "8.92亿",
  volume: "34.72万手",
  updateTime: "2025-05-20 15:30:00",
};

export const basicInfoItems: BasicInfoItem[] = [
  { label: "市盈率（TTM）", value: "27.11" },
  { label: "市净率（LF）", value: "3.21" },
  { label: "市销率（TTM）", value: "2.68" },
  { label: "换手率", value: "1.72%" },
  { label: "成交额", value: "8.92亿" },
  { label: "成交量", value: "34.72万手" },
  { label: "总市值", value: "680.34亿" },
  { label: "流通市值", value: "515.21亿" },
  { label: "所属行业", value: "电子材料" },
  { label: "细分行业", value: "覆铜板" },
];

export const technicalIndicators: TechnicalIndicator[] = [
  { name: "MA5", value: "25.28", direction: "up", desc: "5日均线" },
  { name: "MA10", value: "24.92", direction: "up", desc: "10日均线" },
  { name: "MA20", value: "24.18", direction: "up", desc: "20日均线" },
  { name: "MA60", value: "23.45", direction: "up", desc: "60日均线" },
  { name: "MA120", value: "22.91", direction: "up", desc: "120日均线" },
  { name: "MA250", value: "21.76", direction: "up", desc: "250日均线" },
];

export const stockNewsItems: StockNewsItem[] = [
  {
    id: "news-1",
    title: "生益科技：一季度归母净利润同比增长18.35% 产品结构持续优化",
    source: "证券时报",
    publishedAt: "2025-05-20 10:15",
  },
  {
    id: "news-2",
    title: "生益科技：公司高端覆铜板产能持续释放 订单饱满",
    source: "上海证券报",
    publishedAt: "2025-05-19 16:22",
  },
  {
    id: "news-3",
    title: "PCB行业景气度回升 上游覆铜板企业受益明显",
    source: "中证网",
    publishedAt: "2025-05-19 14:05",
  },
  {
    id: "news-4",
    title: "生益科技：拟投资新建高端覆铜板项目 加快技术升级",
    source: "中国证券报",
    publishedAt: "2025-05-18 20:31",
  },
];

export const conceptTags = ["PCB概念", "5G", "光通信", "半导体材料"];

export const myStockTags = ["长线跟踪", "业绩观察", "技术面关注"];

export const stockKlineItems: KlineItem[] = [
  { date: "2025-01-20", open: 20.1, close: 20.42, low: 19.85, high: 20.72, volume: 22 },
  { date: "2025-01-23", open: 20.42, close: 20.9, low: 20.24, high: 21.2, volume: 28 },
  { date: "2025-01-26", open: 20.9, close: 21.28, low: 20.68, high: 21.62, volume: 31 },
  { date: "2025-02-05", open: 21.28, close: 21.12, low: 20.92, high: 21.58, volume: 24 },
  { date: "2025-02-10", open: 21.12, close: 21.78, low: 20.98, high: 22.1, volume: 36 },
  { date: "2025-02-14", open: 21.78, close: 22.16, low: 21.44, high: 22.36, volume: 33 },
  { date: "2025-02-19", open: 22.16, close: 21.88, low: 21.66, high: 22.42, volume: 26 },
  { date: "2025-02-24", open: 21.88, close: 22.35, low: 21.72, high: 22.65, volume: 30 },
  { date: "2025-02-27", open: 22.35, close: 22.62, low: 22.02, high: 22.88, volume: 38 },
  { date: "2025-03-04", open: 22.62, close: 23.05, low: 22.46, high: 23.34, volume: 40 },
  { date: "2025-03-07", open: 23.05, close: 22.84, low: 22.6, high: 23.2, volume: 29 },
  { date: "2025-03-12", open: 22.84, close: 23.22, low: 22.7, high: 23.58, volume: 35 },
  { date: "2025-03-17", open: 23.22, close: 23.58, low: 23.0, high: 23.86, volume: 41 },
  { date: "2025-03-20", open: 23.58, close: 23.92, low: 23.4, high: 24.12, volume: 43 },
  { date: "2025-03-25", open: 23.92, close: 24.35, low: 23.7, high: 24.58, volume: 46 },
  { date: "2025-03-28", open: 24.35, close: 24.84, low: 24.1, high: 25.08, volume: 52 },
  { date: "2025-04-02", open: 24.84, close: 24.62, low: 24.28, high: 25.02, volume: 44 },
  { date: "2025-04-07", open: 24.62, close: 24.08, low: 23.74, high: 24.78, volume: 39 },
  { date: "2025-04-10", open: 24.08, close: 22.72, low: 21.88, high: 24.22, volume: 60 },
  { date: "2025-04-15", open: 22.72, close: 23.46, low: 22.42, high: 23.78, volume: 48 },
  { date: "2025-04-18", open: 23.46, close: 23.88, low: 23.22, high: 24.12, volume: 42 },
  { date: "2025-04-23", open: 23.88, close: 24.24, low: 23.62, high: 24.5, volume: 45 },
  { date: "2025-04-28", open: 24.24, close: 24.76, low: 24.0, high: 25.1, volume: 50 },
  { date: "2025-05-06", open: 24.76, close: 25.48, low: 24.52, high: 26.18, volume: 62 },
  { date: "2025-05-09", open: 25.48, close: 25.16, low: 24.92, high: 25.82, volume: 55 },
  { date: "2025-05-14", open: 25.16, close: 25.42, low: 24.88, high: 25.7, volume: 46 },
  { date: "2025-05-19", open: 25.32, close: 25.68, low: 25.32, high: 25.88, volume: 34.72 },
];
