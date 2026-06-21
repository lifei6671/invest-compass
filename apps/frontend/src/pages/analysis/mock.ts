import type { AIModel, AnalysisConfig, AnalysisType, ContextSummary, OptionalHoldingContext, RiskPreference, SelectedStock } from "./types";

export const analysisTypes: AnalysisType[] = ["个股综合分析", "技术面分析", "基本面分析", "消息面分析"];

export const aiModels: AIModel[] = [
  "DeepSeek (DeepSeek-V3)",
  "OpenAI (gpt-4o)",
  "Qwen (qwen-max)",
  "本地模型 (Ollama)",
];

export const promptTemplates = ["默认个股分析模板", "技术面分析模板", "基本面跟踪模板", "风险观察模板"];

export const riskPreferences: RiskPreference[] = ["保守", "中等", "积极"];

export const stockCandidates: Array<SelectedStock & { keyword: string; industry: string }> = [
  {
    name: "生益科技",
    symbol: "CN:SH:600183",
    code: "600183.SH",
    keyword: "sykj shengyikeji 600183",
    industry: "电子材料 / 覆铜板",
  },
  {
    name: "贵州茅台",
    symbol: "CN:SH:600519",
    code: "600519.SH",
    keyword: "gzmt guizhoumaotai 600519",
    industry: "白酒",
  },
  {
    name: "宁德时代",
    symbol: "CN:SZ:300750",
    code: "300750.SZ",
    keyword: "ndsd ningdeshidai 300750",
    industry: "动力电池",
  },
  {
    name: "中际旭创",
    symbol: "CN:SZ:300308",
    code: "300308.SZ",
    keyword: "zjxc zhongjixuchuang 300308",
    industry: "光模块",
  },
  {
    name: "新易盛",
    symbol: "CN:SZ:300502",
    code: "300502.SZ",
    keyword: "xys xinyisheng 300502",
    industry: "光通信",
  },
];

export const initialAnalysisConfig: AnalysisConfig = {
  stock: {
    name: "生益科技",
    symbol: "CN:SH:600183",
    code: "600183.SH",
  },
  analysisType: "个股综合分析",
  aiModel: "DeepSeek (DeepSeek-V3)",
  promptTemplate: "默认个股分析模板",
};

export const initialHoldingContext: OptionalHoldingContext = {
  costPrice: "",
  shares: "",
  riskPreference: "中等",
};

export const contextSummary: ContextSummary = {
  basicInfo: {
    companyName: "广东生益科技股份有限公司",
    industry: "电子材料 / 覆铜板",
    listDate: "1998-10-28",
    totalMarketCap: "464.32 亿元",
    floatMarketCap: "421.67 亿元",
  },
  quote: {
    price: "25.68",
    changeAmount: "+0.36",
    changePercent: "+1.42%",
    open: "25.32",
    high: "25.88",
    low: "25.22",
    amount: "8.92 亿元",
    volume: "34.73 万手",
    turnoverRate: "1.72%",
    updateTime: "2025-05-20 15:30:00",
  },
  kline: {
    change20d: "+6.21%",
    change60d: "+12.34%",
    ytdChange: "+18.65%",
    ma20: "24.18",
    ma60: "23.45",
    ma120: "22.91",
  },
  indicators: {
    ma: "25.28 / 24.92 / 24.18",
    macd: "DIF 0.42，DEA 0.31，MACD 0.22",
    rsi: "65.31 / 61.23 / 58.72",
    kdj: "K 74.21，D 69.12，J 84.38",
    boll: "上轨 26.31，中轨 24.18，下轨 22.05",
  },
  news: {
    count: 36,
    period: "近7天",
    items: [
      "生益科技：一季度归母净利润同比增长18.35%，产品结构持续优化",
      "覆铜板需求回暖，生益科技高端产品订单饱满",
      "公司发布2024年年报，营收稳健增长，研发投入持续提升",
    ],
  },
};

export const mockMarkdown = `# 生益科技（600183.SH）个股综合分析报告（示例）

## 1. 核心结论
公司基本面稳健，行业需求回暖带动景气度提升，技术面呈现震荡上行趋势，中期维持偏多观点，建议持续跟踪订单、毛利率及高端产品放量情况。

## 2. 当前行情状态
- 当前价格 25.68 元，较昨日上涨 1.42%。
- 成交额 8.92 亿元，换手率 1.72%，市场活跃度一般。
- 近20日涨跌幅 +6.21%，表现强于沪深300。

## 3. 技术面观察
- 日K线位于20日均线之上，短期趋势偏多。
- MACD 金叉，动能柱为正，动能有所增强。
- RSI(12) 61.23，处于中高位，注意短期波动风险。

## 4. 消息面观察
- 行业需求回暖，上游原材料价格相对稳定，利好公司盈利能力。
- 公司发布年报及一季报，业绩稳健增长，研发投入持续提升。
- 机构调研关注高端覆铜板和封装基板业务进展。

## 5. 风险点
- 下游需求不及预期，行业竞争加剧。
- 原材料价格波动导致成本上升。
- 宏观经济波动及政策不确定性。

## 6. 后续观察指标
- 订单及出货量变化情况。
- 毛利率及费用率趋势。
- 高端产品（高速覆铜板、封装基板）放量进度。`;

export const mockPlainText = mockMarkdown
  .replace(/^#\s+/gm, "")
  .replace(/^##\s+/gm, "")
  .replace(/^- /gm, "· ");
