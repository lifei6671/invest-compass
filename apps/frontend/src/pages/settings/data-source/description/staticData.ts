import type { AIContextDataType, AIOutputNature, DataFreshnessItem, DataSourceExplanationItem, DataUsageOverview, FAQItem, FieldDescription } from "./types";

export const dataUsageOverview: DataUsageOverview = {
  scope: "总览 / 自选股 / 个股详情 / 资讯中心 / AI 分析",
  defaultMarket: "A股",
  marketSource: "AkShare / EastMoney",
  newsSource: "聚合新闻源 / 个股相关新闻",
  klineRange: "近 5 年",
  usage: "本地研究展示 / 上下文构建 / 历史快照",
  disclaimer: "不用于交易执行",
};

export const sourceItems: DataSourceExplanationItem[] = [
  { name: "行情数据", source: "AkShare / EastMoney", description: "实时行情与分时/盘口数据", status: "normal" },
  { name: "K线历史", source: "AkShare", description: "日线/周线/分钟线（近 5 年）", status: "normal" },
  { name: "基础资料", source: "AkShare", description: "公司资料、财务指标等", status: "normal" },
  { name: "个股新闻", source: "聚合新闻源", description: "个股相关新闻", status: "normal" },
  { name: "市场新闻", source: "聚合新闻源", description: "宏观/行业/市场综合新闻", status: "normal" },
  { name: "扩展海外源", source: "Alpha Vantage", description: "美股等海外行情（可选）", status: "limited" },
];

export const freshnessItems: DataFreshnessItem[] = [
  { label: "行情轮询频率", value: "2 秒（盘中）/ 10 秒（非交易时段）" },
  { label: "K线同步时点", value: "每日交易日收盘后完成" },
  { label: "新闻增量同步", value: "5 分钟（盘中）/ 15 分钟（非交易时段）" },
  { label: "缓存更新时间", value: "每次同步完成后" },
  { label: "快照保留周期", value: "30 天（本地可配置）" },
];

export const aiContextDataTypes: AIContextDataType[] = ["行情数据", "K线数据", "技术指标", "新闻摘要", "快照数据"];

export const aiOutputNatures: AIOutputNature[] = [
  { type: "事实", description: "来自原始数据或公开信息，可直接验证", color: "blue" },
  { type: "推断", description: "基于数据逻辑与模型推导，存在不确定性", color: "orange" },
  { type: "观点", description: "模型综合判断与建议，不构成投资建议", color: "green" },
];

export const fieldDescriptions: FieldDescription[] = [
  { field: "现价", description: "最新成交价格" },
  { field: "涨跌额", description: "相对昨日收盘价的变动金额" },
  { field: "涨跌幅", description: "相对昨日收盘价的变动百分比" },
  { field: "成交额", description: "当日成交金额（汇总）" },
  { field: "换手率", description: "当日成交量 / 流通股本" },
  { field: "市盈率", description: "静态市盈率（TTM）" },
  { field: "数据更新时间", description: "指本页数据最后刷新时刻" },
];

export const faqItems: FAQItem[] = [
  { id: "time-diff", question: "为什么不同页面时间不完全一致？" },
  { id: "ai-lag", question: "为什么 AI 报告与页面最新行情略有差异？" },
  { id: "credential", question: "为什么部分资讯需要凭据？" },
];
