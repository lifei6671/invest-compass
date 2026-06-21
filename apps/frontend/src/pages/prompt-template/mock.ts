import type { PromptEditorState, PromptTemplate, PromptTemplateCategory, PromptTemplateCategoryType, PromptVariable } from "./types";

export const defaultPromptContent = `# 角色定义
你是一名专业的金融研究助理，擅长基本面分析、技术分析与消息面分析。
请基于提供的结构化数据，生成高质量的个股研究报告。

# 分析对象
- 股票名称：{{stock_name}}
- 股票代码：{{stock_code}}
- 市场：{{market}}

# 数据输入
- 最新行情：{{quote}}
- K线概况：{{kline_summary}}
- 技术指标：{{indicators}}
- 相关新闻：{{news}}
- 用户持仓（可选）：{{user_position}}
- 风险偏好（可选）：{{risk_level}}
- 分析语言：{{analysis_language}}

# 输出要求
1. 结构清晰，使用 Markdown 输出。
2. 必须包含“风险提示”部分，提示内容仅供研究参考，不构成投资建议。
3. 请明确区分“事实、推断与观点”。`;

export const promptTemplates: PromptTemplate[] = [
  {
    id: "default-stock-analysis",
    name: "默认个股分析模板",
    type: "stock_analysis",
    description: "面向个股的综合分析模板，包含基本面、技术面与消息面分析框架。",
    content: defaultPromptContent,
    isBuiltin: true,
  },
  {
    id: "deep-stock-analysis",
    name: "深度个股分析模板",
    type: "stock_analysis",
    description: "适用于更长周期跟踪的个股深度研究模板。",
    content: defaultPromptContent,
    isBuiltin: true,
  },
  {
    id: "brief-stock-analysis",
    name: "简明个股分析模板",
    type: "stock_analysis",
    description: "面向快速浏览场景的简明个股研究模板。",
    content: defaultPromptContent,
    isBuiltin: true,
  },
  {
    id: "event-driven-analysis",
    name: "事件驱动分析模板",
    type: "stock_analysis",
    description: "围绕新闻、公告和行业事件构建分析框架。",
    content: defaultPromptContent,
    isBuiltin: true,
  },
  {
    id: "value-investing-analysis",
    name: "价值投资分析模板",
    type: "stock_analysis",
    description: "强调商业模式、财务质量与估值区间的研究模板。",
    content: defaultPromptContent,
    isBuiltin: true,
  },
];

export const promptCategories: PromptTemplateCategory[] = [
  { id: "system", name: "系统模板", count: 8 },
  { id: "stock_analysis", name: "个股分析模板", count: 12, expanded: true, templates: promptTemplates },
  { id: "technical_analysis", name: "技术分析模板", count: 9 },
  { id: "financial_analysis", name: "财务分析模板", count: 7 },
  { id: "position_analysis", name: "持仓分析模板", count: 6 },
  { id: "market_review", name: "市场复盘模板", count: 5 },
  { id: "custom", name: "自定义模板", count: 15 },
];

export const promptVariables: PromptVariable[] = [
  { name: "{{stock_name}}", description: "股票名称" },
  { name: "{{stock_code}}", description: "股票代码" },
  { name: "{{market}}", description: "市场（如 A股/港股/美股）" },
  { name: "{{quote}}", description: "最新行情数据摘要" },
  { name: "{{kline_summary}}", description: "K线数据概览" },
  { name: "{{indicators}}", description: "技术指标摘要" },
  { name: "{{news}}", description: "新闻资讯摘要" },
  { name: "{{user_position}}", description: "用户持仓信息（可选）", optional: true },
  { name: "{{risk_level}}", description: "风险偏好（可选）", optional: true },
  { name: "{{analysis_language}}", description: "分析语言" },
];

export const promptTypeOptions: Array<{ label: string; value: PromptTemplateCategoryType }> = [
  { label: "系统模板", value: "system" },
  { label: "个股分析模板", value: "stock_analysis" },
  { label: "技术分析模板", value: "technical_analysis" },
  { label: "财务分析模板", value: "financial_analysis" },
  { label: "持仓分析模板", value: "position_analysis" },
  { label: "市场复盘模板", value: "market_review" },
  { label: "自定义模板", value: "custom" },
];

export const initialPromptEditorState: PromptEditorState = {
  selectedCategoryId: "stock_analysis",
  selectedTemplateId: "default-stock-analysis",
  templateName: "默认个股分析模板",
  templateType: "stock_analysis",
  templateDescription: "面向个股的综合分析模板，包含基本面、技术面与消息面分析框架。",
  promptContent: defaultPromptContent,
  previewFormat: "Markdown",
};

export const promptPreviewMarkdown = `# 生益科技（600183.SH）个股综合分析

## 1. 核心结论
公司基本面稳健，短期关注需求节奏与成本变化，技术面处于...

## 2. 基本面分析
- 行业地位：...
- 财务表现：...
- 估值水平：...

## 3. 技术面分析
- 趋势判断：...
- 关键支撑/压力：...
- 技术指标：...`;
