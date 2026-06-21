import type { HotTopic, MarketIndexItem, RecentReport, RecentTask, WatchlistDistribution } from "./types";

export const marketIndexes: MarketIndexItem[] = [
  {
    name: "上证指数",
    code: "000001.SH",
    value: "3,367.46",
    change: "+12.34",
    changePercent: "+0.37%",
    time: "15:30:00",
    status: "已收盘",
    trend: "up",
    sparkline: [34, 36, 35, 39, 37, 41, 40, 43, 42, 47, 44, 46],
  },
  {
    name: "深证成指",
    code: "399001.SZ",
    value: "10,354.22",
    change: "+48.72",
    changePercent: "+0.47%",
    time: "15:30:00",
    status: "已收盘",
    trend: "up",
    sparkline: [28, 32, 31, 35, 33, 37, 34, 39, 42, 40, 44, 42],
  },
  {
    name: "创业板指",
    code: "399006.SZ",
    value: "2,047.65",
    change: "+23.17",
    changePercent: "+1.14%",
    time: "15:30:00",
    status: "已收盘",
    trend: "up",
    sparkline: [24, 27, 26, 30, 29, 34, 32, 36, 38, 36, 40, 39],
  },
  {
    name: "恒生指数",
    code: "HSI.HI",
    value: "18,823.45",
    change: "-120.56",
    changePercent: "-0.64%",
    time: "16:08:00",
    status: "已收盘",
    trend: "down",
    sparkline: [46, 43, 41, 42, 39, 38, 35, 37, 34, 36, 32, 35],
  },
];

export const watchlistDistribution: WatchlistDistribution = {
  total: 128,
  up: { count: 56, percent: 43.75 },
  down: { count: 58, percent: 45.31 },
  flat: { count: 14, percent: 10.94 },
  avgChangePercent: "+0.28%",
  upProbability: "43.75%",
  changeFromYesterday: "+6.21%",
};

export const hotTopics: HotTopic[] = [
  { rank: 1, name: "半导体", changePercent: "+2.35%", summary: "国产芯片持续发力，产业链景气度提升" },
  { rank: 2, name: "新能源汽车", changePercent: "+1.86%", summary: "销量增长超预期，产业链价格企稳" },
  { rank: 3, name: "光伏设备", changePercent: "+1.62%", summary: "海外需求回暖，组件出口数据向好" },
  { rank: 4, name: "软件开发", changePercent: "+1.28%", summary: "AI+应用落地加速，行业订单改善" },
  { rank: 5, name: "银行", changePercent: "+0.98%", summary: "估值修复预期增强，资金持续流入" },
];

export const recentReports: RecentReport[] = [
  { id: "report-1", title: "宁德时代（300750）深度分析报告", stock: "300750.SZ", analysisType: "深度分析", model: "gpt-4o", generatedAt: "05-20 14:25" },
  { id: "report-2", title: "贵州茅台（600519）基本面跟踪", stock: "600519.SH", analysisType: "基本面分析", model: "gpt-4o-mini", generatedAt: "05-20 11:18" },
  { id: "report-3", title: "中芯国际（688981）技术面分析", stock: "688981.SH", analysisType: "技术面分析", model: "qwen-max", generatedAt: "05-19 19:42" },
  { id: "report-4", title: "比亚迪（002594）综合分析报告", stock: "002594.SZ", analysisType: "深度分析", model: "gpt-4o", generatedAt: "05-19 16:33" },
  { id: "report-5", title: "三一重工（600031）简要跟踪", stock: "600031.SH", analysisType: "跟踪报告", model: "gpt-4o-mini", generatedAt: "05-19 09:21" },
];

export const recentTasks: RecentTask[] = [
  { id: "task-1", title: "宁德时代深度分析", type: "AI 分析报告", status: "RUNNING", progress: 68, summary: "正在生成报告中..." },
  { id: "task-2", title: "市场热点总结", type: "市场分析", status: "SUCCESS", progress: 100, summary: "热点行业：半导体、..." },
  { id: "task-3", title: "中芯国际技术面分析", type: "AI 分析报告", status: "SUCCESS", progress: 100, summary: "技术面偏强，关注支..." },
  { id: "task-4", title: "自选股数据更新", type: "数据更新", status: "SUCCESS", progress: 100, summary: "更新 128 只股票数据..." },
  { id: "task-5", title: "新闻资讯抓取", type: "资讯抓取", status: "FAILED", progress: 0, summary: "网络超时，请稍后重试" },
];
