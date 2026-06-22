export type DataUsageOverview = {
  scope: string;
  defaultMarket: string;
  marketSource: string;
  newsSource: string;
  klineRange: string;
  usage: string;
  disclaimer: string;
};

export type DataSourceExplanationItem = {
  name: string;
  source: string;
  description: string;
  status: "normal" | "limited" | "disabled";
};

export type DataFreshnessItem = {
  label: string;
  value: string;
};

export type AIContextDataType = "行情数据" | "K线数据" | "技术指标" | "新闻摘要" | "快照数据";

export type AIOutputNature = {
  type: "事实" | "推断" | "观点";
  description: string;
  color: "blue" | "orange" | "green";
};

export type FieldDescription = {
  field: string;
  description: string;
};

export type FAQItem = {
  id: string;
  question: string;
};
