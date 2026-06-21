import type { InputSnapshot, ReportDetail, ReportSection } from "./types";

export const reportSections: ReportSection[] = [
  { id: "conclusion", index: 1, title: "核心结论" },
  { id: "quote", index: 2, title: "当前行情状态" },
  { id: "technical", index: 3, title: "技术面观察" },
  { id: "fundamental", index: 4, title: "基本面观察" },
  { id: "news", index: 5, title: "消息面观察" },
  { id: "industry", index: 6, title: "行业与竞争格局" },
  { id: "risks", index: 7, title: "风险点" },
  { id: "tracking", index: 8, title: "后续观察指标" },
  { id: "notes", index: 9, title: "说明" },
];

export const reportMarkdown = `## 1. 核心结论

公司基本面稳健，覆铜板行业需求结构向高端化升级，短期关注成本端波动与订单变化，中期维持谨慎乐观，建议持续跟踪产品结构、毛利率及高端产能释放节奏。

## 2. 当前行情状态

- 当前收盘价 25.68 元，较昨日上涨 +1.42%，日内震荡上行。
- 成交额 8.92 亿元，换手率 1.72%，市场活跃度一般。
- 近 20 日涨跌幅 +6.21%，表现强于沪深300。

## 3. 技术面观察

- 日K线位于20日均线之上，短期趋势偏多。
- MACD 金叉，动能柱为正，动能有所增强。
- RSI(12) 为 61.23，处于中高位，注意短期波动风险。
- 均线系统：股价运行在 MA5 / MA10 上方，MA20 逐步走平。

## 4. 基本面观察

公司盈利能力稳定，净利率和 ROE 处于行业中等偏上水平，现金流状况良好，资产结构健康，具备持续投入研发和产能扩张的能力。

## 5. 消息面观察

- 一季度归母净利润同比增长 18.35%，产品结构持续优化。
- 覆铜板需求回暖，高端产品订单相对饱满。
- 公司持续增加研发投入，关注封装基板与高速材料进展。

## 6. 行业与竞争格局

覆铜板行业受电子终端需求、AI 服务器、通信设备与汽车电子共同影响。公司在客户结构、产品体系和研发投入方面具备一定优势，但仍需关注同行扩产和价格竞争。

## 7. 风险点

- 下游需求不及预期，行业竞争加剧。
- 原材料价格波动导致成本上升。
- 宏观经济波动及政策不确定性。

## 8. 后续观察指标

- 订单及出货量变化情况。
- 毛利率及费用率趋势。
- 高端产品放量进度。

## 9. 说明

本报告基于本地 mock 行情、技术指标和资讯快照生成，仅用于界面展示与研究流程验证。`;

export const reportDetail: ReportDetail = {
  id: "report_20250520_152834",
  title: "生益科技 CN:SH:600183 投研分析",
  stockName: "生益科技",
  symbol: "CN:SH:600183",
  displayCode: "600183.SH",
  analysisType: "个股综合分析",
  model: "DeepSeek-V3",
  generatedAt: "2025-05-20 15:28:34",
  taskId: "task_20250520_152834_abcd1234",
  dataUpdatedAt: "2025-05-20 15:30:00",
  favorite: false,
  markdown: reportMarkdown,
  riskSummary: "成本波动；订单变化；行业竞争加剧",
};

export const inputSnapshot: InputSnapshot = {
  stock: {
    name: "生益科技",
    symbol: "CN:SH:600183",
  },
  market: {
    board: "A股 / 沪市主板",
    industry: "电子材料 / 覆铜板",
  },
  quote: {
    price: "25.68",
    changePercent: "+1.42%",
    amount: "8.92 亿元",
    turnoverRate: "1.72%",
  },
  indicators: {
    ma: "25.28 / 24.92 / 24.18",
    macd: "DIF 0.42，DEA 0.31，MACD 0.22",
    rsi: "61.23",
    kdj: "K 74.21，D 69.12，J 84.38",
    boll: "上轨 26.31，中轨 24.18，下轨 22.05",
  },
  news: {
    count: 36,
    period: "近7天",
  },
  promptTemplate: "默认个股分析模板",
  model: "DeepSeek-V3",
  temperature: "0.70",
  maxTokens: "4096",
};
