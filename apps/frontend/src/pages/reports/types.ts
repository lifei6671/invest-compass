export type ReportStatus = "success" | "failed" | "running";

export type AnalysisType = "个股综合分析" | "技术面分析" | "财务分析" | "持仓分析";

export type ReportItem = {
  id: string;
  title: string;
  stockName: string;
  stockCode: string;
  stockSymbol?: string;
  analysisType: AnalysisType;
  model: string;
  generatedAt: string;
  generatedDate: string;
  riskSummary: string;
  status: ReportStatus;
  favorite?: boolean;
};

export type ReportFilters = {
  keyword: string;
  analysisType: "全部类型" | AnalysisType;
  model: string;
  dateRangeLabel: string;
  dateRangeStart: string;
  dateRangeEnd: string;
  status: "全部状态" | "成功" | "失败" | "生成中";
};

export type ReportStats = {
  totalCount: number;
  uniqueSymbols: number;
  latestCreatedAt: string;
};

export type TopModelItem = {
  name: string;
  count: number;
  percent: number;
};

export type AnalysisTypeDistributionItem = {
  type: AnalysisType;
  value: number;
  percent: number;
  color: string;
};
