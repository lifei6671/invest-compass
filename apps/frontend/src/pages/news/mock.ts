import type { DataSourceStatus, HotIndustry, MentionedStock, NewsFilters, NewsItem, SentimentSummary } from "./types";

export const initialNewsFilters: NewsFilters = {
  keyword: "",
  stock: "全部股票",
  source: "全部来源",
  industry: "全部行业",
  timeRange: "近 7 天",
};

export const newsItems: NewsItem[] = [
  {
    id: "news-1",
    source: "财联社",
    timeLabel: "今天 14:28",
    title: "生益科技：公司高端覆铜板产品订单饱满，持续提升 AI 服务器领域份额",
    summary: "公司在高频、高速覆铜板领域持续突破，产品已批量应用于多家头部 AI 服务器厂商。",
    tags: ["生益科技", "PCB/覆铜板"],
  },
  {
    id: "news-2",
    source: "证券时报",
    timeLabel: "今天 13:52",
    title: "英伟达 Blackwell 需求强劲，光模块厂商迎来新一轮订单增长",
    summary: "机构预计 2025 年下半年 800G/1.6T 光模块需求将显著提升，相关供应链受益明显。",
    tags: ["中际旭创", "新易盛", "光模块"],
  },
  {
    id: "news-3",
    source: "芯榜",
    timeLabel: "今天 12:31",
    title: "国产 AI 芯片加速迭代，多家厂商发布新一代训练推理芯片",
    summary: "国产 AI 芯片在算力、带宽、能效等方面持续优化，生态建设进入关键阶段。",
    tags: ["寒武纪", "海光信息", "AI芯片"],
  },
  {
    id: "news-4",
    source: "界面新闻",
    timeLabel: "今天 11:05",
    title: "半导体设备国产化率提升，刻蚀机与薄膜设备需求旺盛",
    summary: "多家晶圆厂扩产带动设备采购，国产刻蚀机在 14nm 以下工艺实现突破。",
    tags: ["北方华创", "中微公司", "半导体设备"],
  },
  {
    id: "news-5",
    source: "同花顺资讯",
    timeLabel: "今天 10:22",
    title: "PCB 板块震荡走强，服务器需求拉动高端板材景气度",
    summary: "机构指出 AI 服务器放量将长期驱动高端 PCB 板材需求与盈利能力提升。",
    tags: ["沪电股份", "深南电路", "PCB"],
  },
  {
    id: "news-6",
    source: "上海证券报",
    timeLabel: "今天 09:41",
    title: "储能电池价格企稳，多家企业上调二季度排产计划",
    summary: "需求端逐步回暖，海外储能订单增长显著，龙头企业产能利用率提升。",
    tags: ["宁德时代", "亿纬锂能", "储能"],
  },
  {
    id: "news-7",
    source: "Wind 资讯",
    timeLabel: "今天 08:33",
    title: "电力设备板块获北向资金增持，光伏逆变器出口保持高增速",
    summary: "海外市场需求旺盛，头部企业产品结构优化，毛利率环比改善。",
    tags: ["阳光电源", "锦浪科技", "电力设备"],
  },
  {
    id: "news-8",
    source: "第一财经",
    timeLabel: "昨天 22:18",
    title: "行业周报：AI 基础设施持续高景气，关注算力与散热产业链",
    summary: "算力需求持续增长，散热、PCB、电源等环节受益于 AI 基础设施扩张。",
    tags: ["AI硬件", "服务器", "散热"],
  },
  {
    id: "news-9",
    source: "证券日报",
    timeLabel: "昨天 20:37",
    title: "Chat量上市公司披露回购计划，半导体与通信板块居多",
    summary: "AI 应用场景持续拓展，推动算力与模型侧投资机会。",
    tags: ["AI应用", "云计算"],
  },
];

export const hotKeywords = ["AI芯片", "光模块", "PCB", "半导体设备", "电力设备", "储能", "机器人"];

export const hotIndustries: HotIndustry[] = [
  { rank: 1, name: "AI硬件", heat: 92 },
  { rank: 2, name: "半导体", heat: 88 },
  { rank: 3, name: "PCB/覆铜板", heat: 76 },
  { rank: 4, name: "光模块/CPO", heat: 72 },
  { rank: 5, name: "电力设备", heat: 61 },
];

export const mentionedStocks: MentionedStock[] = [
  { name: "中际旭创", count: 128 },
  { name: "新易盛", count: 112 },
  { name: "生益科技", count: 98 },
  { name: "沪电股份", count: 86 },
  { name: "英伟达(NVDA)", count: 72 },
  { name: "北方华创", count: 65 },
  { name: "寒武纪", count: 61 },
  { name: "阳光电源", count: 54 },
  { name: "比亚迪", count: 49 },
  { name: "宁德时代", count: 46 },
];

export const sentimentSummary: SentimentSummary = {
  positive: { count: 218, percent: 52 },
  neutral: { count: 156, percent: 37 },
  negative: { count: 43, percent: 11 },
  summary: "整体情绪偏向乐观，AI 硬件与半导体相关资讯占比提升。",
};

export const dataSourceStatuses: DataSourceStatus[] = [
  { name: "财联社", status: "normal", lastUpdatedAt: "15:29:58", cacheStatus: "good" },
  { name: "同花顺资讯", status: "normal", lastUpdatedAt: "15:29:44", cacheStatus: "good" },
  { name: "东方财富资讯", status: "normal", lastUpdatedAt: "15:29:31", cacheStatus: "good" },
  { name: "Wind 资讯", status: "normal", lastUpdatedAt: "15:29:20", cacheStatus: "good" },
  { name: "上海证券报", status: "normal", lastUpdatedAt: "15:29:10", cacheStatus: "good" },
  { name: "界面新闻", status: "normal", lastUpdatedAt: "15:28:56", cacheStatus: "good" },
];

