export type ReportDetail = {
  id: string;
  title: string;
  stockName: string;
  symbol: string;
  displayCode: string;
  analysisType: string;
  analysisTypeValue: string;
  model: string;
  generatedAt: string;
  taskId: string;
  promptTemplateId: number;
  dataUpdatedAt: string;
  favorite: boolean;
  markdown: string;
  riskSummary: string;
  inputSnapshot: InputSnapshot | null;
};

export type ReportSection = {
  id: string;
  index: number;
  title: string;
  level: number;
};

export type InputSnapshot = Record<string, unknown> & {
  raw_prompt?: string;
  rawPrompt?: string;
  prompt?: string;
  rendered_prompt?: string;
  renderedPrompt?: string;
  stock?: {
    name?: string;
    symbol?: string;
  };
  market?: {
    board?: string;
    industry?: string;
  };
  quote?: {
    price?: string;
    changePercent?: string;
    amount?: string;
    turnoverRate?: string;
  };
  indicators?: {
    ma?: string;
    macd?: string;
    rsi?: string;
    kdj?: string;
    boll?: string;
  };
  news?: {
    count?: number;
    period?: string;
  };
  promptTemplate?: string;
  model?: string;
  temperature?: string;
  maxTokens?: string;
};
