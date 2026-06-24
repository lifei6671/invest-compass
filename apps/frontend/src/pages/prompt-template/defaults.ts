import type { PromptEditorState, PromptTemplateCategory, PromptVariable } from "./types";

export const defaultPromptContent = "";

export const promptCategories: PromptTemplateCategory[] = [
  { id: "system", name: "系统模板", count: 0 },
  { id: "stock_full", name: "个股综合模板", count: 0, expanded: true, templates: [] },
  { id: "technical", name: "技术分析模板", count: 0 },
  { id: "fundamental", name: "基本面模板", count: 0 },
  { id: "news", name: "消息面模板", count: 0 },
  { id: "custom", name: "自定义模板", count: 0 },
];

export const promptVariables: PromptVariable[] = [
  { name: "{{stock_name}}", description: "股票名称" },
  { name: "{{stock_code}}", description: "股票代码" },
  { name: "{{market}}", description: "市场" },
  { name: "{{quote}}", description: "最新行情数据摘要" },
  { name: "{{kline_summary}}", description: "K线数据概览" },
  { name: "{{indicators}}", description: "技术指标摘要" },
  { name: "{{news}}", description: "新闻资讯摘要" },
  { name: "{{market_news}}", description: "市场新闻摘要" },
  { name: "{{industry_news}}", description: "行业新闻摘要" },
  { name: "{{fundamental_summary}}", description: "基本面摘要" },
  { name: "{{financial_summary}}", description: "财务摘要" },
  { name: "{{valuation_summary}}", description: "估值摘要" },
  { name: "{{forecast_summary}}", description: "预测摘要" },
  { name: "{{industry_summary}}", description: "行业摘要" },
  { name: "{{user_position}}", description: "本次分析请求的一次性持仓上下文", optional: true },
  { name: "{{data_asof}}", description: "数据截至时间" },
  { name: "{{context_sources}}", description: "上下文数据来源" },
  { name: "{{context_quality}}", description: "上下文质量说明" },
  { name: "{{prompt_key}}", description: "模板稳定标识" },
  { name: "{{prompt_version}}", description: "模板版本" },
  { name: "{{analysis_language}}", description: "分析语言" },
];

export const initialPromptEditorState: PromptEditorState = {
  selectedCategoryId: "stock_full",
  selectedTemplateId: "",
  templateName: "",
  templateType: "stock_full",
  templateDescription: "",
  promptContent: defaultPromptContent,
  previewFormat: "Markdown",
};
