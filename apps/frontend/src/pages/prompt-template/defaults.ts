import type { PromptEditorState, PromptTemplateCategory, PromptVariable } from "./types";

export const defaultPromptContent = "";

export const promptCategories: PromptTemplateCategory[] = [
  { id: "system", name: "系统模板", count: 0 },
  { id: "stock_analysis", name: "个股分析模板", count: 0, expanded: true, templates: [] },
  { id: "technical_analysis", name: "技术分析模板", count: 0 },
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
  { name: "{{user_position}}", description: "用户持仓信息（可选）", optional: true },
  { name: "{{risk_level}}", description: "风险偏好（可选）", optional: true },
  { name: "{{analysis_language}}", description: "分析语言" },
];

export const initialPromptEditorState: PromptEditorState = {
  selectedCategoryId: "stock_analysis",
  selectedTemplateId: "",
  templateName: "",
  templateType: "stock_analysis",
  templateDescription: "",
  promptContent: defaultPromptContent,
  previewFormat: "Markdown",
};
