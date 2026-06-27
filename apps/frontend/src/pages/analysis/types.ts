import type { ReactNode } from "react";

export type SelectedStock = {
  name: string;
  symbol: string;
  code: string;
};

export type AnalysisType =
  | "个股综合分析"
  | "技术面分析"
  | "基本面分析"
  | "消息面分析";

export type AIModel = string;

export type RiskPreference = "保守" | "中等" | "积极";

export type OutputFormat = "Markdown" | "纯文本";

export type AnalysisConfig = {
  stock: SelectedStock;
  analysisType: AnalysisType;
  aiModel: AIModel;
  promptTemplate: string;
};

export type AnalysisModelOption = {
  id: number;
  label: string;
  apiKeyRef: string;
  disabled?: boolean;
};

export type AnalysisPromptOption = {
  id: number;
  label: string;
  type: string;
};

export type OptionalHoldingContext = {
  costPrice?: string;
  shares?: string;
  riskPreference: RiskPreference;
};

export type ContextSummary = {
  basicInfo: {
    companyName: string;
    industry: string;
    listDate: string;
    totalMarketCap: string;
    floatMarketCap: string;
  };
  quote: {
    price: string;
    changeAmount: string;
    changePercent: string;
    open: string;
    high: string;
    low: string;
    amount: string;
    volume: string;
    turnoverRate: string;
    updateTime: string;
  };
  kline: {
    change20d: string;
    change60d: string;
    ytdChange: string;
    ma20: string;
    ma60: string;
    ma120: string;
    dailyKlines: string;
  };
  indicators: {
    ma: string;
    macd: string;
    rsi: string;
    kdj: string;
    boll: string;
  };
  news: {
    count: number;
    period: string;
    items: string[];
  };
};

export type DataModuleHeaderProps = {
  icon: ReactNode;
  title: string;
  meta?: string;
  tone?: "blue" | "green" | "orange" | "purple";
};
