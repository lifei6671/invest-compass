import type { AnalysisConfig, AnalysisType, ContextSummary, OptionalHoldingContext, RiskPreference } from "./types";

export const analysisTypes: AnalysisType[] = ["个股综合分析", "技术面分析", "基本面分析", "消息面分析"];

export const riskPreferences: RiskPreference[] = ["保守", "中等", "积极"];

export const initialAnalysisConfig: AnalysisConfig = {
  stock: {
    name: "",
    symbol: "",
    code: "",
  },
  analysisType: "个股综合分析",
  aiModel: "",
  promptTemplate: "",
};

export const initialHoldingContext: OptionalHoldingContext = {
  costPrice: "",
  shares: "",
  riskPreference: "中等",
};

export const emptyContextSummary: ContextSummary = {
  basicInfo: {
    companyName: "暂无",
    industry: "暂无",
    listDate: "暂无",
    totalMarketCap: "暂无",
    floatMarketCap: "暂无",
  },
  quote: {
    price: "暂无",
    changeAmount: "暂无",
    changePercent: "暂无",
    open: "暂无",
    high: "暂无",
    low: "暂无",
    amount: "暂无",
    volume: "暂无",
    turnoverRate: "暂无",
    updateTime: "待加载",
  },
  kline: {
    change20d: "暂无",
    change60d: "暂无",
    ytdChange: "暂无",
    ma20: "暂无",
    ma60: "暂无",
    ma120: "暂无",
  },
  indicators: {
    ma: "暂无",
    macd: "暂无",
    rsi: "暂无",
    kdj: "暂无",
    boll: "暂无",
  },
  news: {
    count: 0,
    period: "待加载",
    items: [],
  },
};
